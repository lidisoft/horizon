package issue

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/document"
	"github.com/lidisoft/horizon/internal/issue/db"
)

// Update changes the fields that are set (issue.edit). Description gets
// its own event (IssueDescribed); title, labels, dates and custom fields are
// IssueUpdated; a new parent is IssueParentChanged; a new type, whose
// workflow may differ, needs StatusID in the new workflow (IssueTypeChanged).
type Update struct {
	Title       *string
	Description Text
	Labels      *[]uuid.UUID
	StartDate   *string
	DueDate     *string
	Fields      map[string]json.RawMessage
	// ParentKey "" removes the parent.
	ParentKey *string
	TypeID    *uuid.UUID
	StatusID  *uuid.UUID
}

// Update applies an Update. The result carries the description document.
func (s *Service) Update(ctx context.Context, ic Context, version int32, in Update) (Issue, error) {
	i, err := s.update(ctx, ic, version, in)
	if err != nil {
		return i, err
	}
	saved, err := db.New(s.pool).IssueByID(ctx, db.IssueByIDParams{WorkspaceID: ic.Workspace.ID, ID: i.ID})
	if err != nil {
		return i, err
	}
	doc, _, err := descriptionOf(ctx, s.pool, saved)
	if err != nil {
		return i, err
	}
	i.DescriptionDoc = docJSON(doc)
	return i, nil
}

func (s *Service) update(ctx context.Context, ic Context, version int32, in Update) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueEdit, false, func(e *edit) ([]change, error) {
		var fields apperr.Fields
		var changed []string
		var changes []change
		i := e.next
		if in.Title != nil {
			if t := validTitle(&fields, *in.Title); t != i.Title {
				i.Title = t
				changed = append(changed, "title")
			}
		}
		if in.Description.IsSet() {
			if doc := in.Description.parse(&fields, "description"); doc != nil {
				if err := checkImages(ctx, e.tx, &fields, "description", i.ID, doc); err != nil {
					return nil, err
				}
				cur, _, err := descriptionOf(ctx, e.tx, e.cur)
				if err != nil {
					return nil, err
				}
				if !doc.Eq(cur) {
					e.description = doc
					i.Description = document.PlainText(doc)
					changes = append(changes, change{"IssueDescribed", nil})
				}
			}
		}
		if in.StartDate != nil {
			d, ok := parseDate(*in.StartDate)
			if !ok {
				fields.Add("start_date", "invalid")
			} else if d != i.StartDate {
				i.StartDate = d
				changed = append(changed, "start_date")
			}
		}
		if in.DueDate != nil {
			d, ok := parseDate(*in.DueDate)
			if !ok {
				fields.Add("due_date", "invalid")
			} else if d != i.DueDate {
				i.DueDate = d
				changed = append(changed, "due_date")
			}
		}
		checkDates(&fields, i.StartDate, i.DueDate)
		if in.Labels != nil {
			labels := validLabels(&fields, e.r, *in.Labels)
			e.labels = &labels
			changed = append(changed, "labels")
		}
		if in.TypeID != nil && *in.TypeID != i.TypeID {
			ch, err := s.changeType(ctx, e, &fields, *in.TypeID, in.StatusID)
			if err != nil {
				return nil, err
			}
			changes = append(changes, ch...)
		}
		if len(in.Fields) > 0 {
			values, err := s.validateFields(ctx, e.tx, &fields, e.r, i.TypeID, i.Fields, in.Fields, false)
			if err != nil {
				return nil, err
			}
			i.Fields = values
			changed = append(changed, "fields")
		}
		if in.ParentKey != nil {
			ch, err := s.changeParent(ctx, ic, e, &fields, *in.ParentKey)
			if err != nil {
				return nil, err
			}
			changes = append(changes, ch...)
		} else if in.TypeID != nil {
			it, _ := findType(e.r, i.TypeID)
			var parent *db.Issue
			if i.ParentID.Valid {
				p, err := db.New(e.tx).IssueByID(ctx, db.IssueByIDParams{WorkspaceID: i.WorkspaceID, ID: i.ParentID.UUID})
				if err != nil {
					return nil, err
				}
				parent = &p
			}
			checkParentRule(&fields, e.r, it, parent)
		}
		if err := fields.Err(); err != nil {
			return nil, err
		}
		if len(changed) > 0 {
			changes = append(changes, change{"IssueUpdated", map[string]any{"fields": changed}})
		}
		return changes, nil
	})
}

