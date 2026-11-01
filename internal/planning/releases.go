package planning

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/planning/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
)

// Release states (docs/domain.md#release).
const (
	ReleaseUnreleased = "unreleased"
	ReleaseReleased   = "released"
	ReleaseArchived   = "archived"
)

const maxDescription = 2000

// Release is the public view of a release.
type Release struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name" example:"v0.1"`
	Description string     `json:"description"`
	State       string     `json:"state" enums:"unreleased,released,archived"`
	StartDate   *string    `json:"start_date" example:"2026-11-02"`
	ReleaseDate *string    `json:"release_date" example:"2026-12-18"`
	ReleasedAt  *time.Time `json:"released_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
	// Progress counts the issues targeted to it (lists and reads).
	Progress  *Stats    `json:"progress,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Version   int32     `json:"version"`
}

func releaseFrom(r db.Release) Release {
	return Release{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Description: r.Description, State: r.State,
		StartDate: dateString(r.StartDate), ReleaseDate: dateString(r.ReleaseDate), ReleasedAt: r.ReleasedAt,
		ArchivedAt: r.ArchivedAt, CreatedAt: r.CreatedAt, Version: r.Version,
	}
}

// ReleaseList is a project's releases.
type ReleaseList struct {
	Items []Release `json:"items"`
	// CanManage is true when the caller holds release.manage.
	CanManage bool `json:"can_manage"`
}

// Releases lists the project's releases (archived ones with archived),
// with their progress.
func (s *Service) Releases(ctx context.Context, pc project.Context, archived bool) (ReleaseList, error) {
	if err := s.require(ctx, pc, access.IssueView); err != nil {
		return ReleaseList{}, err
	}
	rows, err := db.New(s.pool).Releases(ctx, db.ReleasesParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, Archived: archived})
	if err != nil {
		return ReleaseList{}, err
	}
	progress, err := s.releaseStats(ctx, pc)
	if err != nil {
		return ReleaseList{}, err
	}
	out := ReleaseList{Items: []Release{}, CanManage: s.can(ctx, pc, access.ReleaseManage)}
	for _, r := range rows {
		rel := releaseFrom(r)
		rel.Progress = progress[r.ID]
		if rel.Progress == nil {
			rel.Progress = newStats()
		}
		out.Items = append(out.Items, rel)
	}
	return out, nil
}

// Release returns one release with its progress.
func (s *Service) Release(ctx context.Context, pc project.Context, id uuid.UUID) (Release, error) {
	if err := s.require(ctx, pc, access.IssueView); err != nil {
		return Release{}, err
	}
	r, err := db.New(s.pool).Release(ctx, db.ReleaseParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return Release{}, apperr.NotFound("release_not_found")
	}
	if err != nil {
		return Release{}, err
	}
	progress, err := s.releaseStats(ctx, pc)
	if err != nil {
		return Release{}, err
	}
	rel := releaseFrom(r)
	rel.Progress = progress[id]
	if rel.Progress == nil {
		rel.Progress = newStats()
	}
	return rel, nil
}

