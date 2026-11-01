package planning

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/planning/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Milestone is the public view of a milestone: a dated point of a project,
// or of the whole workspace when it has none (docs/domain.md#milestone).
type Milestone struct {
	ID          uuid.UUID     `json:"id"`
	ProjectID   uuid.NullUUID `json:"project_id" swaggertype:"string"`
	Name        string        `json:"name" example:"Beta"`
	Description string        `json:"description"`
	Date        string        `json:"date" example:"2026-12-18"`
	// Progress counts its linked issues the caller can see.
	Progress *Stats `json:"progress,omitempty"`
	// CanManage is true when the caller holds milestone.manage on its scope.
	CanManage bool      `json:"can_manage"`
	CreatedAt time.Time `json:"created_at"`
	Version   int32     `json:"version"`
}

func milestoneFrom(m db.Milestone) Milestone {
	return Milestone{
		ID: m.ID, ProjectID: m.ProjectID, Name: m.Name, Description: m.Description, Date: m.Date.Time.Format(time.DateOnly),
		CreatedAt: m.CreatedAt, Version: m.Version,
	}
}

// MilestoneList lists milestones by date.
type MilestoneList struct {
	Items []Milestone `json:"items"`
	// CanCreate is true when the caller may add workspace milestones.
	CanCreate bool `json:"can_create"`
}

// viewer is what the caller sees of the workspace, for milestones.
type viewer struct {
	wc        workspace.Context
	projects  map[uuid.UUID]project.Project // with issue.view
	workspace bool                          // issue.view on the workspace: its milestones
}

func (s *Service) viewer(ctx context.Context, wc workspace.Context) (viewer, error) {
	v := viewer{wc: wc, projects: map[uuid.UUID]project.Project{}}
	all, err := s.projects.List(ctx, wc)
	if err != nil {
		return v, err
	}
	for _, p := range all {
		ok, err := s.access.Can(ctx, wc.Principal, access.Scope{WorkspaceID: wc.Workspace.ID, ProjectID: uuid.NullUUID{UUID: p.ID, Valid: true}}, access.IssueView)
		if err != nil {
			return v, err
		}
		if ok {
			v.projects[p.ID] = p
		}
	}
	v.workspace, err = s.access.Can(ctx, wc.Principal, wc.Scope(), access.IssueView)
	return v, err
}

func (v viewer) sees(m db.Milestone) bool {
	if m.ProjectID.Valid {
		_, ok := v.projects[m.ProjectID.UUID]
		return ok
	}
	return v.workspace
}

func (v viewer) projectIDs() []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(v.projects))
	for id := range v.projects {
		ids = append(ids, id)
	}
	return ids
}

// scope is where a milestone's permissions are checked.
func scopeOf(wc workspace.Context, projectID uuid.NullUUID) access.Scope {
	return access.Scope{WorkspaceID: wc.Workspace.ID, ProjectID: projectID}
}

// Milestones lists the milestones the caller sees: the workspace's and
// those of visible projects; with projectKey, the workspace's and that
// project's.
func (s *Service) Milestones(ctx context.Context, wc workspace.Context, projectKey string) (MilestoneList, error) {
	v, err := s.viewer(ctx, wc)
	if err != nil {
		return MilestoneList{}, err
	}
	ids := v.projectIDs()
	if projectKey != "" {
		pc, err := s.projects.Enter(ctx, wc, projectKey)
		if err != nil {
			return MilestoneList{}, err
		}
		ids = []uuid.UUID{}
		if _, ok := v.projects[pc.Project.ID]; ok {
			ids = []uuid.UUID{pc.Project.ID}
		}
	}
	rows, err := db.New(s.pool).Milestones(ctx, db.MilestonesParams{WorkspaceID: wc.Workspace.ID, ProjectIds: ids})
	if err != nil {
		return MilestoneList{}, err
	}
	out := MilestoneList{Items: []Milestone{}}
	out.CanCreate, err = s.access.Can(ctx, wc.Principal, wc.Scope(), access.MilestoneManage)
	if err != nil {
		return MilestoneList{}, err
	}
	var visible []db.Milestone
	for _, m := range rows {
		if v.sees(m) {
			visible = append(visible, m)
		}
	}
	items, err := s.describeMilestones(ctx, v, visible)
	if err != nil {
		return MilestoneList{}, err
	}
	out.Items = append(out.Items, items...)
	return out, nil
}

