package planning

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/planning/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Iteration states (docs/domain.md#iteration).
const (
	StatePlanned   = "planned"
	StateScheduled = "scheduled"
	StateActive    = "active"
	StateCompleted = "completed"
	StateArchived  = "archived"
)

// Limits of iteration content.
const (
	maxName = 100
	maxGoal = 1000
	// maxLoad bounds the issues one load command moves.
	maxLoad = 500
)

// open reports whether issues can still be loaded into or out of an
// iteration in this state. Completed iterations keep their scope as it was.
func open(state string) bool {
	return state == StatePlanned || state == StateScheduled || state == StateActive
}

// Iteration is the public view of an iteration.
type Iteration struct {
	ID          uuid.UUID  `json:"id"`
	ProjectID   uuid.UUID  `json:"project_id"`
	Name        string     `json:"name" example:"Sprint 4"`
	Goal        string     `json:"goal"`
	State       string     `json:"state" enums:"planned,scheduled,active,completed,archived"`
	StartDate   *string    `json:"start_date" example:"2026-11-02"`
	EndDate     *string    `json:"end_date" example:"2026-11-13"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	ArchivedAt  *time.Time `json:"archived_at"`
	// Completion is what completing it did: the policy in effect, and where
	// unfinished issues went (null: the backlog).
	Completion *Completion `json:"completion,omitempty"`
	// Stats count the issues it holds now (lists only).
	Stats     *Stats    `json:"stats,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Version   int32     `json:"version"`
}

// Completion records how an iteration was completed.
type Completion struct {
	Policy     string        `json:"policy" enums:"archive_done_rollover_rest,archive_all,none"`
	RolloverTo uuid.NullUUID `json:"rollover_to" swaggertype:"string"`
	// AppliedAt is when the policy worker applied it to the issues.
	AppliedAt *time.Time `json:"applied_at,omitempty"`
}

// Tally counts issues and their estimated points.
type Tally struct {
	Issues int     `json:"issues"`
	Points float64 `json:"points"`
}

// Stats summarise an iteration's issues.
type Stats struct {
	Tally
	// Estimated is the number of issues with an estimate.
	Estimated int `json:"estimated"`
	// ByOutcome splits them by their status's outcome (unstarted, started,
	// completed, abandoned).
	ByOutcome map[string]Tally `json:"by_outcome"`
}

func iterationFrom(it db.Iteration) Iteration {
	out := Iteration{
		ID: it.ID, ProjectID: it.ProjectID, Name: it.Name, Goal: it.Goal, State: it.State,
		StartDate: dateString(it.StartDate), EndDate: dateString(it.EndDate), StartedAt: it.StartedAt,
		CompletedAt: it.CompletedAt, ArchivedAt: it.ArchivedAt, CreatedAt: it.CreatedAt, Version: it.Version,
	}
	if len(it.Completion) > 0 {
		var c Completion
		if json.Unmarshal(it.Completion, &c) == nil {
			out.Completion = &c
		}
	}
	return out
}

func dateString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(time.DateOnly)
	return &s
}

// parseDate reads YYYY-MM-DD; "" is no date.
func parseDate(s string) (pgtype.Date, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Date{}, true
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return pgtype.Date{}, false
	}
	return pgtype.Date{Time: t, Valid: true}, true
}

// --- Reading ------------------------------------------------------------------------

// NextIteration suggests the name and dates of the next iteration: the
// working day after the latest one ends (or today), for the default length,
// ending on a weekday.
type NextIteration struct {
	Name      string `json:"name" example:"Sprint 5"`
	StartDate string `json:"start_date" example:"2026-11-16"`
	EndDate   string `json:"end_date" example:"2026-11-27"`
}

// IterationList is a project's iterations with what planning needs around
// them.
type IterationList struct {
	Items    []Iteration                 `json:"items"`
	Next     NextIteration               `json:"next"`
	Settings configdef.IterationSettings `json:"settings"`
	// Velocity is the average of completed points over the last three
	// completed iterations (0 without any).
	Velocity float64 `json:"velocity"`
	// CanManage is true when the caller holds iteration.manage.
	CanManage bool `json:"can_manage"`
}

