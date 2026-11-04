package notify_test

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/notify"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
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
	svc      *notify.Service
	mail     *platformmail.Memory
	owner    identity.Principal
	acme     workspace.Workspace
	now      time.Time
	last     uuid.UUID // the last event delivered
	pushed   []uuid.UUID
}

// newFixture starts the clock on Monday 2 November 2026, 09:00 UTC.
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t), now: time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC), mail: &platformmail.Memory{}}
	clock := func() time.Time { return f.now }
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(f.pool, nil)
	f.cfg = configdef.New(f.pool, acc, nil, nil)
	f.ws = workspace.New(workspace.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{f.cfg.SeedDefaults}})
	f.projects = project.New(f.pool, acc, nil)
	f.issues = issue.New(f.pool, acc, f.cfg, f.projects, clock)
	f.svc = notify.New(notify.Options{Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: acc, Workspaces: f.ws, Issues: f.issues,
		Mail: f.mail, BaseURL: base, Now: clock, Publish: func(id uuid.UUID, _ []byte) { f.pushed = append(f.pushed, id) }})
	f.owner = f.principal("owner@example.com")
	var err error
	if f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID}); err != nil {
		t.Fatal(err)
	}
	f.project("HZN", project.VisibilityWorkspace)
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

// member adds a Contributor to acme.
func (f *fixture) member(email string) identity.Principal {
	f.t.Helper()
	p := f.principal(email)
	if err := f.ws.AddMember(context.Background(), f.acme.ID, p.User.ID); err != nil {
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

func (f *fixture) project(key, visibility string) {
	f.t.Helper()
	if _, err := f.projects.Create(context.Background(), f.wc(f.owner), project.Input{Key: &key, Name: &key, Visibility: &visibility}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) enter(p identity.Principal, key string) issue.Context {
	f.t.Helper()
	ic, err := f.issues.Enter(context.Background(), f.wc(p), key)
	if err != nil {
		f.t.Fatal(err)
	}
	return ic
}

func (f *fixture) create(p identity.Principal, project, title string, assignee uuid.UUID) issue.Issue {
	f.t.Helper()
	pc, err := f.projects.Enter(context.Background(), f.wc(p), project)
	if err != nil {
		f.t.Fatal(err)
	}
	r, err := f.cfg.Resolve(context.Background(), f.pool, f.acme.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		f.t.Fatal(err)
	}
	in := issue.NewIssue{TypeID: r.IssueTypes[0].ID, Title: title}
	if assignee != uuid.Nil {
		in.AssigneeID = uuid.NullUUID{UUID: assignee, Valid: true}
	}
	i, err := f.issues.Create(context.Background(), pc, in)
	if err != nil {
		f.t.Fatal(err)
	}
	return i
}

func (f *fixture) comment(p identity.Principal, key string, text issue.Text) {
	f.t.Helper()
	if _, err := f.issues.AddComment(context.Background(), f.enter(p, key), text); err != nil {
		f.t.Fatal(err)
	}
}

// mention is a comment document mentioning the user.
func mention(id uuid.UUID, text string) issue.Text {
	doc, _ := json.Marshal(map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{
		map[string]any{"type": "mention", "attrs": map[string]any{"user": id.String()}},
		map[string]any{"type": "text", "text": " " + text},
	}}}})
	return issue.Text{Doc: doc}
}

// deliver hands the events since the last delivery to the consumer, as the
// worker would, the clock at each event's time.
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

func (f *fixture) inbox(p identity.Principal, state string) []notify.Item {
	f.t.Helper()
	page, err := f.svc.Inbox(context.Background(), f.wc(p), state, "", 0)
	if err != nil {
		f.t.Fatal(err)
	}
	return page.Items
}

func (f *fixture) subscription(p identity.Principal, key string) notify.Subscription {
	f.t.Helper()
	s, err := f.svc.Subscription(context.Background(), f.enter(p, key))
	if err != nil {
		f.t.Fatal(err)
	}
	return s
}