func (s *Service) releaseStats(ctx context.Context, pc project.Context) (map[uuid.UUID]*Stats, error) {
	r, err := s.config.Resolve(ctx, s.pool, pc.Workspace.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ReleaseStats(ctx, db.ReleaseStatsParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]*Stats{}
	for _, row := range rows {
		if out[row.ReleaseID] == nil {
			out[row.ReleaseID] = newStats()
		}
		out[row.ReleaseID].add(r.Estimation, row.Estimate, row.Outcome, int(row.Issues))
	}
	return out, nil
}

// ReleaseInput creates a release or, with nil fields left alone, changes
// one. Dates are optional and independent.
type ReleaseInput struct {
	Name        *string
	Description *string
	StartDate   *string
	ReleaseDate *string
}

func validDescription(fields *apperr.Fields, d string) string {
	d = strings.TrimSpace(d)
	if len([]rune(d)) > maxDescription {
		fields.Add("description", "too_long")
	}
	return d
}

// apply validates in onto r, returning the changed fields.
func (s *Service) applyRelease(ctx context.Context, q *db.Queries, fields *apperr.Fields, r *db.Release, in ReleaseInput) ([]string, error) {
	var changed []string
	if in.Name != nil {
		if n := validName(fields, *in.Name); n != r.Name {
			taken, err := q.ReleaseNameTaken(ctx, db.ReleaseNameTakenParams{ProjectID: r.ProjectID, Name: n, OtherThan: r.ID})
			if err != nil {
				return nil, err
			}
			if taken {
				fields.Add("name", "taken")
			}
			r.Name = n
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		if d := validDescription(fields, *in.Description); d != r.Description {
			r.Description = d
			changed = append(changed, "description")
		}
	}
	for _, d := range []struct {
		field string
		in    *string
		date  *pgtype.Date
	}{{"start_date", in.StartDate, &r.StartDate}, {"release_date", in.ReleaseDate, &r.ReleaseDate}} {
		if d.in == nil {
			continue
		}
		v, ok := parseDate(*d.in)
		if !ok {
			fields.Add(d.field, "invalid")
			continue
		}
		if v != *d.date {
			*d.date = v
			changed = append(changed, d.field)
		}
	}
	if r.StartDate.Valid && r.ReleaseDate.Valid && r.ReleaseDate.Time.Before(r.StartDate.Time) {
		fields.Add("release_date", "before_start")
	}
	return changed, nil
}

// CreateRelease adds a release to the project (release.manage).
func (s *Service) CreateRelease(ctx context.Context, pc project.Context, in ReleaseInput) (Release, error) {
	if err := s.requireManage(ctx, pc, access.ReleaseManage); err != nil {
		return Release{}, err
	}
	if in.Name == nil {
		in.Name = new("")
	}
	var created db.Release
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		r := db.Release{WorkspaceID: pc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), ProjectID: pc.Project.ID}
		var fields apperr.Fields
		if _, err := s.applyRelease(ctx, q, &fields, &r, in); err != nil {
			return err
		}
		if err := fields.Err(); err != nil {
			return err
		}
		var err error
		created, err = q.CreateRelease(ctx, db.CreateReleaseParams{
			WorkspaceID: r.WorkspaceID, ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, Description: r.Description,
			StartDate: r.StartDate, ReleaseDate: r.ReleaseDate, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(pc.Context, "release", created.ID, created.ProjectID, created.Version, "ReleaseCreated",
			map[string]any{"name": created.Name, "release_date": dateString(created.ReleaseDate)}))
	})
	if err != nil {
		return Release{}, err
	}
	rel := releaseFrom(created)
	rel.Progress = newStats()
	return rel, nil
}

// requireManage checks perm on the project and that it isn't archived.
func (s *Service) requireManage(ctx context.Context, pc project.Context, perm access.Permission) error {
	if err := s.require(ctx, pc, perm); err != nil {
		return err
	}
	if pc.Project.ArchivedAt != nil {
		return apperr.Conflict("project_archived")
	}
	return nil
}

// mutateRelease runs one command on a release, as mutate does for
// iterations.
func (s *Service) mutateRelease(ctx context.Context, pc project.Context, id uuid.UUID, version int32,
	fn func(q *db.Queries, cur db.Release, next *db.Release) ([]change, error)) (Release, error) {
	if err := s.requireManage(ctx, pc, access.ReleaseManage); err != nil {
		return Release{}, err
	}
	var saved db.Release
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockRelease(ctx, db.LockReleaseParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("release_not_found")
		}
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		next := cur
		changes, err := fn(q, cur, &next)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			saved = cur
			return nil
		}
		saved, err = q.SaveRelease(ctx, db.SaveReleaseParams{
			WorkspaceID: next.WorkspaceID, ID: next.ID, Name: next.Name, Description: next.Description, State: next.State,
			StartDate: next.StartDate, ReleaseDate: next.ReleaseDate, ReleasedAt: next.ReleasedAt, ArchivedAt: next.ArchivedAt,
			UpdatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		events := make([]eventlog.Event, len(changes))
		for i, c := range changes {
			events[i] = s.event(pc.Context, "release", saved.ID, saved.ProjectID, saved.Version, c.typ, c.payload)
		}
		return eventlog.Append(ctx, tx, events...)
	})
	if err != nil {
		return Release{}, err
	}
	return s.Release(ctx, pc, saved.ID)
}

