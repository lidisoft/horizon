package access_test

import (
	"context"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

type fixture struct {
	t     *testing.T
	pool  *pgxpool.Pool
	ids   *identity.Service
	acc   *access.Service
	ws    *workspace.Service
	owner identity.Principal // created the workspace; not an instance admin
	acme  workspace.Workspace
	roles map[string]uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testenv.DB(t)
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	f := &fixture{t: t, pool: pool}
	f.ids = identity.New(identity.Options{Pool: pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	f.acc = access.New(pool, nil)
	f.ws = workspace.New(workspace.Options{Pool: pool, Log: zerolog.Nop(), Identity: f.ids, Access: f.acc, Mail: &platformmail.Memory{}, BaseURL: base})
	f.owner = f.principal("owner@example.com")
	var err error
	f.acme, err = f.ws.Create(context.Background(), uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: f.owner.User.ID})
	if err != nil {
		t.Fatal(err)
	}
	roles, err := f.acc.Roles(context.Background(), f.acme.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.roles = map[string]uuid.UUID{}
	for _, r := range roles {
		f.roles[strings.ToLower(r.Name)] = r.ID
	}
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

// member adds a new account to acme with the default role.
func (f *fixture) member(email string) identity.Principal {
	f.t.Helper()
	p := f.principal(email)
	if err := f.ws.AddMember(context.Background(), f.acme.ID, p.User.ID); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) scope() access.Scope { return access.Scope{WorkspaceID: f.acme.ID} }

func (f *fixture) can(p identity.Principal, scope access.Scope, perm access.Permission) bool {
	f.t.Helper()
	ok, err := f.acc.Can(context.Background(), p, scope, perm)
	if err != nil {
		f.t.Fatal(err)
	}
	return ok
}

func (f *fixture) grant(roleKey string, user, team, project uuid.NullUUID) access.Grant {
	f.t.Helper()
	g, err := f.acc.SetGrant(context.Background(), f.acme.ID, uuid.NullUUID{}, access.GrantInput{RoleID: f.roles[roleKey], UserID: user, TeamID: team, ProjectID: project})
	if err != nil {
		f.t.Fatal(err)
	}
	return g
}

func some(id uuid.UUID) uuid.NullUUID { return uuid.NullUUID{UUID: id, Valid: true} }

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestDefaultRoles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	roles, _ := f.acc.Roles(ctx, f.acme.ID)
	var names []string
	for _, r := range roles {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, " "); got != "Admin Facilitator Contributor Viewer Guest" {
		t.Errorf("roles = %s", got)
	}
	if roles[0].People != 1 {
		t.Errorf("admin holders = %d", roles[0].People)
	}

	// The creator is Admin: every permission, and the UI's capability list.
	perms, _ := f.acc.Permissions(ctx, f.owner, f.scope())
	if len(perms) != 28 {
		t.Errorf("owner holds %d permissions", len(perms))
	}

	ada := f.member("ada@example.com") // default role: Contributor
	for perm, want := range map[access.Permission]bool{
		access.IssueCreate: true, access.IssueTransition: true, access.IssueDelete: false,
		access.IssueRank: false, access.WorkspaceManage: false, access.WorkspaceMembersManage: false,
	} {
		if got := f.can(ada, f.scope(), perm); got != want {
			t.Errorf("contributor %s = %v", perm, got)
		}
	}
}

func TestTeamAndProjectGrants(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	wc, _ := f.ws.Enter(ctx, f.owner, "acme")
	ada := f.member("ada@example.com")

	// A team's grant adds to the member's own.
	team, err := f.ws.CreateTeam(ctx, wc, workspace.TeamInput{Name: ptr("Leads")})
	if err != nil {
		t.Fatal(err)
	}
	f.grant("facilitator", uuid.NullUUID{}, some(team.ID), uuid.NullUUID{})
	if f.can(ada, f.scope(), access.IterationManage) {
		t.Fatal("not in the team yet")
	}
	if err := f.ws.SetTeamMember(ctx, wc, team.ID, ada.User.ID, true); err != nil {
		t.Fatal(err)
	}
	if !f.can(ada, f.scope(), access.IterationManage) || !f.can(ada, f.scope(), access.IssueCreate) {
		t.Error("team grant not applied")
	}

	// A project grant applies to that project only.
	projects := project.New(f.pool, f.acc, nil)
	newProject := func(key string) uuid.UUID {
		p, err := projects.Create(ctx, wc, project.Input{Key: &key, Name: &key})
		if err != nil {
			t.Fatal(err)
		}
		return p.ID
	}
	secret, other := newProject("SEC"), newProject("OTH")
	onProject := access.Scope{WorkspaceID: f.acme.ID, ProjectID: some(secret)}
	grace := f.principal("grace@example.com")
	if err := f.ws.AddMember(ctx, f.acme.ID, grace.User.ID); err != nil {
		t.Fatal(err)
	}
	grants, _ := f.acc.Grants(ctx, f.acme.ID)
	for _, g := range grants {
		if g.UserID.UUID == grace.User.ID {
			if err := f.acc.RemoveGrant(ctx, f.acme.ID, uuid.NullUUID{}, g.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	f.grant("guest", some(grace.User.ID), uuid.NullUUID{}, some(secret))
	if !f.can(grace, onProject, access.IssueView) || f.can(grace, f.scope(), access.IssueView) {
		t.Error("project grant leaks to the workspace, or doesn't apply")
	}
	if f.can(grace, access.Scope{WorkspaceID: f.acme.ID, ProjectID: some(other)}, access.IssueView) {
		t.Error("project grant applies to another project")
	}

	// Guests don't see the member list or teams.
	gc, err := f.ws.Enter(ctx, grace, "acme")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.ws.Members(ctx, gc)
	wantCode(t, err, "forbidden")
	_, err = f.ws.Teams(ctx, gc)
	wantCode(t, err, "forbidden")

	// Suspended members hold nothing, grants kept for their return.
	if err := f.ws.SetSuspended(ctx, wc, ada.User.ID, true); err != nil {
		t.Fatal(err)
	}
	if f.can(ada, f.scope(), access.IssueView) {
		t.Error("suspended member keeps permissions")
	}
	if err := f.ws.SetSuspended(ctx, wc, ada.User.ID, false); err != nil {
		t.Fatal(err)
	}
	if !f.can(ada, f.scope(), access.IterationManage) {
		t.Error("reactivated member lost their grants")
	}

	// Deleting the team and removing the member remove their grants.
	if err := f.ws.DeleteTeam(ctx, wc, team.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.ws.RemoveMember(ctx, wc, grace.User.ID); err != nil {
		t.Fatal(err)
	}
	grants, _ = f.acc.Grants(ctx, f.acme.ID)
	if len(grants) != 2 { // owner Admin, ada Contributor
		t.Errorf("grants left = %+v", grants)
	}
}

func TestTokenScopes(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	write := f.owner
	write.Scope = identity.ScopeWrite
	wantCode(t, access.Require(ctx, f.acc, write, f.scope(), access.WorkspaceManage), "insufficient_scope")
	if err := access.Require(ctx, f.acc, write, f.scope(), access.IssueCreate); err != nil {
		t.Errorf("write token, issue.create: %v", err)
	}
	ada := f.member("ada@example.com")
	wantCode(t, access.Require(ctx, f.acc, ada, f.scope(), access.IssueDelete), "forbidden")
}

func TestLockOutGuards(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	admin := f.roles["admin"]
	none := uuid.NullUUID{}

	perms := []access.Permission{access.WorkspaceManage}
	_, err := f.acc.UpdateRole(ctx, f.acme.ID, none, admin, 0, access.RoleInput{Permissions: &perms})
	wantCode(t, err, "admin_keeps_roles_manage")
	wantCode(t, f.acc.DeleteRole(ctx, f.acme.ID, none, admin), "admin_role_required")

	grants, _ := f.acc.Grants(ctx, f.acme.ID)
	ownerGrant := grants[0]
	wantCode(t, f.acc.RemoveGrant(ctx, f.acme.ID, none, ownerGrant.ID), "last_admin")
	_, err = f.acc.SetGrant(ctx, f.acme.ID, none, access.GrantInput{RoleID: f.roles["viewer"], UserID: some(f.owner.User.ID)})
	wantCode(t, err, "last_admin")

	// With a second Admin, the first can step down.
	ada := f.member("ada@example.com")
	f.grant("admin", some(ada.User.ID), none, none)
	if _, err := f.acc.SetGrant(ctx, f.acme.ID, none, access.GrantInput{RoleID: f.roles["viewer"], UserID: some(f.owner.User.ID)}); err != nil {
		t.Fatal(err)
	}
	if f.can(f.owner, f.scope(), access.WorkspaceManage) {
		t.Error("the owner is still admin")
	}
}

func TestCustomRoles(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	none := uuid.NullUUID{}
	bad := []access.Permission{"issue.fly"}
	_, err := f.acc.CreateRole(ctx, f.acme.ID, none, access.RoleInput{Name: ptr("QA"), Permissions: &bad})
	if e, ok := apperr.As(err); !ok || e.Fields[0].Code != "unknown" {
		t.Fatalf("unknown permission: %v", err)
	}
	perms := []access.Permission{access.IssueView, access.IssueTransition, access.IssueView}
	qa, err := f.acc.CreateRole(ctx, f.acme.ID, none, access.RoleInput{Name: ptr("QA"), Permissions: &perms})
	if err != nil || len(qa.Permissions) != 2 {
		t.Fatalf("qa = %+v, %v", qa, err)
	}
	_, err = f.acc.CreateRole(ctx, f.acme.ID, none, access.RoleInput{Name: ptr("qa")})
	if e, ok := apperr.As(err); !ok || e.Fields[0].Code != "taken" {
		t.Fatalf("duplicate name: %v", err)
	}

	ada := f.member("ada@example.com")
	f.roles["qa"] = qa.ID
	f.grant("qa", some(ada.User.ID), none, none)
	if !f.can(ada, f.scope(), access.IssueTransition) || f.can(ada, f.scope(), access.IssueCreate) {
		t.Error("custom role not applied")
	}
	more := append(perms, access.IssueCreate)
	if _, err := f.acc.UpdateRole(ctx, f.acme.ID, none, qa.ID, qa.Version, access.RoleInput{Permissions: &more}); err != nil {
		t.Fatal(err)
	}
	if !f.can(ada, f.scope(), access.IssueCreate) {
		t.Error("role change not applied")
	}
	wantCode(t, f.acc.DeleteRole(ctx, f.acme.ID, none, qa.ID), "role_in_use")
}

func TestInvitingCannotEscalate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	none := uuid.NullUUID{}
	perms := []access.Permission{access.WorkspaceMembersManage}
	inviter, err := f.acc.CreateRole(ctx, f.acme.ID, none, access.RoleInput{Name: ptr("Inviter"), Permissions: &perms})
	if err != nil {
		t.Fatal(err)
	}
	ada := f.member("ada@example.com")
	f.roles["inviter"] = inviter.ID
	f.grant("inviter", some(ada.User.ID), none, none)
	wc, _ := f.ws.Enter(ctx, ada, "acme")

	_, err = f.ws.InvitePeople(ctx, wc, workspace.Invite{Emails: []string{"x@example.com"}, RoleID: some(f.roles["admin"])})
	wantCode(t, err, "forbidden")
	invs, err := f.ws.InvitePeople(ctx, wc, workspace.Invite{Emails: []string{"x@example.com"}})
	if err != nil || invs[0].RoleID.UUID != f.roles["contributor"] {
		t.Fatalf("default role invite = %+v, %v", invs, err)
	}
	// Workspace admins choose any role.
	oc, _ := f.ws.Enter(ctx, f.owner, "acme")
	if _, err := f.ws.InvitePeople(ctx, oc, workspace.Invite{Emails: []string{"y@example.com"}, RoleID: some(f.roles["facilitator"])}); err != nil {
		t.Fatal(err)
	}
	caps, _ := f.ws.Capabilities(ctx, wc)
	if !slices.Equal(caps, []string{"workspace.members.manage"}) {
		t.Errorf("capabilities = %v", caps)
	}
}

func ptr(s string) *string { return &s }
