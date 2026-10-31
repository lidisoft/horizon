package workspace_test

import (
	"context"
	"net/netip"
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
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	t     *testing.T
	pool  *pgxpool.Pool
	ids   *identity.Service
	svc   *workspace.Service
	mail  *platformmail.Memory
	clock *clock
	admin identity.Principal // instance admin, first member of acme
	acme  workspace.Workspace
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, pool: testenv.DB(t), mail: &platformmail.Memory{}, clock: &clock{now: time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)}}
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f.ids = identity.New(identity.Options{Pool: f.pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars", Now: f.clock.Now})
	f.svc = workspace.New(workspace.Options{
		Pool: f.pool, Log: zerolog.Nop(), Identity: f.ids, Access: access.New(f.pool, f.clock.Now), Mail: f.mail, BaseURL: base, Now: f.clock.Now,
	})
	f.admin = f.principal("admin@example.com", true)
	var err error
	f.acme, err = f.svc.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.admin.User.ID})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// principal creates an account and signs it in.
func (f *fixture) principal(email string, admin bool) identity.Principal {
	f.t.Helper()
	ctx := context.Background()
	if _, err := f.ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: email, Name: strings.Split(email, "@")[0], Password: password, InstanceAdmin: admin}); err != nil {
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

func (f *fixture) enter(p identity.Principal) workspace.Context {
	f.t.Helper()
	wc, err := f.svc.Enter(context.Background(), p, "acme")
	if err != nil {
		f.t.Fatal(err)
	}
	return wc
}

// invite invites email to acme as the admin and returns the token from the
// email sent.
func (f *fixture) invite(email string, teams ...uuid.UUID) string {
	f.t.Helper()
	before := len(f.mail.Sent())
	if _, err := f.svc.InvitePeople(context.Background(), f.enter(f.admin), workspace.Invite{Emails: []string{email}, TeamIDs: teams}); err != nil {
		f.t.Fatal(err)
	}
	var sent []platformmail.Message
	for deadline := time.Now().Add(5 * time.Second); len(sent) <= before && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		sent = f.mail.Sent()
	}
	if len(sent) <= before {
		f.t.Fatal("no invitation email")
	}
	last := sent[len(sent)-1]
	_, token, ok := strings.Cut(last.Body, "https://horizon.example.com/invitation#token=")
	if !ok || !strings.EqualFold(last.To.Address, email) {
		f.t.Fatalf("invitation email = %+v", last)
	}
	return strings.Fields(token)[0]
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func wantField(t *testing.T, err error, field, code string) {
	t.Helper()
	if e, ok := apperr.As(err); ok {
		for _, f := range e.Fields {
			if f.Field == field && f.Code == code {
				return
			}
		}
	}
	t.Fatalf("err = %v, want %s: %s", err, field, code)
}

func TestCreateAndEnter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "Acme", Name: "Again", OwnerID: f.admin.User.ID})
	wantField(t, err, "slug", "taken")
	_, err = f.svc.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "1-bad--", Name: "", TimeZone: "Mars/Base", OwnerID: f.admin.User.ID})
	wantField(t, err, "slug", "invalid")
	wantField(t, err, "name", "required")
	wantField(t, err, "time_zone", "unknown")

	list, err := f.svc.ForUser(ctx, f.admin.User.ID)
	if err != nil || len(list) != 1 || list[0].Slug != "acme" || list[0].TimeZone != "UTC" {
		t.Fatalf("workspaces = %+v, %v", list, err)
	}
	wc := f.enter(f.admin)
	if !wc.Member {
		t.Error("the owner is not a member")
	}

	// Outsiders get not-found, instance admins get in without membership.
	outsider := f.principal("eve@example.com", false)
	_, err = f.svc.Enter(ctx, outsider, "acme")
	wantCode(t, err, "workspace_not_found")
	_, err = f.svc.Enter(ctx, outsider, "nowhere")
	wantCode(t, err, "workspace_not_found")
	root := f.principal("root@example.com", true)
	if wc, err := f.svc.Enter(ctx, root, "ACME"); err != nil || wc.Member {
		t.Errorf("instance admin: %+v, %v", wc, err)
	}

	events, _ := eventlog.History(ctx, f.pool, "workspace", f.acme.ID)
	if len(events) != 1 || events[0].PublicType() != "workspace.created" || events[0].WorkspaceID.UUID != f.acme.ID {
		t.Errorf("events = %+v", events)
	}
}

func TestUpdateWorkspace(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	name, slug, tz := "Acme Corp", "acme-corp", "Europe/Paris"
	w, err := f.svc.UpdateWorkspace(ctx, f.enter(f.admin), f.acme.Version, workspace.Update{Name: &name, Slug: &slug, TimeZone: &tz})
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != name || w.Slug != slug || w.TimeZone != tz || w.Version != 2 {
		t.Errorf("workspace = %+v", w)
	}
	wc, err := f.svc.Enter(ctx, f.admin, "acme-corp")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.UpdateWorkspace(ctx, wc, 1, workspace.Update{Name: &name})
	wantCode(t, err, "version_conflict")

	// Members without the permission can't.
	member := f.principal("ada@example.com", false)
	if err := f.svc.AddMember(ctx, f.acme.ID, member.User.ID); err != nil {
		t.Fatal(err)
	}
	mc, _ := f.svc.Enter(ctx, member, "acme-corp")
	_, err = f.svc.UpdateWorkspace(ctx, mc, 0, workspace.Update{Name: &name})
	wantCode(t, err, "forbidden")
}

func TestInvitationNewAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	team, err := f.svc.CreateTeam(ctx, f.enter(f.admin), workspace.TeamInput{Name: ptr("Core")})
	if err != nil {
		t.Fatal(err)
	}
	token := f.invite("ada@example.com", team.ID)

	preview, err := f.svc.Preview(ctx, token)
	if err != nil || preview.Workspace.Slug != "acme" || preview.Email != "ada@example.com" || preview.AccountExists || preview.InviterName != "admin" {
		t.Fatalf("preview = %+v, %v", preview, err)
	}

	_, err = f.svc.Accept(ctx, workspace.Acceptance{Token: token, Name: "Ada"})
	wantField(t, err, "password", "required")
	_, err = f.svc.Accept(ctx, workspace.Acceptance{Token: token, Name: "Ada", Password: "short"})
	wantField(t, err, "password", "too_short")

	out, err := f.svc.Accept(ctx, workspace.Acceptance{Token: token, Name: "Ada Lovelace", Password: password, Client: identity.Client{IP: netip.MustParseAddr("192.0.2.1")}})
	if err != nil {
		t.Fatal(err)
	}
	if out.SignedIn == nil || out.Workspace.Slug != "acme" {
		t.Fatalf("accepted = %+v", out)
	}
	p, err := f.ids.Authenticate(ctx, out.SignedIn.Token)
	if err != nil || p.User.Name != "Ada Lovelace" {
		t.Fatalf("new session: %+v, %v", p, err)
	}
	members, _ := f.svc.Members(ctx, f.enter(p))
	var ada workspace.Member
	for _, m := range members {
		if m.Email == "ada@example.com" {
			ada = m
		}
	}
	if ada.Status != "active" || len(ada.Teams) != 1 || ada.Teams[0].Name != "Core" || ada.LastSeenAt == nil {
		t.Errorf("ada = %+v", ada)
	}

	// The link works once.
	_, err = f.svc.Accept(ctx, workspace.Acceptance{Token: token, Name: "Twin", Password: password})
	wantCode(t, err, "invitation_invalid")
	_, err = f.svc.Preview(ctx, token)
	wantCode(t, err, "invitation_invalid")
}

func TestInvitationExistingAccount(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	grace := f.principal("grace@example.com", false)
	other := f.principal("eve@example.com", false)
	token := f.invite("Grace@Example.com")

	if pv, _ := f.svc.Preview(ctx, token); !pv.AccountExists {
		t.Error("preview: account not found")
	}
	_, err := f.svc.Accept(ctx, workspace.Acceptance{Token: token, Name: "Grace", Password: password})
	wantCode(t, err, "sign_in_required")
	_, err = f.svc.Accept(ctx, workspace.Acceptance{Token: token, Principal: &other})
	wantCode(t, err, "email_mismatch")
	out, err := f.svc.Accept(ctx, workspace.Acceptance{Token: token, Principal: &grace})
	if err != nil || out.SignedIn != nil {
		t.Fatalf("accept signed in: %+v, %v", out, err)
	}
	if _, err := f.svc.Enter(ctx, grace, "acme"); err != nil {
		t.Errorf("grace can't enter: %v", err)
	}

	// Members can't be invited again.
	_, err = f.svc.InvitePeople(ctx, f.enter(f.admin), workspace.Invite{Emails: []string{"grace@example.com"}})
	wantField(t, err, "emails[0]", "already_member")
}

func TestInvitationLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.enter(f.admin)

	_, err := f.svc.InvitePeople(ctx, wc, workspace.Invite{Emails: []string{"ok@example.com", "Bad <b@x.io>", "nope"}})
	wantField(t, err, "emails[1]", "invalid")
	wantField(t, err, "emails[2]", "invalid")
	_, err = f.svc.InvitePeople(ctx, wc, workspace.Invite{Emails: []string{"a@example.com"}, TeamIDs: []uuid.UUID{uuid.Must(uuid.NewV7())}})
	wantField(t, err, "team_ids", "unknown")

	first := f.invite("lucas@example.com")
	again := f.invite("LUCAS@example.com") // renews the same invitation
	list, err := f.svc.Invitations(ctx, wc)
	if err != nil || len(list) != 1 {
		t.Fatalf("invitations = %+v, %v", list, err)
	}
	_, err = f.svc.Preview(ctx, first)
	wantCode(t, err, "invitation_invalid") // the old link died with the renewal
	if _, err := f.svc.Preview(ctx, again); err != nil {
		t.Fatal(err)
	}

	// Seven days later the link has expired; resending renews it.
	f.clock.Advance(workspace.InvitationLifetime)
	_, err = f.svc.Preview(ctx, again)
	wantCode(t, err, "invitation_invalid")
	if list, _ := f.svc.Invitations(ctx, wc); !list[0].Expired {
		t.Error("expired invitation not flagged")
	}
	if _, err := f.svc.Resend(ctx, wc, list[0].ID); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.Revoke(ctx, wc, list[0].ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.Revoke(ctx, wc, list[0].ID), "invitation_not_found")
	if list, _ := f.svc.Invitations(ctx, wc); len(list) != 0 {
		t.Errorf("after revoke: %+v", list)
	}

	// Inviting needs the permission.
	member := f.principal("ada@example.com", false)
	if err := f.svc.AddMember(ctx, f.acme.ID, member.User.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.InvitePeople(ctx, f.enter(member), workspace.Invite{Emails: []string{"x@example.com"}})
	wantCode(t, err, "forbidden")

	events, _ := eventlog.History(ctx, f.pool, "invitation", list[0].ID)
	var types []string
	for _, e := range events {
		types = append(types, e.PublicType())
	}
	if got := strings.Join(types, " "); got != "invitation.sent invitation.resent invitation.resent invitation.revoked" {
		t.Errorf("events = %s", got)
	}
}

func TestSuspendAndRemove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.enter(f.admin)
	ada := f.principal("ada@example.com", false)
	if err := f.svc.AddMember(ctx, f.acme.ID, ada.User.ID); err != nil {
		t.Fatal(err)
	}
	team, _ := f.svc.CreateTeam(ctx, wc, workspace.TeamInput{Name: ptr("Web")})
	if err := f.svc.SetTeamMember(ctx, wc, team.ID, ada.User.ID, true); err != nil {
		t.Fatal(err)
	}

	wantCode(t, f.svc.SetSuspended(ctx, wc, f.admin.User.ID, true), "cannot_change_self")
	if err := f.svc.SetSuspended(ctx, wc, ada.User.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.Enter(ctx, ada, "acme")
	wantCode(t, err, "workspace_not_found")
	if list, _ := f.svc.ForUser(ctx, ada.User.ID); len(list) != 0 {
		t.Errorf("suspended member lists %+v", list)
	}
	// Suspension keeps teams.
	if teams, _ := f.svc.Teams(ctx, wc); len(teams[0].Members) != 1 {
		t.Errorf("teams after suspension = %+v", teams)
	}
	if err := f.svc.SetSuspended(ctx, wc, ada.User.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Enter(ctx, ada, "acme"); err != nil {
		t.Errorf("reactivated: %v", err)
	}

	if err := f.svc.RemoveMember(ctx, wc, ada.User.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.RemoveMember(ctx, wc, ada.User.ID), "member_not_found")
	if teams, _ := f.svc.Teams(ctx, wc); len(teams[0].Members) != 0 {
		t.Errorf("removal kept team membership: %+v", teams)
	}
}

func TestTeams(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc := f.enter(f.admin)

	core, err := f.svc.CreateTeam(ctx, wc, workspace.TeamInput{Name: ptr(" Core "), Description: ptr("Backend")})
	if err != nil || core.Name != "Core" {
		t.Fatalf("team = %+v, %v", core, err)
	}
	_, err = f.svc.CreateTeam(ctx, wc, workspace.TeamInput{Name: ptr("core")})
	wantField(t, err, "name", "taken")
	_, err = f.svc.CreateTeam(ctx, wc, workspace.TeamInput{})
	wantField(t, err, "name", "required")

	if err := f.svc.UpdateTeam(ctx, wc, core.ID, core.Version, workspace.TeamInput{Name: ptr("Platform")}); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.UpdateTeam(ctx, wc, core.ID, core.Version, workspace.TeamInput{Name: ptr("X")}), "version_conflict")

	stranger := f.principal("eve@example.com", false)
	wantCode(t, f.svc.SetTeamMember(ctx, wc, core.ID, stranger.User.ID, true), "member_not_found")
	if err := f.svc.SetTeamMember(ctx, wc, core.ID, f.admin.User.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetTeamMember(ctx, wc, core.ID, f.admin.User.ID, true); err != nil {
		t.Fatal(err) // idempotent
	}
	teams, _ := f.svc.Teams(ctx, wc)
	if len(teams) != 1 || teams[0].Name != "Platform" || len(teams[0].Members) != 1 || teams[0].Version != 3 {
		t.Errorf("teams = %+v", teams)
	}
	if err := f.svc.DeleteTeam(ctx, wc, core.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, f.svc.DeleteTeam(ctx, wc, core.ID), "team_not_found")

	events, _ := eventlog.History(ctx, f.pool, "team", core.ID)
	var types []string
	for _, e := range events {
		types = append(types, e.PublicType())
	}
	if got := strings.Join(types, " "); got != "team.created team.updated team.member_added team.deleted" {
		t.Errorf("events = %s", got)
	}
}

func ptr(s string) *string { return &s }
