package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Limits of a socket.
const (
	maxMessage   = 2 << 20 // a batch of steps can carry a pasted document
	maxTopics    = 200
	sendBuffer   = 256
	idleTimeout  = 75 * time.Second // the client pings every 25 s
	visibleTTL   = 30 * time.Second
	replayWindow = 24 * time.Hour
	replayLimit  = 500
)

// message is the envelope of every frame (docs/realtime.md#envelope).
type message struct {
	T  string `json:"t"`
	ID string `json:"id,omitempty"`
	D  any    `json:"d,omitempty"`
}

type inbound struct {
	T  string          `json:"t"`
	ID string          `json:"id,omitempty"`
	D  json.RawMessage `json:"d,omitempty"`
}

// Topic kinds.
const (
	topicWorkspace = "ws"
	topicProject   = "project"
	topicIssue     = "issue"
)

type topic struct {
	name      string
	kind      string
	workspace uuid.UUID
	project   uuid.UUID
	issue     uuid.UUID
}

type visibility struct {
	ok      bool
	expires time.Time
}

// conn is one socket.
type conn struct {
	hub       *Hub
	ws        *websocket.Conn
	principal identity.Principal
	send      chan []byte
	cancel    context.CancelFunc

	mu      sync.Mutex
	topics  map[string]topic
	wcs     map[uuid.UUID]workspace.Context // per workspace entered
	sees    map[uuid.UUID]bool              // workspace-wide events allowed
	visible map[[2]uuid.UUID]visibility     // (project, 0 or 1: issue.view) → allowed
	docs    map[uuid.UUID]joined            // descriptions joined, by document id
	id      string                          // unique across replicas, for presence and cursors
	present map[uuid.UUID]uuid.UUID         // issues announced → their workspace
}

// Register mounts GET /ws on the API group.
func (h *Hub) Register(api *gin.RouterGroup) {
	api.GET("/ws", h.serve)
}

// serve upgrades the request.
//
//	@Summary		Open the realtime socket
//	@Description	WebSocket (docs/realtime.md): subscribe to ws:<slug>, project:<slug>/<KEY> and issue:<slug>/<KEY-n> and receive their domain events. Session cookie, or an API token in the token query parameter.
//	@Tags			realtime
//	@Param			token	query	string	false	"API token, for clients without cookies"
//	@Success		101
//	@Router			/api/v1/ws [get]
func (h *Hub) serve(c *gin.Context) {
	p, ok := identity.CurrentPrincipal(c)
	if !ok && c.Query("token") != "" {
		var err error
		if p, err = h.o.Identity.AuthenticateToken(c, c.Query("token")); err == nil {
			ok = true
		}
	}
	if !ok {
		server.WriteError(c, apperr.Unauthorized("unauthenticated"))
		return
	}
	ws, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{OriginPatterns: h.o.Origins})
	if err != nil {
		return // Accept answered
	}
	ws.SetReadLimit(maxMessage)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), h.o.MaxAge)
	cn := &conn{hub: h, ws: ws, principal: p, send: make(chan []byte, sendBuffer), cancel: cancel,
		topics: map[string]topic{}, wcs: map[uuid.UUID]workspace.Context{}, sees: map[uuid.UUID]bool{}, visible: map[[2]uuid.UUID]visibility{}, docs: map[uuid.UUID]joined{},
		id: newConnID(h.replica), present: map[uuid.UUID]uuid.UUID{}}
	h.add(cn)
	defer h.remove(cn)
	go cn.writeLoop(ctx)
	err = cn.readLoop(ctx)
	cancel()
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		_ = ws.Close(websocket.StatusGoingAway, "reconnect")
	case websocket.CloseStatus(err) != -1:
	default:
		_ = ws.Close(websocket.StatusInternalError, "")
	}
}

func (c *conn) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-c.send:
			wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.ws.Write(wctx, websocket.MessageText, b)
			cancel()
			if err != nil {
				c.cancel()
				return
			}
		}
	}
}

// push queues a frame; a socket too slow to keep up is closed (the client
// reconnects and replays).
func (c *conn) push(b []byte) {
	select {
	case c.send <- b:
	default:
		c.cancel()
	}
}