// Iterations lists the project's iterations (archived ones with archived)
// with their stats.
func (s *Service) Iterations(ctx context.Context, pc project.Context, archived bool) (IterationList, error) {
	if err := s.require(ctx, pc, access.IssueView); err != nil {
		return IterationList{}, err
	}
	q := db.New(s.pool)
	rows, err := q.Iterations(ctx, db.IterationsParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, Archived: archived})
	if err != nil {
		return IterationList{}, err
	}
	stats, err := s.stats(ctx, pc)
	if err != nil {
		return IterationList{}, err
	}
	out := IterationList{Items: []Iteration{}, CanManage: s.can(ctx, pc, access.IterationManage)}
	var done []float64
	for _, r := range rows {
		it := iterationFrom(r)
		st := stats[r.ID]
		if st == nil {
			st = &Stats{ByOutcome: map[string]Tally{}}
		}
		it.Stats = st
		if r.State == StateCompleted || r.State == StateArchived {
			done = append(done, st.ByOutcome[configdef.OutcomeCompleted].Points)
		}
		out.Items = append(out.Items, it)
	}
	if n := min(len(done), 3); n > 0 {
		var sum float64
		for _, p := range done[:n] {
			sum += p
		}
		out.Velocity = sum / float64(n)
	}
	if out.Settings, err = s.settings(ctx, s.pool, pc); err != nil {
		return IterationList{}, err
	}
	if out.Next, err = s.next(ctx, pc, out.Settings); err != nil {
		return IterationList{}, err
	}
	return out, nil
}

// Iteration returns one iteration with its stats.
func (s *Service) Iteration(ctx context.Context, pc project.Context, id uuid.UUID) (Iteration, error) {
	if err := s.require(ctx, pc, access.IssueView); err != nil {
		return Iteration{}, err
	}
	r, err := db.New(s.pool).Iteration(ctx, db.IterationParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return Iteration{}, apperr.NotFound("iteration_not_found")
	}
	if err != nil {
		return Iteration{}, err
	}
	stats, err := s.stats(ctx, pc)
	if err != nil {
		return Iteration{}, err
	}
	it := iterationFrom(r)
	it.Stats = stats[id]
	if it.Stats == nil {
		it.Stats = &Stats{ByOutcome: map[string]Tally{}}
	}
	return it, nil
}

// stats counts the issues of each of the project's iterations, weighing
// estimates by the project's scale (a value outside it counts as its
// number, or nothing).
func (s *Service) stats(ctx context.Context, pc project.Context) (map[uuid.UUID]*Stats, error) {
	r, err := s.config.Resolve(ctx, s.pool, pc.Workspace.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).IterationStats(ctx, db.IterationStatsParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]*Stats{}
	for _, row := range rows {
		st := out[row.IterationID]
		if st == nil {
			st = &Stats{ByOutcome: map[string]Tally{}}
			out[row.IterationID] = st
		}
		n := int(row.Issues)
		points := 0.0
		if row.Estimate != nil {
			points = weight(r.Estimation, *row.Estimate) * float64(n)
			st.Estimated += n
		}
		st.Issues += n
		st.Points += points
		t := st.ByOutcome[row.Outcome]
		t.Issues += n
		t.Points += points
		st.ByOutcome[row.Outcome] = t
	}
	return out, nil
}

// weight is the points an estimate counts for.
func weight(e configdef.Estimation, label string) float64 {
	for _, v := range e.Values {
		if v.Label == label {
			return v.Weight
		}
	}
	f, err := strconv.ParseFloat(label, 64)
	if err != nil {
		return 0
	}
	return f
}

