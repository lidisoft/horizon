// Package configdef owns the configurable definitions issues follow: status
// categories, workflows and issue types (docs/configuration.md), and the
// resolver that combines a workspace's definitions with a project's
// (ADR-0011).
//
// A definition has workspace scope (ProjectID invalid) or project scope. A
// project inherits every workspace definition it hasn't switched off, plus
// its own; inherited definitions are edited in the workspace, or copied into
// the project (which switches the original off there).
package configdef

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/lidisoft/horizon/internal/configdef/db"
)

// Definition types, as stored in config_disables and named in events.
const (
	TypeCategory  = "status_category"
	TypeWorkflow  = "workflow"
	TypeIssueType = "issue_type"
)

// Outcomes of status categories (docs/configuration.md#status-categories).
const (
	OutcomeUnstarted = "unstarted"
	OutcomeStarted   = "started"
	OutcomeCompleted = "completed"
	OutcomeAbandoned = "abandoned"
)

// Open reports whether an outcome is open (not final).
func Open(outcome string) bool { return outcome == OutcomeUnstarted || outcome == OutcomeStarted }

func validOutcome(o string) bool {
	return o == OutcomeUnstarted || o == OutcomeStarted || o == OutcomeCompleted || o == OutcomeAbandoned
}

// Parent rules of issue types.
const (
	ParentAny      = "any"
	ParentNone     = "none"     // can't have a parent (Epic)
	ParentRequired = "required" // must have one (Sub-task)
)

// Colours definitions can take; the UI maps them to swatch tokens.
var Colours = []string{"blue", "green", "teal", "purple", "pink", "coral", "orange", "yellow", "grey"}

// Icons issue types can take, from the LDS sprite.
var Icons = []string{
	"layers", "bookmark", "check", "alert-circle", "corner-down-right", "flag", "rocket", "milestone",
	"git-branch", "git-pull-request", "message-square", "eye", "search", "kanban", "list", "repeat",
	"tag", "user", "users", "calendar", "lock", "inbox", "paperclip", "link",
}

// Origin says where a definition comes from, seen from a scope.
type Origin struct {
	// Inherited is true for a workspace definition seen from a project.
	Inherited bool `json:"inherited"`
	// Disabled is true when the project switched the inherited one off.
	Disabled bool `json:"disabled"`
}

// Category is a status category.
type Category struct {
	ID        uuid.UUID     `json:"id"`
	ProjectID uuid.NullUUID `json:"project_id" swaggertype:"string"`
	Name      string        `json:"name" example:"In progress"`
	Colour    string        `json:"colour"`
	Outcome   string        `json:"outcome" enums:"unstarted,started,completed,abandoned"`
	Position  int32         `json:"position"`
	Origin
	// Statuses using it, across the workspace's workflows.
	Statuses  []string `json:"statuses"`
	Workflows int      `json:"workflows"`
	Version   int32    `json:"version"`
}

func categoryFrom(c db.StatusCategory) Category {
	return Category{ID: c.ID, ProjectID: c.ProjectID, Name: c.Name, Colour: c.Colour, Outcome: c.Outcome, Position: c.Position, Statuses: []string{}, Version: c.Version}
}

// Status is a published status of a workflow.
type Status struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	CategoryID uuid.UUID `json:"category_id"`
	Initial    bool      `json:"initial"`
}

// Rules restrict and extend a transition (docs/configuration.md#workflows).
type Rules struct {
	// Roles that may perform it; empty: anyone holding issue.transition.
	Roles []uuid.UUID `json:"roles,omitempty"`
	// ChildrenFinal requires every child issue to be in a final category.
	ChildrenFinal bool `json:"children_final,omitempty"`
	// Validators are fields that must be set to enter the target status.
	Validators []string `json:"validators,omitempty"`
	// PostActions run after the transition.
	PostActions []PostAction `json:"post_actions,omitempty"`
}

// PostAction changes a field after a transition.
type PostAction struct {
	Field string `json:"field" example:"assignee"`
	// Op is clear, or set_actor (assignee only: the person transitioning).
	Op string `json:"op" enums:"clear,set_actor"`
}

// Built-in issue fields rules may name; custom fields join with their ids.
var BuiltinFields = []string{"assignee", "estimate", "priority", "due_date", "start_date", "iteration", "release", "description", "labels"}

// Transition is a published transition.
type Transition struct {
	ID   uuid.UUID     `json:"id"`
	From uuid.NullUUID `json:"from" swaggertype:"string"` // invalid: from any status
	To   uuid.UUID     `json:"to"`
	Name string        `json:"name"`
	// Rules has its zero value when the transition is unrestricted.
	Rules Rules `json:"rules"`
}

// Draft is the editable copy of a workflow's statuses and transitions.
type Draft struct {
	Statuses    []Status     `json:"statuses"`
	Transitions []Transition `json:"transitions"`
}

// Workflow is a state machine bound to issue types.
type Workflow struct {
	ID          uuid.UUID     `json:"id"`
	ProjectID   uuid.NullUUID `json:"project_id" swaggertype:"string"`
	Name        string        `json:"name" example:"Engineering"`
	CopiedFrom  uuid.NullUUID `json:"copied_from" swaggertype:"string"`
	Statuses    []Status      `json:"statuses"`
	Transitions []Transition  `json:"transitions"`
	// Draft is the shared work in progress, if any.
	Draft          *Draft        `json:"draft"`
	DraftVersion   int32         `json:"draft_version"`
	DraftUpdatedBy uuid.NullUUID `json:"draft_updated_by" swaggertype:"string"`
	DraftUpdatedAt *time.Time    `json:"draft_updated_at"`
	PublishedAt    time.Time     `json:"published_at"`
	Origin
	// UsedBy lists the issue types bound to it, in the scope asked.
	UsedBy  []uuid.UUID `json:"used_by"`
	Version int32       `json:"version"`
}

// IssueType is an issue type.
type IssueType struct {
	ID            uuid.UUID     `json:"id"`
	ProjectID     uuid.NullUUID `json:"project_id" swaggertype:"string"`
	Name          string        `json:"name" example:"Bug"`
	Icon          string        `json:"icon" example:"alert-circle"`
	Colour        string        `json:"colour"`
	WorkflowID    uuid.UUID     `json:"workflow_id"`
	ParentRule    string        `json:"parent_rule" enums:"any,none,required"`
	ParentTypeIDs []uuid.UUID   `json:"parent_type_ids"`
	CopiedFrom    uuid.NullUUID `json:"copied_from" swaggertype:"string"`
	Position      int32         `json:"position"`
	Origin
	Version int32 `json:"version"`
}

func issueTypeFrom(t db.IssueType) IssueType {
	ids := t.AllowedParentTypeIds
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return IssueType{
		ID: t.ID, ProjectID: t.ProjectID, Name: t.Name, Icon: t.Icon, Colour: t.Colour, WorkflowID: t.WorkflowID,
		ParentRule: t.ParentRule, ParentTypeIDs: ids, CopiedFrom: t.CopiedFrom, Position: t.Position, Version: t.Version,
	}
}

func rulesFrom(raw json.RawMessage) Rules {
	var r Rules
	_ = json.Unmarshal(raw, &r)
	return r
}