// describeMilestones adds progress and permissions.
func (s *Service) describeMilestones(ctx context.Context, v viewer, rows []db.Milestone) ([]Milestone, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, m := range rows {
		ids[i] = m.ID
	}
	stats, err := db.New(s.pool).MilestoneStats(ctx, db.MilestoneStatsParams{WorkspaceID: v.wc.Workspace.ID, MilestoneIds: ids, ProjectIds: v.projectIDs()})
	if err != nil {
		return nil, err
	}
	scales := map[uuid.UUID]configdef.Estimation{}
	progress := map[uuid.UUID]*Stats{}
	for _, r := range stats {
		scale, ok := scales[r.ProjectID]
		if !ok {
			res, err := s.config.Resolve(ctx, s.pool, v.wc.Workspace.ID, uuid.NullUUID{UUID: r.ProjectID, Valid: true})
			if err != nil {
				return nil, err
			}
			scale = res.Estimation
			scales[r.ProjectID] = scale
		}
		if progress[r.MilestoneID] == nil {
			progress[r.MilestoneID] = newStats()
		}
		progress[r.MilestoneID].add(scale, r.Estimate, r.Outcome, int(r.Issues))
	}
	out := make([]Milestone, len(rows))
	for i, m := range rows {
		out[i] = milestoneFrom(m)
		out[i].Progress = progress[m.ID]
		if out[i].Progress == nil {
			out[i].Progress = newStats()
		}
		can, err := s.access.Can(ctx, v.wc.Principal, scopeOf(v.wc, m.ProjectID), access.MilestoneManage)
		if err != nil {
			return nil, err
		}
		out[i].CanManage = can
	}
	return out, nil
}

