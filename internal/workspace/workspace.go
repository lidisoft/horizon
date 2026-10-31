// Package workspace owns workspaces, their members, invitations and teams
// (docs/domain.md#workspace). Users are instance-wide (internal/identity);
// a membership ties one to a workspace, active or suspended.
package workspace

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/workspace/db"
)

// Workspace is the public view of a workspace.
type Workspace struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug" example:"acme"`
	Name      string    `json:"name" example:"Acme"`
	TimeZone  string    `json:"time_zone" example:"Europe/Paris"`
	CreatedAt time.Time `json:"created_at"`
	Version   int32     `json:"version"`
}

func workspaceFrom(w db.Workspace) Workspace {
	return Workspace{ID: w.ID, Slug: w.Slug, Name: w.Name, TimeZone: w.TimeZone, CreatedAt: w.CreatedAt, Version: w.Version}
}

// Options configure a Service.
type Options struct {
	Pool     *pgxpool.Pool
	Log      zerolog.Logger
	Identity *identity.Service
	Access   access.Checker
	// Mail sends invitations; nil makes inviting fail.
	Mail    mail.Sender
	BaseURL *url.URL
	Now     func() time.Time
}

// Service implements the workspace commands and queries.
type Service struct {
	pool     *pgxpool.Pool
	log      zerolog.Logger
	identity *identity.Service
	access   access.Checker
	mail     mail.Sender
	baseURL  *url.URL
	now      func() time.Time
}

// New returns a Service.
func New(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		pool: o.Pool, log: o.Log, identity: o.Identity, access: o.Access, mail: o.Mail, baseURL: o.BaseURL,
		now: func() time.Time { return now().UTC().Truncate(time.Microsecond) },
	}
}