// next suggests the next iteration.
func (s *Service) next(ctx context.Context, pc project.Context, settings configdef.IterationSettings) (NextIteration, error) {
	q := db.New(s.pool)
	n, err := q.CountIterations(ctx, db.CountIterationsParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
	if err != nil {
		return NextIteration{}, err
	}
	last, err := q.LastEnd(ctx, db.LastEndParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID})
	if err != nil {
		return NextIteration{}, err
	}
	start := today(s.now(), pc.Workspace.TimeZone)
	if last.Valid && !last.Time.Before(start) {
		start = last.Time.AddDate(0, 0, 1)
	}
	start, end := span(start, settings.LengthWeeks.Value)
	return NextIteration{Name: fmt.Sprintf("Sprint %d", n+1), StartDate: start.Format(time.DateOnly), EndDate: end.Format(time.DateOnly)}, nil
}

// today is the date in a time zone (UTC when unknown), at midnight UTC.
func today(now time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// span starts on the first weekday from start and lasts weeks, ending on
// the last weekday before the same weekday weeks later. Working calendars
// (holidays) aren't modelled yet: weekends are the non-working days.
func span(start time.Time, weeks int) (time.Time, time.Time) {
	for start.Weekday() == time.Saturday || start.Weekday() == time.Sunday {
		start = start.AddDate(0, 0, 1)
	}
	end := start.AddDate(0, 0, 7*weeks-1)
	for end.Weekday() == time.Saturday || end.Weekday() == time.Sunday {
		end = end.AddDate(0, 0, -1)
	}
	return start, end
}

// --- Commands -----------------------------------------------------------------------

// PlanInput is a new iteration. Dates are optional; with both, it is
// scheduled at once.
type PlanInput struct {
	Name      string
	Goal      string
	StartDate string
	EndDate   string
}

func validName(fields *apperr.Fields, name string) string {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		fields.Add("name", "required")
	case len([]rune(name)) > maxName:
		fields.Add("name", "too_long")
	}
	return name
}

func validGoal(fields *apperr.Fields, goal string) string {
	goal = strings.TrimSpace(goal)
	if len([]rune(goal)) > maxGoal {
		fields.Add("goal", "too_long")
	}
	return goal
}

// validDates parses both dates: both set, or neither.
func validDates(fields *apperr.Fields, start, end string) (pgtype.Date, pgtype.Date) {
	s, ok1 := parseDate(start)
	e, ok2 := parseDate(end)
	if !ok1 {
		fields.Add("start_date", "invalid")
	}
	if !ok2 {
		fields.Add("end_date", "invalid")
	}
	switch {
	case !ok1 || !ok2:
	case s.Valid != e.Valid:
		if !s.Valid {
			fields.Add("start_date", "required")
		} else {
			fields.Add("end_date", "required")
		}
	case s.Valid && e.Time.Before(s.Time):
		fields.Add("end_date", "before_start")
	}
	return s, e
}

func (s *Service) checkProject(ctx context.Context, pc project.Context) error {
	if err := s.require(ctx, pc, access.IterationManage); err != nil {
		return err
	}
	if pc.Project.ArchivedAt != nil {
		return apperr.Conflict("project_archived")
	}
	return nil
}

// Plan adds an iteration to the project (iteration.manage).
func (s *Service) Plan(ctx context.Context, pc project.Context, in PlanInput) (Iteration, error) {
	if err := s.checkProject(ctx, pc); err != nil {
		return Iteration{}, err
	}
	var fields apperr.Fields
	name := validName(&fields, in.Name)
	goal := validGoal(&fields, in.Goal)
	start, end := validDates(&fields, in.StartDate, in.EndDate)
	if err := fields.Err(); err != nil {
		return Iteration{}, err
	}
	state := StatePlanned
	if start.Valid {
		state = StateScheduled
	}
	var created db.Iteration
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		created, err = db.New(tx).CreateIteration(ctx, db.CreateIterationParams{
			WorkspaceID: pc.Workspace.ID, ID: uuid.Must(uuid.NewV7()), ProjectID: pc.Project.ID, Name: name, Goal: goal,
			State: state, StartDate: start, EndDate: end, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		events := []eventlog.Event{s.event(pc.Context, "iteration", created.ID, created.ProjectID, created.Version, "IterationPlanned",
			map[string]any{"name": name})}
		if start.Valid {
			events = append(events, s.event(pc.Context, "iteration", created.ID, created.ProjectID, created.Version, "IterationScheduled",
				map[string]any{"start_date": dateString(start), "end_date": dateString(end)}))
		}
		return eventlog.Append(ctx, tx, events...)
	})
	if err != nil {
		return Iteration{}, err
	}
	it := iterationFrom(created)
	it.Stats = &Stats{ByOutcome: map[string]Tally{}}
	return it, nil
}