// changeType moves the issue to another type; when the workflow differs,
// statusID names its status in the new one.
func (s *Service) changeType(ctx context.Context, e *edit, fields *apperr.Fields, typeID uuid.UUID, statusID *uuid.UUID) ([]change, error) {
	i := e.next
	it, ok := findType(e.r, typeID)
	if !ok {
		fields.Add("type_id", "unknown")
		return nil, nil
	}
	old, _ := findType(e.r, i.TypeID)
	from := i.StatusID
	if it.WorkflowID != old.WorkflowID {
		wf := e.r.Workflows[it.WorkflowID]
		if statusID == nil || !slices.ContainsFunc(wf.Statuses, func(st configdef.Status) bool { return st.ID == *statusID }) {
			fields.Add("status_id", "required_in_workflow")
			return nil, nil
		}
		i.StatusID = *statusID
	}
	// Children must still accept this issue as their parent.
	children, err := db.New(e.tx).Children(ctx, db.ChildrenParams{WorkspaceID: i.WorkspaceID, ParentID: uuid.NullUUID{UUID: i.ID, Valid: true}})
	if err != nil {
		return nil, err
	}
	for _, c := range children {
		if ct, ok := findType(e.r, c.TypeID); ok && len(ct.ParentTypeIDs) > 0 && !slices.Contains(ct.ParentTypeIDs, typeID) {
			fields.Add("type_id", "children_disallow")
			break
		}
	}
	i.TypeID = typeID
	changes := []change{{"IssueTypeChanged", map[string]any{"from": old.ID, "to": typeID}}}
	if i.StatusID != from {
		s.trackOutcome(e, from)
		changes = append(changes, change{"IssueStatusUpdated", statusFacts(e.r, i.StatusID, map[string]any{"from": from, "to": i.StatusID})})
	}
	return changes, nil
}

// changeParent sets or removes the parent: the type's rule applies, and the
// parent chain stays free of cycles.
func (s *Service) changeParent(ctx context.Context, ic Context, e *edit, fields *apperr.Fields, key string) ([]change, error) {
	i := e.next
	it, _ := findType(e.r, i.TypeID)
	if key == "" {
		if !i.ParentID.Valid {
			return nil, nil
		}
		checkParentRule(fields, e.r, it, nil)
		from := i.ParentID
		i.ParentID = uuid.NullUUID{}
		return []change{{"IssueParentChanged", map[string]any{"from": from, "to": nil}}}, nil
	}
	parent, err := s.findParent(ctx, e.tx, ic.Context.Context, key)
	if err != nil {
		fields.Add("parent", "unknown")
		return nil, nil
	}
	if parent.ID == i.ParentID.UUID {
		return nil, nil
	}
	chain, err := db.New(e.tx).Ancestors(ctx, parent.ID)
	if err != nil {
		return nil, err
	}
	if slices.Contains(chain, i.ID) {
		fields.Add("parent", "cycle")
		return nil, nil
	}
	checkParentRule(fields, e.r, it, &parent)
	from := i.ParentID
	i.ParentID = uuid.NullUUID{UUID: parent.ID, Valid: true}
	return []change{{"IssueParentChanged", map[string]any{"from": from, "to": parent.ID}}}, nil
}

// trackOutcome records when work started (first entry into a started
// category) and when it was resolved (entry into a final one; leaving a
// final category clears it).
func (s *Service) trackOutcome(e *edit, from uuid.UUID) {
	i := e.next
	now := s.now()
	to := outcomeOf(e.r, i.StatusID)
	if to == configdef.OutcomeStarted && i.StartedAt == nil {
		i.StartedAt = &now
	}
	if configdef.Open(to) {
		i.ResolvedAt = nil
	} else if configdef.Open(outcomeOf(e.r, from)) || i.ResolvedAt == nil {
		i.ResolvedAt = &now
	}
}

// AvailableTransition is a transition the issue can take from its status,
// and whether the caller may take it now.
type AvailableTransition struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	ToStatus uuid.UUID `json:"to_status_id"`
	// Blocked names why it can't be taken now (forbidden, children_open,
	// required:<field>); empty when it can.
	Blocked string `json:"blocked,omitempty"`
}

