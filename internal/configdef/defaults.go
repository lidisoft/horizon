package configdef

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// The built-in defaults (docs/configuration.md#catalogue), seeded in every
// new workspace. Migration 00008 seeds the same into workspaces that existed
// before.

type defaultCategory struct{ name, colour, outcome string }

var defaultCategories = []defaultCategory{
	{"Backlog", "grey", OutcomeUnstarted},
	{"Todo", "grey", OutcomeUnstarted},
	{"In progress", "blue", OutcomeStarted},
	{"Done", "green", OutcomeCompleted},
	{"Canceled", "grey", OutcomeAbandoned},
}

// defaultTransitions of the Default workflow, by status index (-1: any).
var defaultTransitions = []struct {
	from, to int
	name     string
}{
	{0, 1, "Plan"}, {1, 0, "Unplan"}, {1, 2, "Start"}, {2, 1, "Stop"},
	{2, 3, "Finish"}, {3, 2, "Reopen"}, {-1, 4, "Cancel"}, {4, 0, "Restore"},
}

type defaultType struct {
	name, icon, colour, parentRule string
	parents                        []string
}

var defaultTypes = []defaultType{
	{"Epic", "layers", "purple", ParentNone, nil},
	{"Story", "bookmark", "green", ParentAny, nil},
	{"Task", "check", "blue", ParentAny, nil},
	{"Bug", "alert-circle", "coral", ParentAny, nil},
	{"Sub-task", "corner-down-right", "teal", ParentRequired, []string{"Story", "Task", "Bug"}},
}

// SeedDefaults creates the default categories, workflow and issue types of
// a new workspace inside tx. Its signature fits workspace.Options.OnCreate.
func (s *Service) SeedDefaults(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, actorID uuid.NullUUID) error {
	q := db.New(tx)
	t := Target{WorkspaceID: workspaceID}
	t.Principal.User.ID = actorID.UUID
	now := s.now()

	cats := make([]uuid.UUID, len(defaultCategories))
	for i, c := range defaultCategories {
		row, err := q.CreateCategory(ctx, db.CreateCategoryParams{
			WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), Name: c.name, Colour: c.colour, Outcome: c.outcome,
			Position: int32(i), CreatedAt: now,
		})
		if err != nil {
			return err
		}
		cats[i] = row.ID
	}

	wf, err := q.CreateWorkflow(ctx, db.CreateWorkflowParams{WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), Name: "Default", PublishedAt: now})
	if err != nil {
		return err
	}
	d := Draft{}
	for i, c := range defaultCategories {
		d.Statuses = append(d.Statuses, Status{ID: uuid.Must(uuid.NewV7()), Name: c.name, CategoryID: cats[i], Initial: i == 0})
	}
	for _, tr := range defaultTransitions {
		from := uuid.NullUUID{}
		if tr.from >= 0 {
			from = uuid.NullUUID{UUID: d.Statuses[tr.from].ID, Valid: true}
		}
		d.Transitions = append(d.Transitions, Transition{ID: uuid.Must(uuid.NewV7()), From: from, To: d.Statuses[tr.to].ID, Name: tr.name})
	}
	if err := s.writePublished(ctx, tx, wf.ID, d, nil); err != nil {
		return err
	}

	byName := map[string]uuid.UUID{}
	for i, it := range defaultTypes {
		parents := []uuid.UUID{}
		for _, p := range it.parents {
			parents = append(parents, byName[p])
		}
		row, err := q.CreateIssueType(ctx, db.CreateIssueTypeParams{
			WorkspaceID: workspaceID, ID: uuid.Must(uuid.NewV7()), Name: it.name, Icon: it.icon, Colour: it.colour,
			WorkflowID: wf.ID, ParentRule: it.parentRule, AllowedParentTypeIds: parents, Position: int32(i), CreatedAt: now,
		})
		if err != nil {
			return err
		}
		byName[it.name] = row.ID
	}
	return eventlog.Append(ctx, tx, s.event(t, "workspace", workspaceID, 1, "WorkspaceConfigurationSeeded",
		map[string]any{"workflow_id": wf.ID}))
}
