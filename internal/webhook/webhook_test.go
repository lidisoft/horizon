package webhook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/webhook"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t        *testing.T
	pool     *pgxpool.Pool
	ids      *identity.Service
	ws       *workspace.Service
	cfg      *configdef.Service
	projects *project.Service
	issues   *issue.Service
	box      *secrets.Box
	svc      *webhook.Service
	owner    identity.Principal
	acme     workspace.Workspace
	now      time.Time
	last     uuid.UUID
}

func newFixture(t *testing.T, allowPrivate bool) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t), now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return f.now }
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.cfg = configdef.New(f.pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(f.pool, acc, nil)
	f.issues = issue.New(f.pool, acc, f.cfg, f.projects, clock)
	var err error
	if f.box, err = secrets.New(bytes.Repeat([]byte{7}, 32)); err != nil {
		t.Fatal(err)
	}
	f.svc = webhook.New(webhook.Options{Pool: f.pool, Log: zerolog.Nop(), Access: acc, Projects: f.projects, Issues: f.issues,
		Secrets: f.box, AllowPrivate: allowPrivate, Now: clock})
	f.owner = f.principal("owner@example.com")
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"HZN", "OPS"} {
		name, vis := key, project.VisibilityWorkspace
		if _, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &name, Visibility: &vis}); err != nil {
			t.Fatal(err)
		}
	}
	f.deliver() // the setup's events: no endpoint yet
	return f
}

func (f *fixture) principal(email string) identity.Principal {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: strings.Split(email, "@")[0], Password: password}); err != nil {
		f.t.Fatal(err)
	}
	in, err := f.ids.Login(ctx, email, password, identity.Client{})
	if err != nil {
		f.t.Fatal(err)
	}
	p, err := f.ids.Authenticate(ctx, in.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) wc(p identity.Principal) workspace.Context {
	f.t.Helper()
	wc, err := f.ws.Enter(context.Background(), p, "acme")
	if err != nil {
		f.t.Fatal(err)
	}
	return wc
}

func (f *fixture) create(key, title string) {
	f.t.Helper()
	ctx := context.Background()
	pc, err := f.projects.Enter(ctx, f.wc(f.owner), key)
	if err != nil {
		f.t.Fatal(err)
	}
	r, err := f.cfg.Resolve(ctx, f.pool, f.acme.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.issues.Create(ctx, pc, issue.NewIssue{TypeID: r.IssueTypes[0].ID, Title: title}); err != nil {
		f.t.Fatal(err)
	}
}

// deliver hands the events since the last delivery to the consumer.
func (f *fixture) deliver() {
	f.t.Helper()
	ctx := context.Background()
	rows, err := f.pool.Query(ctx, `
		SELECT id, workspace_id, aggregate_type, aggregate_id, aggregate_version, type, actor_id, occurred_at, payload
		FROM events WHERE id > $1 ORDER BY id`, f.last)
	if err != nil {
		f.t.Fatal(err)
	}
	var events []eventlog.Event
	for rows.Next() {
		var e eventlog.Event
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.AggregateType, &e.AggregateID, &e.AggregateVersion, &e.Type, &e.ActorID, &e.OccurredAt, &e.Payload); err != nil {
			f.t.Fatal(err)
		}
		events = append(events, e)
	}
	rows.Close()
	for _, e := range events {
		if err := f.svc.Handle(ctx, e); err != nil {
			f.t.Fatalf("%s: %v", e.Type, err)
		}
		f.last = e.ID
	}
}

func (f *fixture) deliveries(id uuid.UUID) []webhook.Delivery {
	f.t.Helper()
	page, err := f.svc.Deliveries(context.Background(), f.wc(f.owner), id, "", 0)
	if err != nil {
		f.t.Fatal(err)
	}
	return page.Items
}

// receiver records requests, answering with the next status.
type receiver struct {
	mu       sync.Mutex
	requests []*http.Request
	bodies   [][]byte
	status   int
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	b, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.bodies = append(r.bodies, b)
	status := r.status
	r.mu.Unlock()
	w.WriteHeader(status)
	_, _ = w.Write([]byte("thanks"))
}