func (c *conn) reply(m message) {
	b, _ := json.Marshal(m)
	c.push(b)
}

func (c *conn) readLoop(ctx context.Context) error {
	for {
		rctx, cancel := context.WithTimeout(ctx, idleTimeout)
		_, data, err := c.ws.Read(rctx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		var in inbound
		if err := json.Unmarshal(data, &in); err != nil {
			c.reply(message{T: "error", D: map[string]string{"code": "bad_message"}})
			continue
		}
		switch in.T {
		case "ping":
			c.reply(message{T: "pong", ID: in.ID})
		case "sub":
			c.subscribe(ctx, in)
		case "doc.join", "doc.steps", "doc.leave", "doc.cursor":
			c.handleDoc(ctx, in)
		case "presence":
			c.handlePresence(ctx, in)
		case "unsub":
			var d struct {
				Topics []string `json:"topics"`
			}
			_ = json.Unmarshal(in.D, &d)
			c.mu.Lock()
			for _, t := range d.Topics {
				delete(c.topics, t)
			}
			c.mu.Unlock()
			c.reply(message{T: "ack", ID: in.ID, D: map[string]any{"topics": d.Topics}})
		default:
			c.reply(message{T: "error", ID: in.ID, D: map[string]string{"code": "unknown_type"}})
		}
	}
}

// subscribe checks and adds topics, then replays what the client missed
// since the event it names.
func (c *conn) subscribe(ctx context.Context, in inbound) {
	var d struct {
		Topics []string `json:"topics"`
		Since  string   `json:"since,omitempty"`
	}
	if err := json.Unmarshal(in.D, &d); err != nil {
		c.reply(message{T: "error", ID: in.ID, D: map[string]string{"code": "bad_message"}})
		return
	}
	accepted, denied := []string{}, []string{}
	var added []topic
	for _, name := range d.Topics {
		c.mu.Lock()
		n := len(c.topics)
		c.mu.Unlock()
		if n >= maxTopics {
			denied = append(denied, name)
			continue
		}
		t, err := c.resolve(ctx, name)
		if err != nil {
			denied = append(denied, name)
			continue
		}
		c.mu.Lock()
		c.topics[name] = t
		c.mu.Unlock()
		accepted = append(accepted, name)
		added = append(added, t)
	}
	c.reply(message{T: "ack", ID: in.ID, D: map[string]any{"topics": accepted, "denied": denied}})
	if d.Since != "" && len(added) > 0 {
		c.replay(ctx, d.Since, added)
	}
}

// resolve checks a topic the way REST checks the resource: what isn't
// visible doesn't exist.
func (c *conn) resolve(ctx context.Context, name string) (topic, error) {
	kind, rest, _ := strings.Cut(name, ":")
	slug, key, _ := strings.Cut(rest, "/")
	h := c.hub
	wc, err := c.workspace(ctx, slug)
	if err != nil {
		return topic{}, err
	}
	t := topic{name: name, kind: kind, workspace: wc.Workspace.ID}
	switch kind {
	case topicWorkspace:
		if key != "" {
			return t, apperr.NotFound("topic_not_found")
		}
		sees, err := h.o.Access.SeesWorkspace(ctx, c.principal, wc.Workspace.ID)
		if err != nil {
			return t, err
		}
		c.mu.Lock()
		c.sees[wc.Workspace.ID] = sees
		c.mu.Unlock()
	case topicProject:
		pc, err := h.o.Projects.Enter(ctx, wc, key)
		if err != nil {
			return t, err
		}
		t.project = pc.Project.ID
	case topicIssue:
		ic, err := h.o.Issues.Enter(ctx, wc, key)
		if err != nil {
			return t, err
		}
		t.project, t.issue = ic.Issue.ProjectID, ic.Issue.ID
	default:
		return t, apperr.NotFound("topic_not_found")
	}
	return t, nil
}

func (c *conn) workspace(ctx context.Context, slug string) (workspace.Context, error) {
	wc, err := c.hub.o.Workspaces.Enter(ctx, c.principal, slug)
	if err != nil {
		return wc, err
	}
	c.mu.Lock()
	c.wcs[wc.Workspace.ID] = wc
	c.mu.Unlock()
	return wc, nil
}

// watches reports whether a topic of the socket is in the workspace.
func (c *conn) watches(ws uuid.UUID) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range c.topics {
		if t.workspace == ws {
			return true
		}
	}
	return false
}