// candidates returns the transitions from the issue's status: specific
// ones first, then "from any status" to statuses not already covered.
func candidates(r *configdef.Resolved, i db.Issue) []configdef.Transition {
	it, _ := findType(r, i.TypeID)
	wf := r.Workflows[it.WorkflowID]
	var specific, fromAny []configdef.Transition
	for _, tr := range wf.Transitions {
		switch {
		case tr.To == i.StatusID:
		case tr.From.Valid && tr.From.UUID == i.StatusID:
			specific = append(specific, tr)
		case !tr.From.Valid:
			fromAny = append(fromAny, tr)
		}
	}
	for _, tr := range fromAny {
		if !slices.ContainsFunc(specific, func(x configdef.Transition) bool { return x.To == tr.To }) {
			specific = append(specific, tr)
		}
	}
	return specific
}

// Transitions lists what the issue can move to, for the status picker.
func (s *Service) Transitions(ctx context.Context, ic Context) ([]AvailableTransition, error) {
	r, err := s.config.Resolve(ctx, s.pool, ic.Workspace.ID, uuid.NullUUID{UUID: ic.Issue.ProjectID, Valid: true})
	if err != nil {
		return nil, err
	}
	canTransition := s.require(ctx, ic.Context, access.IssueTransition) == nil
	out := []AvailableTransition{}
	for _, tr := range candidates(r, ic.Issue) {
		at := AvailableTransition{ID: tr.ID, Name: tr.Name, ToStatus: tr.To}
		if !canTransition || ic.Issue.ArchivedAt != nil {
			at.Blocked = "forbidden"
		} else if ok, err := s.access.HoldsRole(ctx, ic.Principal, ic.Scope(), tr.Rules.Roles); err != nil {
			return nil, err
		} else if !ok {
			at.Blocked = "forbidden"
		}
		out = append(out, at)
	}
	return out, nil
}

// Transition moves the issue to toStatus through an allowed transition of
// its workflow (issue.transition), checking the transition's conditions and
// validators and running its post-actions.
func (s *Service) Transition(ctx context.Context, ic Context, version int32, toStatus uuid.UUID) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueTransition, false, func(e *edit) ([]change, error) {
		var tr *configdef.Transition
		for _, c := range candidates(e.r, e.cur) {
			if c.To == toStatus {
				tr = &c
				break
			}
		}
		if tr == nil {
			return nil, apperr.Conflict("transition_not_allowed")
		}
		ok, err := s.access.HoldsRole(ctx, ic.Principal, ic.Scope(), tr.Rules.Roles)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, &apperr.Error{Status: 403, Code: "transition_forbidden", Detail: "the transition " + tr.Name + " is limited to some roles"}
		}
		return s.applyTransition(ctx, e, ic, *tr)
	})
}

// Status targets of SystemTransition.
const (
	// ToCompleted is the nearest completed status.
	ToCompleted = configdef.MergeCompleted
	// maxSteps bounds the transitions an automation chains.
	maxSteps = 6
)

// SystemTransition moves the issue, for an automation (a merged pull
// request), to the status of its workflow named to (or the nearest
// completed one for ToCompleted), through the fewest allowed transitions:
// the workflow decides the way, each step's validators, children rule
// and post-actions apply, and one IssueStatusUpdated records the move
// (with the transitions taken, and why). It has no actor and needs no
// permission or role. moved is false when no way leads there, the issue
// is there already or archived, or a rule refuses.
func (s *Service) SystemTransition(ctx context.Context, workspaceID, issueID uuid.UUID, to string, why map[string]any) (moved bool, err error) {
	var ic Context
	ic.Workspace.ID = workspaceID
	err = s.pool.QueryRow(ctx, `SELECT p.id, p.key FROM issues i JOIN projects p ON p.id = i.project_id
		WHERE i.workspace_id = $1 AND i.id = $2`, workspaceID, issueID).Scan(&ic.Project.ID, &ic.Project.Key)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	ic.Issue.ID, ic.Issue.WorkspaceID, ic.Issue.ProjectID = issueID, workspaceID, ic.Project.ID
	_, err = s.mutate(ctx, ic, 0, "", false, func(e *edit) ([]change, error) {
		target := func(id uuid.UUID) bool {
			st, _, ok := e.r.Status(id)
			if !ok {
				return false
			}
			if to == ToCompleted {
				return e.r.Categories[st.CategoryID].Outcome == configdef.OutcomeCompleted
			}
			return strings.EqualFold(st.Name, to)
		}
		if target(e.cur.StatusID) {
			return nil, nil
		}
		path := shortestPath(e.r, e.cur, target)
		if path == nil {
			return nil, nil
		}
		from := e.cur.StatusID
		var changes []change
		ids := make([]uuid.UUID, len(path))
		for i, tr := range path {
			if err := s.checkTransition(ctx, e, tr); err != nil {
				return nil, err
			}
			e.next.StatusID = tr.To
			changes = append(changes, s.postActions(e, ic, tr)...)
			ids[i] = tr.ID
		}
		last := path[len(path)-1]
		s.trackOutcome(e, from)
		payload := map[string]any{"from": from, "to": last.To, "transition_id": last.ID}
		if len(ids) > 1 {
			payload["transition_ids"] = ids
		}
		for k, v := range why {
			payload[k] = v
		}
		moved = true
		return append([]change{{"IssueStatusUpdated", statusFacts(e.r, last.To, payload)}}, changes...), nil
	})
	if apperr.Is(err, "children_open") || apperr.Is(err, "transition_requires_fields") || apperr.Is(err, "issue_archived") {
		return false, nil
	}
	return moved && err == nil, err
}

