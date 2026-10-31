// Package access decides what a caller may do (docs/security.md#authorisation).
// Code checks permission keys, never role names.
//
// Service is the Checker: roles are permission sets defined per workspace,
// and grants give them to members or teams on the workspace or a project.
package access

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity"
)

// Permission is a stable permission key.
type Permission string

// The v1 permission catalogue (docs/security.md#permission-catalogue-v1).
const (
	WorkspaceManage             Permission = "workspace.manage"
	WorkspaceMembersManage      Permission = "workspace.members.manage"
	WorkspaceRolesManage        Permission = "workspace.roles.manage"
	WorkspaceConfigManage       Permission = "workspace.config.manage"
	WorkspaceWebhooksManage     Permission = "workspace.webhooks.manage"
	WorkspaceIntegrationsManage Permission = "workspace.integrations.manage"

	ProjectCreate       Permission = "project.create"
	ProjectView         Permission = "project.view"
	ProjectManage       Permission = "project.manage"
	ProjectConfigManage Permission = "project.config.manage"

	IssueView            Permission = "issue.view"
	IssueCreate          Permission = "issue.create"
	IssueEdit            Permission = "issue.edit"
	IssueTransition      Permission = "issue.transition"
	IssueAssign          Permission = "issue.assign"
	IssueEstimate        Permission = "issue.estimate"
	IssueRank            Permission = "issue.rank"
	IssueLink            Permission = "issue.link"
	IssueMove            Permission = "issue.move"
	IssueArchive         Permission = "issue.archive"
	IssueDelete          Permission = "issue.delete"
	IssueComment         Permission = "issue.comment"
	IssueCommentModerate Permission = "issue.comment.moderate"
	IssueAttach          Permission = "issue.attach"

	IterationManage Permission = "iteration.manage"
	ReleaseManage   Permission = "release.manage"
	MilestoneManage Permission = "milestone.manage"

	ViewShare Permission = "view.share"
)

// Administrative reports whether p needs the admin token scope.
func (p Permission) Administrative() bool {
	return strings.HasPrefix(string(p), "workspace.")
}

// Scope is where a permission is checked: a workspace, and optionally one of
// its projects.
type Scope struct {
	WorkspaceID uuid.UUID
	ProjectID   uuid.NullUUID
}

// Checker answers permission questions.
type Checker interface {
	Can(ctx context.Context, p identity.Principal, scope Scope, perm Permission) (bool, error)
}

// Require returns nil when p holds perm in scope, else a 403: forbidden, or
// insufficient_scope for an API token without the admin scope asking for an
// administrative permission.
func Require(ctx context.Context, c Checker, p identity.Principal, scope Scope, perm Permission) error {
	if perm.Administrative() && !p.Scope.Includes(identity.ScopeAdmin) {
		return apperr.Forbidden("insufficient_scope")
	}
	ok, err := c.Can(ctx, p, scope, perm)
	if err != nil {
		return err
	}
	if !ok {
		return &apperr.Error{Status: 403, Code: "forbidden", Detail: "missing permission " + string(perm)}
	}
	return nil
}