type change struct {
	typ     string
	payload map[string]any
}

// mutate runs one command on an iteration: permission, lock, version check
// (0 skips it), fn, then the save and the events, in one transaction.
func (s *Service) mutate(ctx context.Context, pc project.Context, id uuid.UUID, version int32,
	fn func(tx pgx.Tx, cur db.Iteration, next *db.Iteration) ([]change, error)) (Iteration, error) {
	if err := s.checkProject(ctx, pc); err != nil {
		return Iteration{}, err
	}
	var saved db.Iteration
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockIteration(ctx, db.LockIterationParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("iteration_not_found")
		}
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return apperr.Conflict("version_conflict")
		}
		next := cur
		changes, err := fn(tx, cur, &next)
		if err != nil {
			return err
		}
		if len(changes) == 0 {
			saved = cur
			return nil
		}
		saved, err = q.SaveIteration(ctx, db.SaveIterationParams{
			WorkspaceID: next.WorkspaceID, ID: next.ID, Name: next.Name, Goal: next.Goal, State: next.State,
			StartDate: next.StartDate, EndDate: next.EndDate, StartedAt: next.StartedAt, CompletedAt: next.CompletedAt,
			ArchivedAt: next.ArchivedAt, Completion: next.Completion, UpdatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		events := make([]eventlog.Event, len(changes))
		for i, c := range changes {
			events[i] = s.event(pc.Context, "iteration", saved.ID, saved.ProjectID, saved.Version, c.typ, c.payload)
		}
		return eventlog.Append(ctx, tx, events...)
	})
	if err != nil {
		return Iteration{}, err
	}
	return iterationFrom(saved), nil
}

// UpdateInput changes an iteration; nil fields are left alone. Dates go
// together: both empty unschedule a planned or scheduled iteration.
type UpdateInput struct {
	Name      *string
	Goal      *string
	StartDate *string
	EndDate   *string
}

