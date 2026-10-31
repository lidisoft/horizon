package access

// Area groups permissions for display.
type Area struct {
	Key         string       `json:"key" example:"workspace"`
	Permissions []Permission `json:"permissions"`
}

// Catalogue lists every permission, by area, in display order
// (docs/security.md#permission-catalogue-v1).
var Catalogue = []Area{
	{"workspace", []Permission{WorkspaceManage, WorkspaceMembersManage, WorkspaceRolesManage, WorkspaceConfigManage, WorkspaceWebhooksManage, WorkspaceIntegrationsManage}},
	{"project", []Permission{ProjectCreate, ProjectView, ProjectManage, ProjectConfigManage}},
	{"issue", []Permission{IssueView, IssueCreate, IssueEdit, IssueTransition, IssueAssign, IssueEstimate, IssueRank, IssueLink, IssueMove, IssueArchive, IssueDelete, IssueComment, IssueCommentModerate, IssueAttach}},
	{"planning", []Permission{IterationManage, ReleaseManage, MilestoneManage}},
	{"view", []Permission{ViewShare}},
}

var known = func() map[Permission]bool {
	m := map[Permission]bool{}
	for _, a := range Catalogue {
		for _, p := range a.Permissions {
			m[p] = true
		}
	}
	return m
}()

// Known reports whether p is in the catalogue.
func Known(p Permission) bool { return known[p] }

// Built-in role keys.
const (
	BuiltinAdmin       = "admin"
	BuiltinFacilitator = "facilitator"
	BuiltinContributor = "contributor"
	BuiltinViewer      = "viewer"
	BuiltinGuest       = "guest"
)

// DefaultRoleForInvites is the role newcomers get when the inviter doesn't
// choose one.
const DefaultRoleForInvites = BuiltinContributor

type defaultRole struct {
	builtin, name, description string
	permissions                []Permission
}

func all() []Permission {
	var out []Permission
	for _, a := range Catalogue {
		out = append(out, a.Permissions...)
	}
	return out
}

// defaultRoles are seeded in every new workspace and editable afterwards
// (docs/security.md#default-roles). Migration 00006 seeds the same sets into
// workspaces that existed before roles.
var defaultRoles = []defaultRole{
	{BuiltinAdmin, "Admin", "Everything, including workspace configuration, roles and members", all()},
	{BuiltinFacilitator, "Facilitator", "Contributor, plus planning, ranking and project boards", []Permission{
		ProjectView, ProjectConfigManage,
		IssueView, IssueCreate, IssueEdit, IssueTransition, IssueAssign, IssueEstimate, IssueRank, IssueLink,
		IssueMove, IssueArchive, IssueComment, IssueCommentModerate, IssueAttach,
		IterationManage, ReleaseManage, MilestoneManage, ViewShare,
	}},
	{BuiltinContributor, "Contributor", "Create, edit, transition, comment on and archive issues", []Permission{
		ProjectView,
		IssueView, IssueCreate, IssueEdit, IssueTransition, IssueAssign, IssueEstimate, IssueLink, IssueArchive,
		IssueComment, IssueAttach,
	}},
	{BuiltinViewer, "Viewer", "Read and comment", []Permission{ProjectView, IssueView, IssueComment}},
	{BuiltinGuest, "Guest", "Read and comment on the projects shared with them; no member list", []Permission{ProjectView, IssueView, IssueComment}},
}