// slugRe: lower-case letters, digits and single hyphens, starting with a
// letter, 2 to 40 characters (it appears in every URL: /w/<slug>/…).
var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,39}$`)

func validSlug(s string) bool {
	return slugRe.MatchString(s) && !strings.Contains(s, "--") && !strings.HasSuffix(s, "-")
}

func validTimeZone(tz string) bool {
	_, err := time.LoadLocation(tz)
	return err == nil && tz != "" && tz != "Local"
}

// NewWorkspace is the input of Create.
type NewWorkspace struct {
	Slug     string
	Name     string
	TimeZone string    // default UTC
	OwnerID  uuid.UUID // joins as the first member
}

// Create makes a workspace with its owner as first member. Only instance
// admins create workspaces (docs/security.md#accounts); the caller checks.
func (s *Service) Create(ctx context.Context, actor uuid.NullUUID, in NewWorkspace) (Workspace, error) {
	slug := strings.ToLower(strings.TrimSpace(in.Slug))
	name := strings.TrimSpace(in.Name)
	tz := strings.TrimSpace(in.TimeZone)
	if tz == "" {
		tz = "UTC"
	}
	var fields apperr.Fields
	if !validSlug(slug) {
		fields.Add("slug", "invalid")
	}
	switch {
	case name == "":
		fields.Add("name", "required")
	case len([]rune(name)) > 100:
		fields.Add("name", "too_long")
	}
	if !validTimeZone(tz) {
		fields.Add("time_zone", "unknown")
	}
	if err := fields.Err(); err != nil {
		return Workspace{}, err
	}

	var created db.Workspace
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.now()
		var err error
		created, err = q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
			ID: uuid.Must(uuid.NewV7()), Slug: slug, Name: name, TimeZone: tz, CreatedAt: now,
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "slug", Code: "taken"})
		}
		if err != nil {
			return err
		}
		ev := s.event(created.ID, actor, "workspace", created.ID, int64(created.Version), "WorkspaceCreated",
			map[string]any{"slug": slug, "owner_id": in.OwnerID})
		if err := eventlog.Append(ctx, tx, ev); err != nil {
			return err
		}
		_, err = s.join(ctx, tx, created.ID, in.OwnerID, actor, "creation")
		return err
	})
	if err != nil {
		return Workspace{}, err
	}
	return workspaceFrom(created), nil
}

// ForUser lists the workspaces where the user is an active member.
func (s *Service) ForUser(ctx context.Context, userID uuid.UUID) ([]Workspace, error) {
	rows, err := db.New(s.pool).UserWorkspaces(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, len(rows))
	for i, r := range rows {
		out[i] = workspaceFrom(r)
	}
	return out, nil
}

// All lists every workspace of the instance, for instance admins.
func (s *Service) All(ctx context.Context) ([]Workspace, error) {
	rows, err := db.New(s.pool).AllWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Workspace, len(rows))
	for i, r := range rows {
		out[i] = workspaceFrom(r)
	}
	return out, nil
}

// Context is a workspace as seen by the caller of a request.
type Context struct {
	Workspace Workspace
	Principal identity.Principal
	// Member is false for an instance admin who isn't a member.
	Member bool
}

// Scope is the workspace's access scope.
func (wc Context) Scope() access.Scope { return access.Scope{WorkspaceID: wc.Workspace.ID} }

// Enter resolves a workspace for p: active members and instance admins
// enter; anyone else gets workspace_not_found, so slugs don't leak.
func (s *Service) Enter(ctx context.Context, p identity.Principal, slug string) (Context, error) {
	q := db.New(s.pool)
	w, err := q.WorkspaceBySlug(ctx, strings.ToLower(slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return Context{}, apperr.NotFound("workspace_not_found")
	}
	if err != nil {
		return Context{}, err
	}
	m, err := q.Membership(ctx, db.MembershipParams{WorkspaceID: w.ID, UserID: p.User.ID})
	member := err == nil && m.Status == statusActive
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Context{}, err
	}
	if !member && !p.User.IsInstanceAdmin {
		return Context{}, apperr.NotFound("workspace_not_found")
	}
	return Context{Workspace: workspaceFrom(w), Principal: p, Member: member}, nil
}

// Require checks a permission in the workspace.
func (s *Service) Require(ctx context.Context, wc Context, perm access.Permission) error {
	return access.Require(ctx, s.access, wc.Principal, wc.Scope(), perm)
}

// Update changes the fields that are set.
type Update struct {
	Name     *string
	Slug     *string
	TimeZone *string
}

// UpdateWorkspace edits the workspace (workspace.manage). version is the
// If-Match value, 0 to skip the check.
func (s *Service) UpdateWorkspace(ctx context.Context, wc Context, version int32, in Update) (Workspace, error) {
	if err := s.Require(ctx, wc, access.WorkspaceManage); err != nil {
		return Workspace{}, err
	}
	var updated db.Workspace
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		w, err := q.LockWorkspace(ctx, wc.Workspace.ID)
		if err != nil {
			return err
		}
		if version != 0 && w.Version != version {
			return apperr.Conflict("version_conflict")
		}
		p := db.UpdateWorkspaceParams{ID: w.ID, Name: w.Name, Slug: w.Slug, TimeZone: w.TimeZone, UpdatedAt: s.now()}
		var fields apperr.Fields
		var changed []string
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			switch {
			case name == "":
				fields.Add("name", "required")
			case len([]rune(name)) > 100:
				fields.Add("name", "too_long")
			case name != w.Name:
				p.Name = name
				changed = append(changed, "name")
			}
		}
		if in.Slug != nil {
			slug := strings.ToLower(strings.TrimSpace(*in.Slug))
			switch {
			case !validSlug(slug):
				fields.Add("slug", "invalid")
			case slug != w.Slug:
				p.Slug = slug
				changed = append(changed, "slug")
			}
		}
		if in.TimeZone != nil {
			tz := strings.TrimSpace(*in.TimeZone)
			switch {
			case !validTimeZone(tz):
				fields.Add("time_zone", "unknown")
			case tz != w.TimeZone:
				p.TimeZone = tz
				changed = append(changed, "time_zone")
			}
		}
		if err := fields.Err(); err != nil {
			return err
		}
		if len(changed) == 0 {
			updated = w
			return nil
		}
		updated, err = q.UpdateWorkspace(ctx, p)
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "slug", Code: "taken"})
		}
		if err != nil {
			return err
		}
		payload := map[string]any{"fields": changed}
		if p.Slug != w.Slug {
			payload["slug"] = p.Slug
		}
		return eventlog.Append(ctx, tx, s.event(w.ID, actorOf(wc.Principal), "workspace", w.ID, int64(updated.Version), "WorkspaceUpdated", payload))
	})
	if err != nil {
		return Workspace{}, err
	}
	return workspaceFrom(updated), nil
}

// --- Helpers ----------------------------------------------------------------

func (s *Service) event(workspaceID uuid.UUID, actor uuid.NullUUID, aggregate string, id uuid.UUID, version int64, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return eventlog.Event{
		WorkspaceID:      uuid.NullUUID{UUID: workspaceID, Valid: true},
		AggregateType:    aggregate,
		AggregateID:      id,
		AggregateVersion: version,
		Type:             typ,
		ActorID:          actor,
		OccurredAt:       s.now(),
		Payload:          eventlog.Payload(payload),
	}
}

func actorOf(p identity.Principal) uuid.NullUUID {
	return uuid.NullUUID{UUID: p.User.ID, Valid: p.User.ID != uuid.Nil}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// workspacePermissions are reported by Capabilities for the UI.
var workspacePermissions = []access.Permission{
	access.WorkspaceManage, access.WorkspaceMembersManage, access.WorkspaceRolesManage,
	access.WorkspaceConfigManage, access.WorkspaceWebhooksManage, access.WorkspaceIntegrationsManage,
	access.ProjectCreate,
}

// Capabilities lists the workspace-level permissions the caller holds, so
// the UI can hide what they can't do. The server checks again on each
// request.
func (s *Service) Capabilities(ctx context.Context, wc Context) ([]string, error) {
	out := []string{}
	for _, perm := range workspacePermissions {
		err := s.Require(ctx, wc, perm)
		if err == nil {
			out = append(out, string(perm))
		} else if _, ok := apperr.As(err); !ok {
			return nil, err
		}
	}
	return out, nil
}