// forget drops cached permissions in a workspace after an access change.
func (c *conn) forget(ws uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.visible)
	delete(c.sees, ws)
}

// offer sends an event if a topic matches and the recipient may see it.
func (c *conn) offer(ctx context.Context, r routed) {
	ws := r.e.WorkspaceID.UUID
	c.mu.Lock()
	match := false
	for _, t := range c.topics {
		if t.workspace != ws {
			continue
		}
		switch t.kind {
		case topicWorkspace:
			match = true
		case topicProject:
			match = match || slices.Contains(r.projects, t.project)
		case topicIssue:
			match = match || (r.issueID != uuid.Nil && r.issueID == t.issue)
		}
	}
	wc, entered := c.wcs[ws]
	c.mu.Unlock()
	if !match || !entered || (r.onlyUser != uuid.Nil && r.onlyUser != c.principal.User.ID) {
		return
	}
	if c.allowed(ctx, wc, r) {
		c.push(r.data)
	}
}

// allowed checks the event against the recipient's current access: a
// project's events need its visibility (and issue.view for issues),
// workspace-wide ones need a workspace grant (guests don't see them).
func (c *conn) allowed(ctx context.Context, wc workspace.Context, r routed) bool {
	if len(r.projects) == 0 {
		c.mu.Lock()
		sees, ok := c.sees[wc.Workspace.ID]
		c.mu.Unlock()
		if !ok {
			s, err := c.hub.o.Access.SeesWorkspace(ctx, c.principal, wc.Workspace.ID)
			if err != nil {
				return false
			}
			c.mu.Lock()
			c.sees[wc.Workspace.ID] = s
			c.mu.Unlock()
			sees = s
		}
		return sees
	}
	for _, p := range r.projects {
		if c.canSee(ctx, wc, p, r.issueID != uuid.Nil) {
			return true
		}
	}
	return false
}

func (c *conn) canSee(ctx context.Context, wc workspace.Context, projectID uuid.UUID, issues bool) bool {
	key := [2]uuid.UUID{projectID}
	if issues {
		key[1] = uuid.Max
	}
	c.mu.Lock()
	v, ok := c.visible[key]
	c.mu.Unlock()
	if ok && time.Now().Before(v.expires) {
		return v.ok
	}
	allowed := false
	if pc, err := c.hub.o.Projects.EnterByID(ctx, wc, projectID); err == nil {
		allowed = !issues
		if issues {
			allowed, _ = c.hub.o.Access.Can(ctx, c.principal, pc.Scope(), access.IssueView)
		}
	}
	c.mu.Lock()
	c.visible[key] = visibility{ok: allowed, expires: time.Now().Add(visibleTTL)}
	c.mu.Unlock()
	return allowed
}

// replay sends the events of the new topics after since, from the event
// log; too old or too many and the client is told to refetch instead.
func (c *conn) replay(ctx context.Context, since string, added []topic) {
	names := make([]string, len(added))
	for i, t := range added {
		names[i] = t.name
	}
	resync := func() { c.reply(message{T: "resync", D: map[string]any{"topics": names}}) }
	id, err := uuid.Parse(since)
	if err != nil || id.Version() != 7 {
		resync()
		return
	}
	sec, nsec := id.Time().UnixTime()
	if time.Since(time.Unix(sec, nsec)) > replayWindow {
		resync()
		return
	}
	workspaces := map[uuid.UUID]bool{}
	for _, t := range added {
		workspaces[t.workspace] = true
	}
	// Every topic of the socket gets the replayed events: the client drops
	// those it already has, by id.
	for ws := range workspaces {
		events, err := eventlog.After(ctx, c.hub.o.Pool, ws, id, replayLimit+1)
		if err != nil || len(events) > replayLimit {
			resync()
			return
		}
		for _, e := range events {
			c.hub.dispatch(ctx, e, c)
		}
	}
}