func ptr[T any](v T) *T { return &v }

func TestWebhooks(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	rx := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rx)
	defer srv.Close()

	// Only workspace.webhooks.manage holders manage endpoints.
	ann := f.principal("ann@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, ann.User.ID); err != nil {
		t.Fatal(err)
	}
	f.deliver()
	if _, err := f.svc.List(ctx, f.wc(ann)); !apperr.Is(err, "forbidden") {
		t.Errorf("member listed webhooks: %v", err)
	}
	if _, err := f.svc.Create(ctx, f.wc(f.owner), webhook.Input{Name: ptr("x"), URL: ptr("ftp://example.com")}); !apperr.Is(err, "invalid_input") {
		t.Errorf("ftp accepted: %v", err)
	}

	// One endpoint for HZN's issue creations, one for everything.
	hzn, err := f.svc.Create(ctx, f.wc(f.owner), webhook.Input{Name: ptr("HZN created"), URL: ptr(srv.URL + "/hzn"), Project: ptr("HZN"),
		EventTypes: &[]string{"issue.created"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hzn.Secret, "whsec_") || hzn.Project != "HZN" {
		t.Fatalf("created = %+v", hzn)
	}
	all, err := f.svc.Create(ctx, f.wc(f.owner), webhook.Input{Name: ptr("Everything"), URL: ptr(srv.URL + "/all")})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.svc.Get(ctx, f.wc(f.owner), all.ID); got.Secret != "" {
		t.Error("the secret is shown again")
	}
	var sealed string
	if err := f.pool.QueryRow(ctx, `SELECT secret FROM webhooks WHERE id = $1`, all.ID).Scan(&sealed); err != nil || strings.Contains(sealed, all.Secret) {
		t.Errorf("stored secret = %q", sealed)
	}

	f.create("HZN", "Outbox relay")
	f.create("OPS", "Rotate keys")
	f.deliver()
	f.deliver() // again: nothing new
	if got := f.deliveries(hzn.ID); len(got) != 1 || got[0].EventType != "issue.created" {
		t.Fatalf("hzn deliveries = %+v", got)
	}
	// "Everything" has both issues and both endpoints' creation (workspace
	// events, which never name the address).
	if got := f.deliveries(all.ID); len(got) != 4 || got[3].EventType != "webhook.created" || strings.Contains(string(got[3].Body), srv.URL) {
		t.Fatalf("all deliveries = %+v", got)
	}

	if err := f.svc.Send(ctx); err != nil {
		t.Fatal(err)
	}
	if len(rx.requests) != 5 {
		t.Fatalf("requests = %d", len(rx.requests))
	}
	for i, r := range rx.requests {
		secret := all.Secret
		if r.URL.Path == "/hzn" {
			secret = hzn.Secret
		}
		if got := r.Header.Get("Horizon-Signature"); got != webhook.Sign(secret, rx.bodies[i]) {
			t.Errorf("signature %q", got)
		}
		if r.Header.Get("Horizon-Event") == "" || r.Header.Get("Horizon-Delivery") == "" {
			t.Errorf("headers = %v", r.Header)
		}
	}
	var body struct {
		Type      string
		Workspace string
		Aggregate struct{ Key string }
	}
	for i, r := range rx.requests {
		if r.URL.Path == "/hzn" {
			_ = json.Unmarshal(rx.bodies[i], &body)
		}
	}
	if body.Type != "issue.created" || body.Workspace != "acme" || body.Aggregate.Key != "HZN-1" {
		t.Errorf("body = %+v", body)
	}
	if got := f.deliveries(hzn.ID)[0]; got.State != "succeeded" || *got.StatusCode != 200 || got.ResponseExcerpt != "thanks" || got.Attempts != 1 {
		t.Errorf("delivery = %+v", got)
	}

	// A failing receiver: retried with backoff, given up after a day.
	rx.status = http.StatusBadGateway
	ping, err := f.svc.Ping(ctx, f.wc(f.owner), hzn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Send(ctx); err != nil {
		t.Fatal(err)
	}
	d := f.deliveries(hzn.ID)[0]
	if d.ID != ping.ID || d.State != "pending" || d.Attempts != 1 || d.NextAttemptAt == nil || !d.NextAttemptAt.Equal(f.now.Add(time.Minute)) {
		t.Fatalf("after one failure = %+v", d)
	}
	for f.now.Before(time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)) {
		f.now = f.now.Add(10 * time.Minute)
		if err := f.svc.Send(ctx); err != nil {
			t.Fatal(err)
		}
	}
	d = f.deliveries(hzn.ID)[0]
	if d.State != "failed" || d.Attempts < 5 || d.Attempts > 10 {
		t.Fatalf("after a day = %+v", d)
	}

	// Redelivered once the receiver is back.
	rx.status = http.StatusNoContent
	again, err := f.svc.Redeliver(ctx, f.wc(f.owner), hzn.ID, ping.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Send(ctx); err != nil {
		t.Fatal(err)
	}
	d = f.deliveries(hzn.ID)[0]
	if d.ID != again.ID || d.State != "succeeded" || d.RedeliveryOf == nil || *d.RedeliveryOf != ping.ID {
		t.Errorf("redelivery = %+v", d)
	}

	// Rotating the secret; turning an endpoint off stops its deliveries.
	rotated, err := f.svc.Update(ctx, f.wc(f.owner), hzn.ID, hzn.Version, webhook.Input{RotateSecret: true, Active: ptr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Secret == "" || rotated.Secret == hzn.Secret || rotated.Active {
		t.Errorf("rotated = %+v", rotated)
	}
	if _, err := f.svc.Update(ctx, f.wc(f.owner), hzn.ID, hzn.Version, webhook.Input{Name: ptr("stale")}); !apperr.Is(err, "version_conflict") {
		t.Errorf("stale update: %v", err)
	}
	f.create("HZN", "Replay")
	f.deliver()
	if got := f.deliveries(hzn.ID); len(got) != 3 {
		t.Errorf("an inactive endpoint got a delivery: %d", len(got))
	}
	if err := f.svc.Delete(ctx, f.wc(f.owner), hzn.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Get(ctx, f.wc(f.owner), hzn.ID); !apperr.Is(err, "webhook_not_found") {
		t.Errorf("deleted: %v", err)
	}
}

func TestPrivateAddressesRefused(t *testing.T) {
	f := newFixture(t, false)
	ctx := context.Background()
	rx := &receiver{status: http.StatusOK}
	srv := httptest.NewServer(rx)
	defer srv.Close()
	w, err := f.svc.Create(ctx, f.wc(f.owner), webhook.Input{Name: ptr("Local"), URL: ptr(srv.URL)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Ping(ctx, f.wc(f.owner), w.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Send(ctx); err != nil {
		t.Fatal(err)
	}
	d := f.deliveries(w.ID)[0]
	if len(rx.requests) != 0 || d.State != "pending" || !strings.Contains(d.Error, "address not allowed") {
		t.Errorf("delivery = %+v, requests = %d", d, len(rx.requests))
	}
}

func TestRekey(t *testing.T) {
	f := newFixture(t, true)
	ctx := context.Background()
	w, err := f.svc.Create(ctx, f.wc(f.owner), webhook.Input{Name: ptr("x"), URL: ptr("https://example.com/hook")})
	if err != nil {
		t.Fatal(err)
	}
	// A new key, the old one kept to decrypt.
	box, err := secrets.New(bytes.Repeat([]byte{8}, 32), bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	svc := webhook.New(webhook.Options{Pool: f.pool, Log: zerolog.Nop(), Secrets: box})
	if n, err := svc.Rekey(ctx); err != nil || n != 1 {
		t.Fatalf("rekey = %d, %v", n, err)
	}
	if n, err := svc.Rekey(ctx); err != nil || n != 0 {
		t.Errorf("second rekey = %d, %v", n, err)
	}
	var sealed string
	if err := f.pool.QueryRow(ctx, `SELECT secret FROM webhooks WHERE id = $1`, w.ID).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if got, err := box.Open(sealed, "webhooks:"+w.ID.String()); err != nil || got != w.Secret {
		t.Errorf("open = %q, %v", got, err)
	}
}