// shortestPath finds the fewest transitions of the issue's workflow from
// its status to one target accepts (breadth first, in the workflow's
// order), or nil.
func shortestPath(r *configdef.Resolved, i db.Issue, target func(uuid.UUID) bool) []configdef.Transition {
	type node struct {
		status uuid.UUID
		path   []configdef.Transition
	}
	seen := map[uuid.UUID]bool{i.StatusID: true}
	queue := []node{{status: i.StatusID}}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if len(n.path) == maxSteps {
			continue
		}
		at := i
		at.StatusID = n.status
		for _, tr := range candidates(r, at) {
			if seen[tr.To] {
				continue
			}
			seen[tr.To] = true
			path := append(slices.Clone(n.path), tr)
			if target(tr.To) {
				return path
			}
			queue = append(queue, node{status: tr.To, path: path})
		}
	}
	return nil
}

// applyTransition checks a transition's rules and moves the issue.
func (s *Service) applyTransition(ctx context.Context, e *edit, ic Context, tr configdef.Transition) ([]change, error) {
	if err := s.checkTransition(ctx, e, tr); err != nil {
		return nil, err
	}
	from := e.next.StatusID
	e.next.StatusID = tr.To
	s.trackOutcome(e, from)
	changes := []change{{"IssueStatusUpdated", statusFacts(e.r, tr.To, map[string]any{"from": from, "to": tr.To, "transition_id": tr.ID})}}
	return append(changes, s.postActions(e, ic, tr)...), nil
}

// checkTransition applies a transition's children rule and validators.
func (s *Service) checkTransition(ctx context.Context, e *edit, tr configdef.Transition) error {
	if tr.Rules.ChildrenFinal {
		var open int
		err := e.tx.QueryRow(ctx, `
			SELECT count(*) FROM issues c
			JOIN statuses s ON s.id = c.status_id
			JOIN status_categories sc ON sc.id = s.category_id
			WHERE c.parent_id = $1 AND sc.outcome IN ('unstarted', 'started')`, e.next.ID).Scan(&open)
		if err != nil {
			return err
		}
		if open > 0 {
			return apperr.Conflict("children_open")
		}
	}
	var missing apperr.Fields
	for _, f := range tr.Rules.Validators {
		if !isSet(e, f) {
			missing.Add(f, "required")
		}
	}
	if len(missing) > 0 {
		return &apperr.Error{Status: 422, Code: "transition_requires_fields", Fields: missing}
	}
	return nil
}

// postActions runs a transition's post-actions and tells what they changed.
func (s *Service) postActions(e *edit, ic Context, tr configdef.Transition) []change {
	i := e.next
	var changes []change
	for _, pa := range tr.Rules.PostActions {
		before, iteration, release := i.AssigneeID, i.IterationID, i.ReleaseID
		switch pa.Op {
		case "set_actor":
			if ic.Principal.User.ID != uuid.Nil { // an automation has no actor to assign
				i.AssigneeID = uuid.NullUUID{UUID: ic.Principal.User.ID, Valid: true}
			}
		default:
			clearField(e, pa.Field)
		}
		switch pa.Field {
		case "assignee":
			if before != i.AssigneeID {
				changes = append(changes, change{"IssueAssigned", map[string]any{"from": before, "to": i.AssigneeID, "by": "workflow"}})
			}
		case "iteration":
			if iteration.Valid {
				changes = append(changes, change{"IssueIterationChanged", map[string]any{"from": iteration, "to": nil, "by": "workflow"}})
			}
		case "release":
			if release.Valid {
				changes = append(changes, change{"IssueReleaseChanged", map[string]any{"from": release, "to": nil, "by": "workflow"}})
			}
		default:
			changes = append(changes, change{"IssueUpdated", map[string]any{"fields": []string{pa.Field}, "by": "workflow"}})
		}
	}
	return changes
}