// Update renames an iteration, changes its goal (IterationUpdated) or its
// dates (IterationScheduled). An active iteration keeps its start date;
// completed and archived ones don't change.
func (s *Service) Update(ctx context.Context, pc project.Context, id uuid.UUID, version int32, in UpdateInput) (Iteration, error) {
	return s.mutate(ctx, pc, id, version, func(_ pgx.Tx, cur db.Iteration, next *db.Iteration) ([]change, error) {
		if !open(cur.State) {
			return nil, apperr.Conflict("iteration_closed")
		}
		var fields apperr.Fields
		var changed []string
		var changes []change
		if in.Name != nil {
			if n := validName(&fields, *in.Name); n != next.Name {
				next.Name = n
				changed = append(changed, "name")
			}
		}
		if in.Goal != nil {
			if g := validGoal(&fields, *in.Goal); g != next.Goal {
				next.Goal = g
				changed = append(changed, "goal")
			}
		}
		if in.StartDate != nil || in.EndDate != nil {
			start, end := cur.StartDate, cur.EndDate
			startIn, endIn := dateString(start), dateString(end)
			if in.StartDate != nil {
				startIn = in.StartDate
			}
			if in.EndDate != nil {
				endIn = in.EndDate
			}
			start, end = validDates(&fields, deref(startIn), deref(endIn))
			switch {
			case cur.State == StateActive && !start.Valid:
				fields.Add("start_date", "required")
			case cur.State == StateActive && start != cur.StartDate:
				fields.Add("start_date", "started")
			}
			if fields.Err() == nil && (start != cur.StartDate || end != cur.EndDate) {
				next.StartDate, next.EndDate = start, end
				if cur.State != StateActive {
					next.State = StatePlanned
					if start.Valid {
						next.State = StateScheduled
					}
				}
				changes = append(changes, change{"IterationScheduled", map[string]any{"start_date": dateString(start), "end_date": dateString(end)}})
			}
		}
		if err := fields.Err(); err != nil {
			return nil, err
		}
		if len(changed) > 0 {
			changes = append([]change{{"IterationUpdated", map[string]any{"fields": changed}}}, changes...)
		}
		return changes, nil
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Start makes a scheduled iteration active (iteration.manage), within the
// project's maximum of active iterations. IterationStarted lists the issues
// it holds: its committed scope.
func (s *Service) Start(ctx context.Context, pc project.Context, id uuid.UUID, version int32) (Iteration, error) {
	return s.mutate(ctx, pc, id, version, func(tx pgx.Tx, cur db.Iteration, next *db.Iteration) ([]change, error) {
		switch cur.State {
		case StateScheduled:
		case StatePlanned:
			return nil, &apperr.Error{Status: 409, Code: "iteration_not_scheduled", Detail: "an iteration needs dates to start"}
		default:
			return nil, apperr.Conflict("iteration_not_startable")
		}
		q := db.New(tx)
		if err := q.LockProjectIterations(ctx, db.LockProjectIterationsParams{WorkspaceID: cur.WorkspaceID, ID: cur.ProjectID}); err != nil {
			return nil, err
		}
		settings, err := s.settings(ctx, tx, pc)
		if err != nil {
			return nil, err
		}
		active, err := q.CountActive(ctx, db.CountActiveParams{WorkspaceID: cur.WorkspaceID, ProjectID: cur.ProjectID})
		if err != nil {
			return nil, err
		}
		if int(active) >= settings.MaxActive.Value {
			return nil, &apperr.Error{Status: 409, Code: "too_many_active",
				Detail: fmt.Sprintf("the project allows %d active iterations at a time", settings.MaxActive.Value)}
		}
		issues, err := q.IterationIssueIDs(ctx, db.IterationIssueIDsParams{WorkspaceID: cur.WorkspaceID, IterationID: uuid.NullUUID{UUID: cur.ID, Valid: true}})
		if err != nil {
			return nil, err
		}
		now := s.now()
		next.State, next.StartedAt = StateActive, &now
		return []change{{"IterationStarted", map[string]any{"issue_ids": nonNil(issues)}}}, nil
	})
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}

// Complete closes an active iteration (iteration.manage) under the
// completion policy in effect, which the policy consumer then applies to
// its issues. With archive_done_rollover_rest, rolloverTo names the open
// iteration of the project that takes the unfinished issues; invalid sends
// them to the backlog.
func (s *Service) Complete(ctx context.Context, pc project.Context, id uuid.UUID, version int32, rolloverTo uuid.NullUUID) (Iteration, error) {
	return s.mutate(ctx, pc, id, version, func(tx pgx.Tx, cur db.Iteration, next *db.Iteration) ([]change, error) {
		if cur.State != StateActive {
			return nil, apperr.Conflict("iteration_not_active")
		}
		settings, err := s.settings(ctx, tx, pc)
		if err != nil {
			return nil, err
		}
		c := Completion{Policy: settings.CompletionPolicy.Value}
		if c.Policy == configdef.CompletionRollover && rolloverTo.Valid {
			to, err := db.New(tx).Iteration(ctx, db.IterationParams{WorkspaceID: cur.WorkspaceID, ProjectID: cur.ProjectID, ID: rolloverTo.UUID})
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (to.ID == cur.ID || !open(to.State))) {
				return nil, apperr.Invalid(apperr.Field{Field: "rollover_to", Code: "invalid"})
			}
			if err != nil {
				return nil, err
			}
			c.RolloverTo = rolloverTo
		}
		raw, _ := json.Marshal(c)
		now := s.now()
		next.State, next.CompletedAt, next.Completion = StateCompleted, &now, raw
		return []change{{"IterationCompleted", map[string]any{"policy": c.Policy, "rollover_to": c.RolloverTo}}}, nil
	})
}

// Archive hides a completed iteration from planning (iteration.manage).
func (s *Service) Archive(ctx context.Context, pc project.Context, id uuid.UUID, version int32) (Iteration, error) {
	return s.mutate(ctx, pc, id, version, func(_ pgx.Tx, cur db.Iteration, next *db.Iteration) ([]change, error) {
		switch cur.State {
		case StateArchived:
			return nil, nil
		case StateCompleted:
		default:
			return nil, apperr.Conflict("iteration_not_completed")
		}
		now := s.now()
		next.State, next.ArchivedAt = StateArchived, &now
		return []change{{"IterationArchived", nil}}, nil
	})
}

// Delete hard-deletes an iteration that isn't active (iteration.manage):
// its issues go back to the backlog, its events are scrubbed, and an
// IterationDeleted tombstone remains (ADR-0012).
func (s *Service) Delete(ctx context.Context, pc project.Context, id uuid.UUID) error {
	if err := s.checkProject(ctx, pc); err != nil {
		return err
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockIteration(ctx, db.LockIterationParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("iteration_not_found")
		}
		if err != nil {
			return err
		}
		if cur.State == StateActive {
			return apperr.Conflict("iteration_active")
		}
		ids, err := q.IterationIssueIDs(ctx, db.IterationIssueIDsParams{WorkspaceID: cur.WorkspaceID, IterationID: uuid.NullUUID{UUID: cur.ID, Valid: true}})
		if err != nil {
			return err
		}
		if _, err := s.issues.SetIteration(ctx, tx, pc.Context, cur.ProjectID, ids, uuid.NullUUID{}, "iteration_deleted", true); err != nil {
			return err
		}
		if err := q.DeleteIteration(ctx, db.DeleteIterationParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID}); err != nil {
			return err
		}
		return eventlog.Tombstone(ctx, tx, s.event(pc.Context, "iteration", cur.ID, cur.ProjectID, cur.Version+1, "IterationDeleted", nil))
	})
}

