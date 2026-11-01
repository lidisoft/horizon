package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// Presence (docs/realtime.md#presence): who views or edits an issue. Each
// socket announces it (presence {ws, issue, state}) and repeats it with its
// pings; replicas share the announcements on NATS core
// (<prefix>.presence.<workspace id>) and drop those not repeated for 60 s.
// Sockets announcing an issue receive its list as it changes.

const presenceTTL = 60 * time.Second

var connIDs atomic.Int64

// presenceEntry is one socket on one issue.
type presenceEntry struct {
	Conn  string    `json:"conn"`
	Issue uuid.UUID `json:"issue"`
	User  uuid.UUID `json:"user"`
	Name  string    `json:"name"`
	State string    `json:"state"` // viewing, editing, gone
	At    time.Time `json:"at"`
}

// PresentUser is a person on an issue, as clients see it.
type PresentUser struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	State string    `json:"state" enums:"viewing,editing"`
}

func (h *Hub) presenceSubject(ws string) string { return h.o.Stream.Prefix + ".presence." + ws }

// announce publishes a socket's presence on an issue.
func (h *Hub) announce(ws uuid.UUID, e presenceEntry) {
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	if err := h.o.NATS.Publish(h.presenceSubject(ws.String()), b); err != nil {
		h.o.Log.Warn().Err(err).Msg("could not publish presence")
	}
}

// runPresence keeps the table from every replica's announcements and
// expires silent entries.
func (h *Hub) runPresence(ctx context.Context) error {
	ch := make(chan *nats.Msg, 1024)
	sub, err := h.o.NATS.ChanSubscribe(h.presenceSubject("*"), ch)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-ch:
			var e presenceEntry
			if json.Unmarshal(m.Data, &e) != nil {
				continue
			}
			h.presMu.Lock()
			if h.presence[e.Issue] == nil {
				h.presence[e.Issue] = map[string]presenceEntry{}
			}
			if e.State == "gone" {
				delete(h.presence[e.Issue], e.Conn)
			} else {
				e.At = time.Now()
				h.presence[e.Issue][e.Conn] = e
			}
			h.presMu.Unlock()
			h.pushPresence(e.Issue)
		case now := <-tick.C:
			var changed []uuid.UUID
			h.presMu.Lock()
			for issue, entries := range h.presence {
				for id, e := range entries {
					if now.Sub(e.At) > presenceTTL {
						delete(entries, id)
						changed = append(changed, issue)
					}
				}
				if len(entries) == 0 {
					delete(h.presence, issue)
				}
			}
			h.presMu.Unlock()
			for _, issue := range slices.Compact(changed) {
				h.pushPresence(issue)
			}
		}
	}
}

// pushPresence sends an issue's list to the local sockets on it.
func (h *Hub) pushPresence(issue uuid.UUID) {
	h.presMu.Lock()
	byUser := map[uuid.UUID]PresentUser{}
	for _, e := range h.presence[issue] {
		u, ok := byUser[e.User]
		if !ok || (u.State != "editing" && e.State == "editing") {
			byUser[e.User] = PresentUser{ID: e.User, Name: e.Name, State: e.State}
		}
	}
	targets := make(map[*conn]string, len(h.viewers[issue]))
	for c, key := range h.viewers[issue] {
		targets[c] = key
	}
	h.presMu.Unlock()
	users := make([]PresentUser, 0, len(byUser))
	for _, u := range byUser {
		users = append(users, u)
	}
	slices.SortFunc(users, func(a, b PresentUser) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	for c, key := range targets {
		c.reply(message{T: "presence", D: map[string]any{"issue": key, "users": users}})
	}
}

// presenceIn is the body of a presence message.
type presenceIn struct {
	WS    string `json:"ws"`
	Issue string `json:"issue"`
	State string `json:"state"`
}

// handlePresence records a socket's presence on an issue it may see.
func (c *conn) handlePresence(ctx context.Context, in inbound) {
	var d presenceIn
	if err := json.Unmarshal(in.D, &d); err != nil || d.Issue == "" || !slices.Contains([]string{"viewing", "editing", "gone"}, d.State) {
		c.reply(message{T: "error", ID: in.ID, D: map[string]string{"code": "bad_message"}})
		return
	}
	wc, err := c.workspace(ctx, d.WS)
	if err != nil {
		return
	}
	ic, err := c.hub.o.Issues.Enter(ctx, wc, d.Issue)
	if err != nil {
		return
	}
	h := c.hub
	h.presMu.Lock()
	if d.State == "gone" {
		delete(h.viewers[ic.Issue.ID], c)
	} else {
		if h.viewers[ic.Issue.ID] == nil {
			h.viewers[ic.Issue.ID] = map[*conn]string{}
		}
		h.viewers[ic.Issue.ID][c] = d.Issue
	}
	h.presMu.Unlock()
	c.mu.Lock()
	if d.State == "gone" {
		delete(c.present, ic.Issue.ID)
	} else {
		c.present[ic.Issue.ID] = wc.Workspace.ID
	}
	c.mu.Unlock()
	h.announce(wc.Workspace.ID, presenceEntry{Conn: c.id, Issue: ic.Issue.ID, User: c.principal.User.ID, Name: c.principal.User.Name, State: d.State})
}

// leavePresence announces a closing socket gone from its issues.
func (c *conn) leavePresence() {
	c.mu.Lock()
	present := make(map[uuid.UUID]uuid.UUID, len(c.present))
	for issue, ws := range c.present {
		present[issue] = ws
	}
	c.mu.Unlock()
	h := c.hub
	h.presMu.Lock()
	for issue := range present {
		delete(h.viewers[issue], c)
	}
	h.presMu.Unlock()
	for issue, ws := range present {
		h.announce(ws, presenceEntry{Conn: c.id, Issue: issue, User: c.principal.User.ID, State: "gone"})
	}
}

func newConnID(replica string) string { return fmt.Sprintf("%s-%d", replica, connIDs.Add(1)) }