// Assign sets or clears (uuid.Nil) the assignee (issue.assign).
func (s *Service) Assign(ctx context.Context, ic Context, version int32, userID uuid.UUID) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueAssign, false, func(e *edit) ([]change, error) {
		to := uuid.NullUUID{UUID: userID, Valid: userID != uuid.Nil}
		if to == e.next.AssigneeID {
			return nil, nil
		}
		if to.Valid {
			var fields apperr.Fields
			s.checkAssignee(ctx, e.tx, &fields, e.next.WorkspaceID, userID)
			if err := fields.Err(); err != nil {
				return nil, err
			}
		}
		from := e.next.AssigneeID
		e.next.AssigneeID = to
		return []change{{"IssueAssigned", map[string]any{"from": from, "to": to}}}, nil
	})
}

// Estimate sets or clears ("") the estimate: a value of the project's
// scale (issue.estimate).
func (s *Service) Estimate(ctx context.Context, ic Context, version int32, value string) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueEstimate, false, func(e *edit) ([]change, error) {
		var fields apperr.Fields
		v := validEstimate(&fields, e.r, value)
		if err := fields.Err(); err != nil {
			return nil, err
		}
		if equalPtr(v, e.next.Estimate) {
			return nil, nil
		}
		from := e.next.Estimate
		e.next.Estimate = v
		payload := map[string]any{"from": from, "to": v}
		if w, ok := weightOf(e.r, v); ok {
			payload["weight"] = w // the scale's weight now, for reports
		}
		return []change{{"IssueEstimated", payload}}, nil
	})
}

// Prioritize sets the priority (issue.edit).
func (s *Service) Prioritize(ctx context.Context, ic Context, version int32, priority string) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueEdit, false, func(e *edit) ([]change, error) {
		if !slices.Contains(Priorities, priority) {
			return nil, apperr.Invalid(apperr.Field{Field: "priority", Code: "invalid"})
		}
		if priority == e.next.Priority {
			return nil, nil
		}
		from := e.next.Priority
		e.next.Priority = priority
		return []change{{"IssuePrioritized", map[string]any{"from": from, "to": priority}}}, nil
	})
}

// Rank places the issue right after another of its project (afterKey), or
// first when afterKey is empty (issue.rank).
func (s *Service) Rank(ctx context.Context, ic Context, version int32, afterKey string) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueRank, false, func(e *edit) ([]change, error) {
		q := db.New(e.tx)
		lo := ""
		if afterKey != "" {
			after, err := s.Enter(ctx, ic.Context.Context, afterKey)
			if err != nil || after.Issue.ProjectID != e.cur.ProjectID {
				return nil, apperr.Invalid(apperr.Field{Field: "after", Code: "unknown"})
			}
			if after.Issue.ID == e.cur.ID {
				return nil, nil
			}
			lo = after.Issue.Rank
		}
		n, err := q.NeighbourRanks(ctx, db.NeighbourRanksParams{WorkspaceID: e.cur.WorkspaceID, ProjectID: e.cur.ProjectID, Rank: lo})
		if err != nil {
			return nil, err
		}
		hi := n.After
		if hi == e.cur.Rank {
			// The issue already sits right after lo: nothing to do.
			return nil, nil
		}
		rank, err := Between(lo, hi)
		if err != nil {
			return nil, err
		}
		from := e.next.Rank
		e.next.Rank = rank
		return []change{{"IssueRanked", map[string]any{"from": from, "to": rank}}}, nil
	})
}

// SetArchived archives (read-only) or restores an issue (issue.archive).
func (s *Service) SetArchived(ctx context.Context, ic Context, version int32, archived bool) (Issue, error) {
	return s.mutate(ctx, ic, version, access.IssueArchive, true, func(e *edit) ([]change, error) {
		if (e.next.ArchivedAt != nil) == archived {
			return nil, nil
		}
		if archived {
			now := s.now()
			e.next.ArchivedAt = &now
			return []change{{"IssueArchived", nil}}, nil
		}
		e.next.ArchivedAt = nil
		return []change{{"IssueUnarchived", nil}}, nil
	})
}

func equalPtr(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
