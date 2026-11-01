// Package planning owns what a project plans its issues into: iterations
// (docs/domain.md#iteration). Issues hold their iteration
// (issues.iteration_id); this package moves them in and out through the
// issue module and records each iteration's side of it.
package planning

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/planning/db"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Service implements the planning commands and queries.
type Service struct {
	pool     *pgxpool.Pool
	access   *access.Service
	config   *configdef.Service
	projects *project.Service
	issues   *issue.Service
	now      func() time.Time
}

// New returns a Service and plugs it into the issue and project modules:
// issues leaving an iteration on their own (deleted, moved) are recorded,
// and a deleted project's iterations go with it.
func New(pool *pgxpool.Pool, acc *access.Service, config *configdef.Service, projects *project.Service, issues *issue.Service,
	now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	s := &Service{pool: pool, access: acc, config: config, projects: projects, issues: issues,
		now: func() time.Time { return now().UTC().Truncate(time.Microsecond) }}
	issues.OnLeaveIteration(s.issuesLeft)
	projects.OnDelete(s.deleteProject)
	return s
}

func (s *Service) require(ctx context.Context, pc project.Context, perm access.Permission) error {
	return access.Require(ctx, s.access, pc.Principal, pc.Scope(), perm)
}

// can reports whether the caller holds perm on the project, for the UI.
func (s *Service) can(ctx context.Context, pc project.Context, perm access.Permission) bool {
	return s.require(ctx, pc, perm) == nil
}

func (s *Service) settings(ctx context.Context, conn db.DBTX, pc project.Context) (configdef.IterationSettings, error) {
	return s.config.IterationSettings(ctx, conn, configdef.Target{
		Principal: pc.Principal, WorkspaceID: pc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: pc.Project.ID, Valid: true},
	})
}

// event builds a planning event; every payload names the project, so
// pushes reach its viewers (docs/realtime.md#topics).
func (s *Service) event(wc workspace.Context, aggregate string, id, projectID uuid.UUID, version int32, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["project_id"] = projectID
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: wc.Workspace.ID, Valid: true}, AggregateType: aggregate, AggregateID: id,
		AggregateVersion: int64(version), Type: typ,
		ActorID:    uuid.NullUUID{UUID: wc.Principal.User.ID, Valid: wc.Principal.User.ID != uuid.Nil},
		OccurredAt: s.now(), Payload: eventlog.Payload(payload),
	}
}

// deleteProject removes a deleted project's iterations and scrubs their
// history (a project.DeleteHook; ProjectDeleted is their tombstone).
func (s *Service) deleteProject(ctx context.Context, tx pgx.Tx, pc project.Context) error {
	rows, err := tx.Query(ctx, `DELETE FROM iterations WHERE workspace_id = $1 AND project_id = $2 RETURNING id`, pc.Workspace.ID, pc.Project.ID)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return err
	}
	return eventlog.Scrub(ctx, tx, uuid.NullUUID{UUID: pc.Workspace.ID, Valid: true}, "iteration", ids)
}
