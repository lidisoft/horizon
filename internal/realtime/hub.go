// Package realtime pushes domain events to browsers over WebSockets
// (docs/realtime.md). Each serve replica subscribes to the event subjects on
// NATS core, matches every event against its sockets' topics, and checks it
// against each recipient's current permissions before sending it.
package realtime

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Options are the hub's dependencies.
type Options struct {
	Log        zerolog.Logger
	Pool       *pgxpool.Pool
	NATS       *nats.Conn
	Stream     eventlog.Stream
	Identity   *identity.Service
	Access     *access.Service
	Workspaces *workspace.Service
	Projects   *project.Service
	Issues     *issue.Service
	// Origins are the hosts pages may open sockets from (BASE_URL's), on
	// top of the request's own host.
	Origins []string
	// MaxAge closes sockets after this long so clients reconnect and
	// authenticate again; 1 h by default.
	MaxAge time.Duration
}

// Hub holds a replica's sockets.
type Hub struct {
	o Options

	mu    sync.RWMutex
	conns map[*conn]struct{}

	slugMu sync.Mutex
	slugs  map[uuid.UUID]string

	docMu    sync.Mutex
	docConns map[uuid.UUID]map[*conn]struct{} // sockets editing each description

	replica  string // this replica's id, in socket ids
	presMu   sync.Mutex
	presence map[uuid.UUID]map[string]presenceEntry // issue → socket id → entry, every replica's
	viewers  map[uuid.UUID]map[*conn]string         // issue → local sockets on it (the key they named)
}

// New returns a hub; Run feeds it.
func New(o Options) *Hub {
	if o.MaxAge == 0 {
		o.MaxAge = time.Hour
	}
	h := &Hub{o: o, conns: map[*conn]struct{}{}, slugs: map[uuid.UUID]string{}, docConns: map[uuid.UUID]map[*conn]struct{}{},
		replica: uuid.NewString()[:8], presence: map[uuid.UUID]map[string]presenceEntry{}, viewers: map[uuid.UUID]map[*conn]string{}}
	if o.Issues != nil {
		o.Issues.OnDocSteps(h.publishDoc)
	}
	return h
}

// Run receives the published events until ctx ends.
func (h *Hub) Run(ctx context.Context) error {
	go func() {
		if err := h.runDocs(ctx); err != nil {
			h.o.Log.Error().Err(err).Msg("could not subscribe to document steps")
		}
	}()
	go func() {
		if err := h.runPresence(ctx); err != nil {
			h.o.Log.Error().Err(err).Msg("could not subscribe to presence")
		}
	}()
	ch := make(chan *nats.Msg, 1024)
	sub, err := h.o.NATS.ChanSubscribe(h.o.Stream.Prefix+".*.*.*.*", ch)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-ch:
			e, err := eventlog.Decode(m.Data)
			if err != nil {
				h.o.Log.Warn().Err(err).Str("subject", m.Subject).Msg("undecodable event")
				continue
			}
			h.dispatch(ctx, e, nil)
		}
	}
}

// routed is an event ready to offer to sockets.
type routed struct {
	e        eventlog.Event
	data     []byte      // the "event" message
	projects []uuid.UUID // whose viewers may see it; none: workspace-wide
	issueID  uuid.UUID   // for issue topics
	onlyUser uuid.UUID   // a personal thing (a personal view): its owner only
}