func TestAssignAndComment(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ann := f.member("ann@example.com")
	bob := f.member("bob@example.com")

	// The owner creates HZN-1 for Ann: Ann is told, the owner isn't.
	f.create(f.owner, "HZN", "Outbox relay", ann.User.ID)
	f.deliver()
	if got := f.subscription(f.owner, "HZN-1"); got.Reason != notify.ReasonCreator {
		t.Errorf("owner subscription = %+v", got)
	}
	if got := f.subscription(ann, "HZN-1"); got.Reason != notify.ReasonAssignee {
		t.Errorf("ann subscription = %+v", got)
	}
	if got := f.inbox(f.owner, "unread"); len(got) != 0 {
		t.Errorf("owner inbox = %+v", got)
	}
	items := f.inbox(ann, "unread")
	if len(items) != 1 || items[0].Issue.Key != "HZN-1" || items[0].Reason != notify.Assigned || items[0].Event == nil || items[0].Event.Type != "issue.created" {
		t.Fatalf("ann inbox = %+v", items)
	}

	// Ann comments mentioning Bob: the owner (creator) hears of it; Bob is
	// mentioned and follows from now on; Ann follows anyway.
	f.now = f.now.Add(time.Minute)
	f.comment(ann, "HZN-1", mention(bob.User.ID, "can you look?"))
	f.deliver()
	owner := f.inbox(f.owner, "unread")
	if len(owner) != 1 || owner[0].Reason != notify.Subscribed || owner[0].Event.Type != "issue.commented" {
		t.Fatalf("owner inbox = %+v", owner)
	}
	if got := f.inbox(bob, "unread"); len(got) != 1 || got[0].Reason != notify.Mentioned {
		t.Fatalf("bob inbox = %+v", got)
	}
	if got := f.subscription(bob, "HZN-1"); got.Reason != notify.ReasonMention {
		t.Errorf("bob subscription = %+v", got)
	}
	if got := f.inbox(ann, "unread"); len(got) != 1 || got[0].UnreadCount != 1 || got[0].Event.Type != "issue.created" {
		t.Errorf("ann's own comment changed her inbox: %+v", got)
	}

	// Redelivered events change nothing.
	f.last = uuid.Nil
	f.deliver()
	if got := f.inbox(f.owner, "unread"); len(got) != 1 || got[0].UnreadCount != 1 {
		t.Errorf("after redelivery: %+v", got)
	}

	// Bob reads it, then the owner moves it: unread again, from 1.
	if err := f.svc.UpdateItem(ctx, f.wc(bob), f.inbox(bob, "unread")[0].ID, notify.ItemUpdate{Read: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if got := f.inbox(bob, "unread"); len(got) != 0 {
		t.Fatalf("read row still unread: %+v", got)
	}
	if got := f.inbox(bob, "all"); len(got) != 1 || !got[0].Read {
		t.Fatalf("all = %+v", got)
	}
	ic := f.enter(f.owner, "HZN-1")
	if _, err := f.issues.Assign(ctx, ic, 0, bob.User.ID); err != nil {
		t.Fatal(err)
	}
	f.deliver()
	got := f.inbox(bob, "unread")
	if len(got) != 1 || got[0].Reason != notify.Assigned || got[0].UnreadCount != 1 {
		t.Fatalf("bob after the assignment = %+v", got)
	}
	if s := notify.Summary(*got[0].Event, bob.User.ID, got[0].Reason); s != "owner assigned it to you" {
		t.Errorf("summary = %q", s)
	}
	// Ann, unassigned, hears of it as a follower; her row counts 2 now and
	// keeps its stronger reason.
	if got := f.inbox(ann, "unread"); len(got) != 1 || got[0].UnreadCount != 2 || got[0].Reason != notify.Assigned {
		t.Errorf("ann = %+v", got)
	}
	if n, err := f.svc.UnreadCount(ctx, f.wc(ann)); err != nil || n != 1 {
		t.Errorf("count = %d, %v", n, err)
	}
	if len(f.pushed) == 0 {
		t.Error("no inbox change published")
	}
}

func TestUnsubscribeSnoozeArchive(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ann := f.member("ann@example.com")
	f.create(f.owner, "HZN", "Outbox relay", uuid.Nil)
	f.deliver()

	// Ann subscribes by hand, then unsubscribes: comments pass her by.
	if s, err := f.svc.Subscribe(ctx, f.enter(ann, "HZN-1")); err != nil || s.Reason != notify.ReasonManual {
		t.Fatalf("subscribe = %+v, %v", s, err)
	}
	f.comment(f.owner, "HZN-1", issue.PlainText("First"))
	f.deliver()
	items := f.inbox(ann, "unread")
	if len(items) != 1 {
		t.Fatalf("inbox = %+v", items)
	}

	// Snoozed until tomorrow: out of the inbox, in the snoozed list.
	tomorrow := f.now.Add(24 * time.Hour)
	if err := f.svc.UpdateItem(ctx, f.wc(ann), items[0].ID, notify.ItemUpdate{SnoozedUntil: notify.NullableTime{Set: true, Value: &tomorrow}}); err != nil {
		t.Fatal(err)
	}
	if got := f.inbox(ann, "unread"); len(got) != 0 {
		t.Errorf("snoozed row listed: %+v", got)
	}
	if got := f.inbox(ann, "snoozed"); len(got) != 1 {
		t.Errorf("snoozed = %+v", got)
	}
	past := f.now.Add(-time.Hour)
	if err := f.svc.UpdateItem(ctx, f.wc(ann), items[0].ID, notify.ItemUpdate{SnoozedUntil: notify.NullableTime{Set: true, Value: &past}}); err == nil {
		t.Error("snoozed into the past")
	}
	// New activity wakes it.
	f.comment(f.owner, "HZN-1", issue.PlainText("Second"))
	f.deliver()
	if got := f.inbox(ann, "unread"); len(got) != 1 || got[0].SnoozedUntil != nil || got[0].UnreadCount != 2 {
		t.Fatalf("woken = %+v", got)
	}

	// Archived (after reading): out of every list until new activity.
	if err := f.svc.ReadAll(ctx, f.wc(ann)); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ArchiveRead(ctx, f.wc(ann)); err != nil {
		t.Fatal(err)
	}
	if got := f.inbox(ann, "all"); len(got) != 0 {
		t.Errorf("archived row listed: %+v", got)
	}
	if err := f.svc.Unsubscribe(ctx, f.enter(ann, "HZN-1")); err != nil {
		t.Fatal(err)
	}
	f.comment(f.owner, "HZN-1", issue.PlainText("Third"))
	f.deliver()
	if got := f.inbox(ann, "all"); len(got) != 0 {
		t.Errorf("unsubscribed but told: %+v", got)
	}
	// A mention subscribes her again.
	f.comment(f.owner, "HZN-1", mention(ann.User.ID, "back"))
	f.deliver()
	if got := f.inbox(ann, "unread"); len(got) != 1 || got[0].Reason != notify.Mentioned {
		t.Errorf("mentioned = %+v", got)
	}
}

func TestOnlyVisibleIssuesNotify(t *testing.T) {
	f := newFixture(t)
	ann := f.member("ann@example.com")
	f.project("SEC", project.VisibilityPrivate)
	// Ann can't see SEC: a mention neither subscribes nor tells her.
	f.create(f.owner, "SEC", "Secret", uuid.Nil)
	f.comment(f.owner, "SEC-1", mention(ann.User.ID, "psst"))
	f.deliver()
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM inbox_items WHERE user_id = $1`, ann.User.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows = %d, %v", n, err)
	}
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM subscriptions WHERE user_id = $1`, ann.User.ID).Scan(&n); err != nil || n != 0 {
		t.Errorf("subscriptions = %d, %v", n, err)
	}
}

func TestEmails(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	ann := f.member("ann@example.com")
	bob := f.member("bob@example.com")
	f.create(f.owner, "HZN", "Outbox relay", ann.User.ID)
	f.create(f.owner, "HZN", "Replay after reconnect", bob.User.ID)
	f.deliver()

	// Nothing until the rows have been unread for the delay.
	if err := f.svc.SendDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.mail.Sent(); len(got) != 0 {
		t.Fatalf("sent early: %+v", got)
	}
	// Bob reads his meanwhile; Ann doesn't.
	if err := f.svc.ReadAll(ctx, f.wc(bob)); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(notify.EmailDelay)
	if err := f.svc.SendDue(ctx); err != nil {
		t.Fatal(err)
	}
	sent := f.mail.Sent()
	if len(sent) != 1 || sent[0].To.Address != "ann@example.com" || sent[0].Subject != "HZN-1 Outbox relay" {
		t.Fatalf("sent = %+v", sent)
	}
	for _, want := range []string{"owner created the issue", "https://horizon.example.com/w/acme/issues/HZN-1", "https://horizon.example.com/account"} {
		if !strings.Contains(sent[0].Body, want) {
			t.Errorf("body lacks %q:\n%s", want, sent[0].Body)
		}
	}
	// Told once.
	if err := f.svc.SendDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.mail.Sent(); len(got) != 1 {
		t.Errorf("sent again: %d", len(got))
	}

	// Ann turns email off: a new comment isn't emailed.
	if _, err := f.svc.SaveSettings(ctx, ann.User.ID, notify.Settings{Email: "off", Digest: true}); err != nil {
		t.Fatal(err)
	}
	f.comment(f.owner, "HZN-1", issue.PlainText("Ping"))
	f.deliver()
	f.now = f.now.Add(notify.EmailDelay)
	if err := f.svc.SendDue(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.mail.Sent(); len(got) != 1 {
		t.Errorf("emailed with email off: %d", len(got))
	}

	// Her digest goes out from 08:00 in her zone, once a day. It is 09:20
	// UTC: 10:20 in Paris, 04:20 in New York.
	zone := "America/New_York"
	if _, err := f.ids.UpdateProfile(ctx, ann.User.ID, 0, identity.ProfileUpdate{Timezone: &zone}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.mail.Sent(); len(got) != 1 {
		t.Fatalf("digest before the morning: %d", len(got))
	}
	f.now = time.Date(2026, 11, 2, 13, 5, 0, 0, time.UTC) // 08:05 in New York
	if err := f.svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	sent = f.mail.Sent()
	if len(sent) != 2 || sent[1].Subject != "Your Horizon digest: 1 unread" || !strings.Contains(sent[1].Body, "owner commented") {
		t.Fatalf("digest = %+v", sent[1:])
	}
	// The next morning, again.
	f.now = f.now.Add(24 * time.Hour)
	if err := f.svc.SendDigests(ctx); err != nil {
		t.Fatal(err)
	}
	if got := f.mail.Sent(); len(got) != 3 {
		t.Errorf("second digest: %d", len(got))
	}
}

func ptr[T any](v T) *T { return &v }