// LoadInput lists issues (keys) to add to an iteration and to take out.
type LoadInput struct {
	Add    []string
	Remove []string
}

// Load moves issues into an iteration and out of it (iteration.manage):
// IterationLoaded on it, IssueIterationChanged on each issue. An issue
// taken from another open iteration is recorded as leaving that one too.
// Only issues of the project, not archived, can be loaded; completed
// iterations don't change.
func (s *Service) Load(ctx context.Context, pc project.Context, id uuid.UUID, in LoadInput) (Iteration, error) {
	if err := s.checkProject(ctx, pc); err != nil {
		return Iteration{}, err
	}
	if len(in.Add)+len(in.Remove) > maxLoad {
		return Iteration{}, apperr.Invalid(apperr.Field{Field: "add", Code: "too_many"})
	}
	var fields apperr.Fields
	add := s.issueIDs(ctx, pc, &fields, "add", in.Add)
	remove := s.issueIDs(ctx, pc, &fields, "remove", in.Remove)
	if err := fields.Err(); err != nil {
		return Iteration{}, err
	}
	var saved db.Iteration
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockIteration(ctx, db.LockIterationParams{WorkspaceID: pc.Workspace.ID, ProjectID: pc.Project.ID, ID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("iteration_not_found")
		}
		if err != nil {
			return err
		}
		saved = cur
		if !open(cur.State) {
			return apperr.Conflict("iteration_closed")
		}
		this := uuid.NullUUID{UUID: cur.ID, Valid: true}
		before, err := s.issues.SetIteration(ctx, tx, pc.Context, cur.ProjectID, add, this, "", false)
		if err != nil {
			return err
		}
		var added []uuid.UUID
		left := map[uuid.UUID][]uuid.UUID{} // other iterations they leave
		for _, b := range before {
			if b.IterationID == this {
				continue
			}
			added = append(added, b.ID)
			if b.IterationID.Valid {
				left[b.IterationID.UUID] = append(left[b.IterationID.UUID], b.ID)
			}
		}
		// Only what is in this iteration can be taken out of it.
		var out []uuid.UUID
		if len(remove) > 0 {
			inside, err := q.IterationIssueIDs(ctx, db.IterationIssueIDsParams{WorkspaceID: cur.WorkspaceID, IterationID: this})
			if err != nil {
				return err
			}
			for _, r := range remove {
				if slices.Contains(inside, r) && !slices.Contains(add, r) {
					out = append(out, r)
				}
			}
			if _, err := s.issues.SetIteration(ctx, tx, pc.Context, cur.ProjectID, out, uuid.NullUUID{}, "", false); err != nil {
				return err
			}
		}
		for other, ids := range left {
			if err := s.recordLeft(ctx, tx, pc.Context, other, ids, "moved", this); err != nil {
				return err
			}
		}
		if len(added) == 0 && len(out) == 0 {
			return nil
		}
		if saved, err = q.BumpIteration(ctx, db.BumpIterationParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, UpdatedAt: s.now()}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(pc.Context, "iteration", saved.ID, saved.ProjectID, saved.Version, "IterationLoaded",
			map[string]any{"added": nonNil(added), "removed": nonNil(out)}))
	})
	if err != nil {
		return Iteration{}, err
	}
	return s.Iteration(ctx, pc, saved.ID)
}