// Milestone returns one milestone the caller sees.
func (s *Service) Milestone(ctx context.Context, wc workspace.Context, id uuid.UUID) (Milestone, error) {
	v, err := s.viewer(ctx, wc)
	if err != nil {
		return Milestone{}, err
	}
	m, err := db.New(s.pool).Milestone(ctx, db.MilestoneParams{WorkspaceID: wc.Workspace.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !v.sees(m)) {
		return Milestone{}, apperr.NotFound("milestone_not_found")
	}
	if err != nil {
		return Milestone{}, err
	}
	out, err := s.describeMilestones(ctx, v, []db.Milestone{m})
	if err != nil {
		return Milestone{}, err
	}
	return out[0], nil
}

// MilestoneIssues lists the keys of a milestone's linked issues the caller
// sees.
func (s *Service) MilestoneIssues(ctx context.Context, wc workspace.Context, id uuid.UUID) ([]string, error) {
	v, err := s.viewer(ctx, wc)
	if err != nil {
		return nil, err
	}
	m, err := db.New(s.pool).Milestone(ctx, db.MilestoneParams{WorkspaceID: wc.Workspace.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !v.sees(m)) {
		return nil, apperr.NotFound("milestone_not_found")
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.key || '-' || i.number FROM milestone_issues mi
		JOIN issues i ON i.id = mi.issue_id JOIN projects p ON p.id = i.project_id
		WHERE mi.milestone_id = $1 AND i.project_id = ANY($2::uuid[])
		ORDER BY p.key, i.number`, id, v.projectIDs())
	if err != nil {
		return nil, err
	}
	keys, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if keys == nil {
		keys = []string{}
	}
	return keys, err
}

// MilestoneInput creates a milestone or, with nil fields left alone,
// changes one. Project (a key) applies on creation only: none makes a
// workspace milestone.
type MilestoneInput struct {
	Project     string
	Name        *string
	Description *string
	Date        *string
}

// milestoneTarget resolves where a milestone lives and checks
// milestone.manage there (and that its project isn't archived).
func (s *Service) milestoneTarget(ctx context.Context, wc workspace.Context, projectID uuid.NullUUID, projectKey string) (uuid.NullUUID, error) {
	if projectKey != "" || projectID.Valid {
		var pc project.Context
		var err error
		if projectID.Valid {
			pc, err = s.projects.EnterByID(ctx, wc, projectID.UUID)
		} else {
			pc, err = s.projects.Enter(ctx, wc, projectKey)
		}
		if err != nil {
			if projectID.Valid {
				return projectID, apperr.NotFound("milestone_not_found")
			}
			return projectID, apperr.Invalid(apperr.Field{Field: "project", Code: "unknown"})
		}
		if err := s.requireManage(ctx, pc, access.MilestoneManage); err != nil {
			return projectID, err
		}
		return uuid.NullUUID{UUID: pc.Project.ID, Valid: true}, nil
	}
	return projectID, access.Require(ctx, s.access, wc.Principal, wc.Scope(), access.MilestoneManage)
}

func (s *Service) applyMilestone(fields *apperr.Fields, m *db.Milestone, in MilestoneInput) []string {
	var changed []string
	if in.Name != nil {
		if n := validName(fields, *in.Name); n != m.Name {
			m.Name = n
			changed = append(changed, "name")
		}
	}
	if in.Description != nil {
		if d := validDescription(fields, *in.Description); d != m.Description {
			m.Description = d
			changed = append(changed, "description")
		}
	}
	if in.Date != nil {
		d, ok := parseDate(*in.Date)
		switch {
		case !ok:
			fields.Add("date", "invalid")
		case !d.Valid:
			fields.Add("date", "required")
		case d != m.Date:
			m.Date = d
			changed = append(changed, "date")
		}
	}
	return changed
}

// CreateMilestone adds a milestone (milestone.manage on its project, or on
// the workspace for a workspace milestone).
func (s *Service) CreateMilestone(ctx context.Context, wc workspace.Context, in MilestoneInput) (Milestone, error) {
	projectID, err := s.milestoneTarget(ctx, wc, uuid.NullUUID{}, strings.TrimSpace(in.Project))
	if err != nil {
		return Milestone{}, err
	}
	if in.Name == nil {
		in.Name = new("")
	}
	if in.Date == nil {
		in.Date = new("")
	}
	m := db.Milestone{WorkspaceID: wc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), ProjectID: projectID}
	var fields apperr.Fields
	s.applyMilestone(&fields, &m, in)
	if err := fields.Err(); err != nil {
		return Milestone{}, err
	}
	err = postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		created, err := db.New(tx).CreateMilestone(ctx, db.CreateMilestoneParams{
			WorkspaceID: m.WorkspaceID, ID: m.ID, ProjectID: m.ProjectID, Name: m.Name, Description: m.Description, Date: m.Date, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(wc, "milestone", created.ID, created.ProjectID.UUID, created.Version, "MilestoneCreated",
			map[string]any{"name": created.Name, "date": dateString(created.Date)}))
	})
	if err != nil {
		return Milestone{}, err
	}
	return s.Milestone(ctx, wc, m.ID)
}

// mutateMilestone runs one command on a milestone the caller manages.
func (s *Service) mutateMilestone(ctx context.Context, wc workspace.Context, id uuid.UUID, version int32,
	fn func(tx pgx.Tx, cur db.Milestone, next *db.Milestone) ([]change, error)) (Milestone, error) {
	var saved db.Milestone
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockMilestone(ctx, db.LockMilestoneParams{WorkspaceID: wc.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("milestone_not_found")
		}
		if err != nil {
			return err
		}
		if _, err := s.milestoneTarget(ctx, wc, cur.ProjectID, ""); err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		next := cur
		changes, err := fn(tx, cur, &next)
		if err != nil || len(changes) == 0 {
			saved = cur
			return err
		}
		saved, err = q.SaveMilestone(ctx, db.SaveMilestoneParams{
			WorkspaceID: next.WorkspaceID, ID: next.ID, Name: next.Name, Description: next.Description, Date: next.Date, UpdatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		events := make([]eventlog.Event, len(changes))
		for i, c := range changes {
			events[i] = s.event(wc, "milestone", saved.ID, saved.ProjectID.UUID, saved.Version, c.typ, c.payload)
		}
		return eventlog.Append(ctx, tx, events...)
	})
	if err != nil {
		return Milestone{}, err
	}
	return s.Milestone(ctx, wc, saved.ID)
}

// UpdateMilestone renames, describes or moves a milestone (MilestoneUpdated).
func (s *Service) UpdateMilestone(ctx context.Context, wc workspace.Context, id uuid.UUID, version int32, in MilestoneInput) (Milestone, error) {
	return s.mutateMilestone(ctx, wc, id, version, func(_ pgx.Tx, _ db.Milestone, next *db.Milestone) ([]change, error) {
		var fields apperr.Fields
		changed := s.applyMilestone(&fields, next, in)
		if err := fields.Err(); err != nil {
			return nil, err
		}
		if len(changed) == 0 {
			return nil, nil
		}
		return []change{{"MilestoneUpdated", map[string]any{"fields": changed}}}, nil
	})
}

// LinkInput lists issues (keys) to link to a milestone and to unlink.
type LinkInput struct {
	Add    []string
	Remove []string
}

// LinkMilestone links issues to a milestone and unlinks others
// (MilestoneUpdated with the issues added and removed). A project
// milestone takes its project's issues only; a workspace milestone any the
// caller sees.
func (s *Service) LinkMilestone(ctx context.Context, wc workspace.Context, id uuid.UUID, in LinkInput) (Milestone, error) {
	if len(in.Add)+len(in.Remove) > maxLoad {
		return Milestone{}, apperr.Invalid(apperr.Field{Field: "add", Code: "too_many"})
	}
	return s.mutateMilestone(ctx, wc, id, 0, func(tx pgx.Tx, cur db.Milestone, _ *db.Milestone) ([]change, error) {
		var fields apperr.Fields
		resolve := func(field string, keys []string) []uuid.UUID {
			var ids []uuid.UUID
			for _, k := range keys {
				ic, err := s.issues.Enter(ctx, wc, k)
				if err != nil || (cur.ProjectID.Valid && ic.Issue.ProjectID != cur.ProjectID.UUID) {
					fields.Add(field, "unknown")
					continue
				}
				if !slices.Contains(ids, ic.Issue.ID) {
					ids = append(ids, ic.Issue.ID)
				}
			}
			return ids
		}
		add, remove := resolve("add", in.Add), resolve("remove", in.Remove)
		if err := fields.Err(); err != nil {
			return nil, err
		}
		q := db.New(tx)
		added, removed := []uuid.UUID{}, []uuid.UUID{}
		for _, i := range add {
			n, err := q.LinkMilestoneIssue(ctx, db.LinkMilestoneIssueParams{WorkspaceID: cur.WorkspaceID, MilestoneID: cur.ID, IssueID: i})
			if err != nil {
				return nil, err
			}
			if n > 0 {
				added = append(added, i)
			}
		}
		for _, i := range remove {
			if slices.Contains(add, i) {
				continue
			}
			n, err := q.UnlinkMilestoneIssue(ctx, db.UnlinkMilestoneIssueParams{MilestoneID: cur.ID, IssueID: i})
			if err != nil {
				return nil, err
			}
			if n > 0 {
				removed = append(removed, i)
			}
		}
		if len(added) == 0 && len(removed) == 0 {
			return nil, nil
		}
		return []change{{"MilestoneUpdated", map[string]any{"added": added, "removed": removed}}}, nil
	})
}

// DeleteMilestone hard-deletes a milestone: its links go, its events are
// scrubbed, a MilestoneDeleted tombstone remains.
func (s *Service) DeleteMilestone(ctx context.Context, wc workspace.Context, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockMilestone(ctx, db.LockMilestoneParams{WorkspaceID: wc.Workspace.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("milestone_not_found")
		}
		if err != nil {
			return err
		}
		if _, err := s.milestoneTarget(ctx, wc, cur.ProjectID, ""); err != nil {
			return err
		}
		if err := q.DeleteMilestone(ctx, db.DeleteMilestoneParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID}); err != nil {
			return err
		}
		return eventlog.Tombstone(ctx, tx, s.event(wc, "milestone", cur.ID, cur.ProjectID.UUID, cur.Version+1, "MilestoneDeleted", nil))
	})
}