// UpdateRelease changes a release that isn't archived (ReleaseUpdated).
func (s *Service) UpdateRelease(ctx context.Context, pc project.Context, id uuid.UUID, version int32, in ReleaseInput) (Release, error) {
	return s.mutateRelease(ctx, pc, id, version, func(q *db.Queries, cur db.Release, next *db.Release) ([]change, error) {
		if cur.State == ReleaseArchived {
			return nil, apperr.Conflict("release_archived")
		}
		var fields apperr.Fields
		changed, err := s.applyRelease(ctx, q, &fields, next, in)
		if err != nil {
			return nil, err
		}
		if err := fields.Err(); err != nil {
			return nil, err
		}
		if len(changed) == 0 {
			return nil, nil
		}
		return []change{{"ReleaseUpdated", map[string]any{"fields": changed}}}, nil
	})
}

// ShipRelease marks an unreleased release released (ReleaseReleased); its
// release date becomes today when it had none. Issues stay targeted to it,
// finished or not.
func (s *Service) ShipRelease(ctx context.Context, pc project.Context, id uuid.UUID, version int32) (Release, error) {
	return s.mutateRelease(ctx, pc, id, version, func(_ *db.Queries, cur db.Release, next *db.Release) ([]change, error) {
		switch cur.State {
		case ReleaseReleased:
			return nil, nil
		case ReleaseArchived:
			return nil, apperr.Conflict("release_archived")
		}
		now := s.now()
		next.State, next.ReleasedAt = ReleaseReleased, &now
		if !next.ReleaseDate.Valid {
			cal, err := s.calendar(ctx, s.pool, pc)
			if err != nil {
				return nil, err
			}
			next.ReleaseDate = pgtype.Date{Time: cal.Today(now), Valid: true}
		}
		return []change{{"ReleaseReleased", map[string]any{"release_date": dateString(next.ReleaseDate)}}}, nil
	})
}

// ArchiveRelease hides a release from pickers and planning
// (ReleaseArchived); its issues keep it.
func (s *Service) ArchiveRelease(ctx context.Context, pc project.Context, id uuid.UUID, version int32) (Release, error) {
	return s.mutateRelease(ctx, pc, id, version, func(_ *db.Queries, cur db.Release, next *db.Release) ([]change, error) {
		if cur.State == ReleaseArchived {
			return nil, nil
		}
		now := s.now()
		next.State, next.ArchivedAt = ReleaseArchived, &now
		return []change{{"ReleaseArchived", nil}}, nil
	})
}

// DeleteRelease hard-deletes a release (release.manage): its issues lose
// it, its events are scrubbed, a ReleaseDeleted tombstone remains.
func (s *Service) DeleteRelease(ctx context.Context, pc project.Context, id uuid.UUID) error {
	if err := s.requireManage(ctx, pc, access.ReleaseManage); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockRelease(ctx, db.LockReleaseParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("release_not_found")
		}
		if err != nil {
			return err
		}
		if err := s.issues.ClearRelease(ctx, tx, pc.Context, cur.ID, "release_deleted"); err != nil {
			return err
		}
		if err := q.DeleteRelease(ctx, db.DeleteReleaseParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID}); err != nil {
			return err
		}
		return eventlog.Tombstone(ctx, tx, s.event(pc.Context, "release", cur.ID, cur.ProjectID, cur.Version+1, "ReleaseDeleted", nil))
	})
}