// issueIDs resolves issue keys of the project.
func (s *Service) issueIDs(ctx context.Context, pc project.Context, fields *apperr.Fields, field string, keys []string) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(keys))
	for _, k := range keys {
		ic, err := s.issues.Enter(ctx, pc.Context, k)
		if err != nil || ic.Issue.ProjectID != pc.Project.ID {
			fields.Add(field, "unknown")
			continue
		}
		if !slices.Contains(ids, ic.Issue.ID) {
			ids = append(ids, ic.Issue.ID)
		}
	}
	return ids
}

// recordLeft records issues leaving an open iteration other than by its
// own load command: IterationLoaded with what was removed and why (to:
// where they went, when known). Completed iterations keep their record as
// it was at completion.
func (s *Service) recordLeft(ctx context.Context, tx pgx.Tx, wc workspace.Context, iterationID uuid.UUID, ids []uuid.UUID,
	reason string, to uuid.NullUUID) error {
	q := db.New(tx)
	it, err := q.LockIterationByID(ctx, db.LockIterationByIDParams{WorkspaceID: wc.Workspace.ID, ID: iterationID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !open(it.State) {
		return nil
	}
	it, err = q.BumpIteration(ctx, db.BumpIterationParams{WorkspaceID: it.WorkspaceID, ID: it.ID, UpdatedAt: s.now()})
	if err != nil {
		return err
	}
	payload := map[string]any{"added": []uuid.UUID{}, "removed": ids, "reason": reason}
	if to.Valid {
		payload["to"] = to.UUID
	}
	return eventlog.Append(ctx, tx, s.event(wc, "iteration", it.ID, it.ProjectID, it.Version, "IterationLoaded", payload))
}

// issuesLeft is the issue module's IterationHook.
func (s *Service) issuesLeft(ctx context.Context, tx pgx.Tx, wc workspace.Context, iterationID uuid.UUID, ids []uuid.UUID, reason string) error {
	return s.recordLeft(ctx, tx, wc, iterationID, ids, reason, uuid.NullUUID{})
}