// dispatch offers e to the sockets watching its workspace (only to one
// socket when replaying).
func (h *Hub) dispatch(ctx context.Context, e eventlog.Event, only *conn) {
	if !e.WorkspaceID.Valid {
		return
	}
	ws := e.WorkspaceID.UUID
	var targets []*conn
	if only != nil {
		targets = []*conn{only}
	} else {
		h.mu.RLock()
		for c := range h.conns {
			if c.watches(ws) {
				targets = append(targets, c)
			}
		}
		h.mu.RUnlock()
	}
	// Access may have changed: recompute what each socket may see.
	if e.AggregateType != "issue" {
		if e.AggregateType == "workspace" {
			h.slugMu.Lock()
			delete(h.slugs, ws)
			h.slugMu.Unlock()
		}
		for _, c := range targets {
			c.forget(ws)
		}
	}
	if len(targets) == 0 {
		return
	}
	r, ok, err := h.route(ctx, e)
	if err != nil {
		h.o.Log.Warn().Err(err).Stringer("event", e.ID).Msg("could not describe event")
		return
	}
	if !ok {
		return
	}
	for _, c := range targets {
		c.offer(ctx, r)
	}
}

func (h *Hub) route(ctx context.Context, e eventlog.Event) (routed, bool, error) {
	r := routed{e: e}
	var public any
	if e.AggregateType == "issue" {
		p, ok, err := h.o.Issues.Public(ctx, e)
		if err != nil || !ok {
			return r, false, err
		}
		public, r.projects, r.issueID = p.Event, p.Projects, e.AggregateID
	} else {
		slug, err := h.slug(ctx, e.WorkspaceID.UUID)
		if err != nil {
			return r, false, err
		}
		var payload map[string]any
		_ = json.Unmarshal(e.Payload, &payload)
		pe := issue.PublicEvent{
			ID: e.ID, Type: e.PublicType(), Workspace: slug, OccurredAt: e.OccurredAt, Payload: payload,
			Aggregate: issue.EventAggregate{Type: e.AggregateType, ID: e.AggregateID, Version: e.AggregateVersion},
		}
		if e.ActorID.Valid {
			pe.Actor = &issue.Ref{ID: e.ActorID.UUID}
		}
		if owner, ok := payload["owner_id"].(string); ok && e.AggregateType == "view" {
			if id, err := uuid.Parse(owner); err == nil {
				r.onlyUser = id
			}
		}
		switch {
		case e.AggregateType == "project" && changesVisibility(e.Type, payload):
			// Who sees the project changes: every member hears of it (they
			// all saw it before it turned private, or will after).
		case e.AggregateType == "project":
			r.projects = []uuid.UUID{e.AggregateID}
		default:
			if pid, ok := payload["project_id"].(string); ok {
				if id, err := uuid.Parse(pid); err == nil {
					r.projects = []uuid.UUID{id}
				}
			}
		}
		public = pe
	}
	data, err := json.Marshal(message{T: "event", D: public})
	if err != nil {
		return r, false, err
	}
	r.data = data
	return r, true, nil
}

func (h *Hub) slug(ctx context.Context, id uuid.UUID) (string, error) {
	h.slugMu.Lock()
	defer h.slugMu.Unlock()
	if s, ok := h.slugs[id]; ok {
		return s, nil
	}
	var s string
	if err := h.o.Pool.QueryRow(ctx, `SELECT slug FROM workspaces WHERE id = $1`, id).Scan(&s); err != nil {
		return "", err
	}
	h.slugs[id] = s
	return s, nil
}

func (h *Hub) add(c *conn) {
	h.mu.Lock()
	h.conns[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *conn) {
	c.leaveAll()
	c.leavePresence()
	h.mu.Lock()
	delete(h.conns, c)
	h.mu.Unlock()
}

// Count is the number of open sockets.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// changesVisibility tells project events that change who sees the project.
func changesVisibility(typ string, payload map[string]any) bool {
	if typ == "ProjectDeleted" {
		return true
	}
	fields, _ := payload["fields"].([]any)
	for _, f := range fields {
		if f == "visibility" {
			return true
		}
	}
	return false
}

// RegisterMetrics exposes the number of open sockets.
func (h *Hub) RegisterMetrics(reg prometheus.Registerer) {
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "horizon", Subsystem: "realtime", Name: "sockets",
		Help: "Open WebSockets on this replica.",
	}, func() float64 { return float64(h.Count()) }))
}
