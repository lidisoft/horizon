package configdef

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Limits of a workflow.
const (
	maxStatuses    = 50
	maxTransitions = 400
)

// workflows loads the workflows seen from t, with statuses, transitions,
// drafts, origins and the issue types bound to each.
func (s *Service) workflows(ctx context.Context, q *db.Queries, t Target, off map[string]map[uuid.UUID]bool) ([]Workflow, error) {
	rows, err := q.Workflows(ctx, db.WorkflowsParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	statuses, err := q.Statuses(ctx, ids)
	if err != nil {
		return nil, err
	}
	transitions, err := q.Transitions(ctx, ids)
	if err != nil {
		return nil, err
	}
	types, err := q.IssueTypes(ctx, db.IssueTypesParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, err
	}
	out := make([]Workflow, len(rows))
	for i, r := range rows {
		w := Workflow{
			ID: r.ID, ProjectID: r.ProjectID, Name: r.Name, CopiedFrom: r.CopiedFrom, Statuses: []Status{}, Transitions: []Transition{},
			DraftVersion: r.DraftVersion, DraftUpdatedBy: r.DraftUpdatedBy, DraftUpdatedAt: r.DraftUpdatedAt,
			PublishedAt: r.PublishedAt, Origin: origin(t, r.ProjectID, off[TypeWorkflow], r.ID), UsedBy: []uuid.UUID{}, Version: r.Version,
		}
		for _, st := range statuses {
			if st.WorkflowID == r.ID {
				w.Statuses = append(w.Statuses, Status{ID: st.ID, Name: st.Name, CategoryID: st.CategoryID, Initial: st.IsInitial})
			}
		}
		for _, tr := range transitions {
			if tr.WorkflowID == r.ID {
				w.Transitions = append(w.Transitions, Transition{ID: tr.ID, From: tr.FromStatusID, To: tr.ToStatusID, Name: tr.Name, Rules: rulesFrom(tr.Rules)})
			}
		}
		if r.Draft != nil {
			var d Draft
			if err := json.Unmarshal(r.Draft, &d); err != nil {
				return nil, fmt.Errorf("workflow %s: draft: %w", r.ID, err)
			}
			w.Draft = &d
		}
		for _, it := range types {
			if it.WorkflowID == r.ID && !off[TypeIssueType][it.ID] {
				w.UsedBy = append(w.UsedBy, it.ID)
			}
		}
		out[i] = w
	}
	return out, nil
}

// Workflows lists the workflows seen from the target.
func (s *Service) Workflows(ctx context.Context, t Target) ([]Workflow, error) {
	q := db.New(s.pool)
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	return s.workflows(ctx, q, t, off)
}

// Workflow returns one workflow seen from the target.
func (s *Service) Workflow(ctx context.Context, t Target, id uuid.UUID) (Workflow, error) {
	list, err := s.Workflows(ctx, t)
	if err != nil {
		return Workflow{}, err
	}
	for _, w := range list {
		if w.ID == id {
			return w, nil
		}
	}
	return Workflow{}, apperr.NotFound("workflow_not_found")
}

// CreateWorkflow makes a workflow in the target's scope, a published copy
// of copyFrom (a workflow the target sees). Copying an inherited workflow
// into a project doesn't switch the original off: the project's issue types
// move to the copy one by one.
func (s *Service) CreateWorkflow(ctx context.Context, t Target, wfName string, copyFrom uuid.UUID) (Workflow, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Workflow{}, err
	}
	var fields apperr.Fields
	n := name(&fields, "name", wfName, 60)
	if err := fields.Err(); err != nil {
		return Workflow{}, err
	}
	src, err := s.Workflow(ctx, t, copyFrom)
	if err != nil {
		return Workflow{}, apperr.Invalid(apperr.Field{Field: "copy_from", Code: "unknown"})
	}
	var id uuid.UUID
	err = inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		w, err := q.CreateWorkflow(ctx, db.CreateWorkflowParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: n,
			CopiedFrom: uuid.NullUUID{UUID: src.ID, Valid: true}, PublishedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		id = w.ID
		// New ids: statuses belong to exactly one workflow.
		newID := map[uuid.UUID]uuid.UUID{}
		d := Draft{}
		for _, st := range src.Statuses {
			newID[st.ID] = uuid.Must(uuid.NewV7())
			d.Statuses = append(d.Statuses, Status{ID: newID[st.ID], Name: st.Name, CategoryID: st.CategoryID, Initial: st.Initial})
		}
		for _, tr := range src.Transitions {
			from := tr.From
			if from.Valid {
				from.UUID = newID[from.UUID]
			}
			d.Transitions = append(d.Transitions, Transition{ID: uuid.Must(uuid.NewV7()), From: from, To: newID[tr.To], Name: tr.Name, Rules: tr.Rules})
		}
		if err := s.writePublished(ctx, tx, w.ID, d, nil); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeWorkflow, w.ID, int64(w.Version), "WorkflowCreated",
			map[string]any{"copied_from": src.ID}))
	})
	if err != nil {
		return Workflow{}, err
	}
	return s.Workflow(ctx, t, id)
}

func (s *Service) lockOwnWorkflow(ctx context.Context, q *db.Queries, t Target, id uuid.UUID) (db.Workflow, error) {
	w, err := q.LockWorkflow(ctx, db.LockWorkflowParams{WorkspaceID: t.WorkspaceID, ID: id})
	if err != nil {
		return w, notFound(err, "workflow_not_found")
	}
	if !sameScope(t, w.ProjectID) {
		return w, conflict("inherited_read_only")
	}
	return w, nil
}

// RenameWorkflow changes a workflow's name.
func (s *Service) RenameWorkflow(ctx context.Context, t Target, id uuid.UUID, newName string) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	var fields apperr.Fields
	n := name(&fields, "name", newName, 60)
	if err := fields.Err(); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := s.lockOwnWorkflow(ctx, q, t, id); err != nil {
			return err
		}
		w, err := q.RenameWorkflow(ctx, db.RenameWorkflowParams{WorkspaceID: t.WorkspaceID, ID: id, Name: n, UpdatedAt: s.now()})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeWorkflow, id, int64(w.Version), "WorkflowRenamed", nil))
	})
}

// DeleteWorkflow removes a workflow no issue type uses.
func (s *Service) DeleteWorkflow(ctx context.Context, t Target, id uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		w, err := s.lockOwnWorkflow(ctx, q, t, id)
		if err != nil {
			return err
		}
		n, err := q.WorkflowInUse(ctx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return conflict("workflow_in_use")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM config_disables WHERE definition_type = $1 AND definition_id = $2`, TypeWorkflow, id); err != nil {
			return err
		}
		if err := q.DeleteWorkflow(ctx, db.DeleteWorkflowParams{WorkspaceID: t.WorkspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeWorkflow, id, int64(w.Version)+1, "WorkflowDeleted", nil))
	})
}

// --- Drafts -------------------------------------------------------------------

// SaveDraft stores the shared draft (the whole document). version is the
// draft_version the editor started from; a newer one is a conflict.
func (s *Service) SaveDraft(ctx context.Context, t Target, id uuid.UUID, version int32, d Draft) (Workflow, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Workflow{}, err
	}
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		w, err := s.lockOwnWorkflow(ctx, q, t, id)
		if err != nil {
			return err
		}
		if w.DraftVersion != version {
			return conflict("version_conflict")
		}
		// Structure is checked now so the draft stays loadable; references
		// (categories, roles) are checked again on publishing.
		if err := s.validateDraft(ctx, tx, t, w, d); err != nil {
			return err
		}
		raw, err := json.Marshal(d)
		if err != nil {
			return err
		}
		now := s.now()
		_, err = q.SaveDraft(ctx, db.SaveDraftParams{WorkspaceID: t.WorkspaceID, ID: id, Draft: raw, DraftUpdatedBy: actor(t), DraftUpdatedAt: &now})
		return err
	})
	if err != nil {
		return Workflow{}, err
	}
	return s.Workflow(ctx, t, id)
}

// DiscardDraft drops the draft; the published workflow is untouched.
func (s *Service) DiscardDraft(ctx context.Context, t Target, id uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := s.lockOwnWorkflow(ctx, q, t, id); err != nil {
			return err
		}
		_, err := q.SaveDraft(ctx, db.SaveDraftParams{WorkspaceID: t.WorkspaceID, ID: id, Draft: nil})
		return err
	})
}

// validateDraft checks a draft completely: names, one initial status in an
// open category, categories the workflow's scope sees, transitions between
// its statuses, and rules naming known roles and fields.
func (s *Service) validateDraft(ctx context.Context, tx pgx.Tx, t Target, w db.Workflow, d Draft) error {
	var fields apperr.Fields
	r, err := s.Resolve(ctx, tx, t.WorkspaceID, w.ProjectID)
	if err != nil {
		return err
	}
	switch {
	case len(d.Statuses) == 0:
		fields.Add("statuses", "required")
	case len(d.Statuses) > maxStatuses:
		fields.Add("statuses", "too_many")
	}
	if len(d.Transitions) > maxTransitions {
		fields.Add("transitions", "too_many")
	}
	ids := map[uuid.UUID]bool{}
	names := map[string]bool{}
	initials := 0
	for i, st := range d.Statuses {
		at := fmt.Sprintf("statuses[%d]", i)
		if st.ID == uuid.Nil || ids[st.ID] {
			fields.Add(at+".id", "invalid")
		}
		ids[st.ID] = true
		n := name(&fields, at+".name", st.Name, 40)
		if names[strings.ToLower(n)] {
			fields.Add(at+".name", "taken")
		}
		names[strings.ToLower(n)] = true
		cat, ok := r.Categories[st.CategoryID]
		if !ok {
			fields.Add(at+".category_id", "unknown")
		}
		if st.Initial {
			initials++
			if ok && !Open(cat.Outcome) {
				fields.Add(at+".initial", "final_category")
			}
		}
	}
	if len(d.Statuses) > 0 && initials != 1 {
		fields.Add("statuses", "one_initial")
	}
	roles, err := s.access.Roles(ctx, t.WorkspaceID)
	if err != nil {
		return err
	}
	pairs := map[[2]uuid.UUID]bool{}
	tids := map[uuid.UUID]bool{}
	for i, tr := range d.Transitions {
		at := fmt.Sprintf("transitions[%d]", i)
		if tr.ID == uuid.Nil || tids[tr.ID] {
			fields.Add(at+".id", "invalid")
		}
		tids[tr.ID] = true
		name(&fields, at+".name", tr.Name, 40)
		if !ids[tr.To] {
			fields.Add(at+".to", "unknown")
		}
		if tr.From.Valid && (!ids[tr.From.UUID] || tr.From.UUID == tr.To) {
			fields.Add(at+".from", "invalid")
		}
		pair := [2]uuid.UUID{tr.From.UUID, tr.To}
		if pairs[pair] {
			fields.Add(at, "duplicate")
		}
		pairs[pair] = true
		for _, role := range tr.Rules.Roles {
			if !slices.ContainsFunc(roles, func(r2 access.Role) bool { return r2.ID == role }) {
				fields.Add(at+".rules.roles", "unknown")
			}
		}
		for _, f := range tr.Rules.Validators {
			if !slices.Contains(BuiltinFields, f) {
				fields.Add(at+".rules.validators", "unknown")
			}
		}
		for _, pa := range tr.Rules.PostActions {
			if !slices.Contains(BuiltinFields, pa.Field) || (pa.Op != "clear" && !(pa.Op == "set_actor" && pa.Field == "assignee")) {
				fields.Add(at+".rules.post_actions", "invalid")
			}
		}
	}
	if err := fields.Err(); err != nil {
		return err
	}
	// Ids of statuses and transitions are global: a draft may only reuse
	// this workflow's own.
	all := make([]uuid.UUID, 0, len(ids)+len(tids))
	for id := range ids {
		all = append(all, id)
	}
	for id := range tids {
		all = append(all, id)
	}
	var foreign int
	err = tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM statuses WHERE id = ANY($1) AND workflow_id <> $2)
		     + (SELECT count(*) FROM transitions WHERE id = ANY($1) AND workflow_id <> $2)`, all, w.ID).Scan(&foreign)
	if err != nil {
		return err
	}
	if foreign > 0 {
		return apperr.Invalid(apperr.Field{Field: "statuses", Code: "foreign_id"})
	}
	return nil
}

// PublishResult describes a publication, or why it can't happen yet.
type PublishResult struct {
	Added   []uuid.UUID `json:"added"`
	Removed []uuid.UUID `json:"removed"`
	// Moved counts issues moved from each removed status.
	Moved map[uuid.UUID]int64 `json:"moved"`
}

// Publish applies the draft (docs/configuration.md#editing-drafts-and-publishing).
// Every removed status that still holds issues needs a mapping to a status
// of the draft; until then publishing answers 409 unmapped_statuses, one
// field per status (status_mapping.<id>) with its issue count in Detail.
func (s *Service) Publish(ctx context.Context, t Target, id uuid.UUID, version int32, mapping map[uuid.UUID]uuid.UUID) (PublishResult, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return PublishResult{}, err
	}
	var res PublishResult
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		w, err := s.lockOwnWorkflow(ctx, q, t, id)
		if err != nil {
			return err
		}
		if w.Draft == nil {
			return conflict("no_draft")
		}
		if version != 0 && w.DraftVersion != version {
			return conflict("version_conflict")
		}
		var d Draft
		if err := json.Unmarshal(w.Draft, &d); err != nil {
			return err
		}
		if err := s.validateDraft(ctx, tx, t, w, d); err != nil {
			return err
		}
		published, err := q.Statuses(ctx, []uuid.UUID{w.ID})
		if err != nil {
			return err
		}
		inDraft := map[uuid.UUID]bool{}
		for _, st := range d.Statuses {
			inDraft[st.ID] = true
		}
		inPublished := map[uuid.UUID]bool{}
		for _, st := range published {
			inPublished[st.ID] = true
			if !inDraft[st.ID] {
				res.Removed = append(res.Removed, st.ID)
			}
		}
		for _, st := range d.Statuses {
			if !inPublished[st.ID] {
				res.Added = append(res.Added, st.ID)
			}
		}
		counts, err := s.issues.CountByStatus(ctx, tx, res.Removed)
		if err != nil {
			return err
		}
		res.Moved = map[uuid.UUID]int64{}
		moves := map[uuid.UUID]uuid.UUID{}
		var unmapped apperr.Fields
		for _, removed := range res.Removed {
			n := counts[removed]
			if n == 0 {
				continue
			}
			to, ok := mapping[removed]
			if !ok || !inDraft[to] {
				unmapped.Add("status_mapping."+removed.String(), fmt.Sprintf("required:%d", n))
				continue
			}
			moves[removed] = to
			res.Moved[removed] = n
		}
		if len(unmapped) > 0 {
			return &apperr.Error{Status: 409, Code: "unmapped_statuses", Fields: unmapped,
				Detail: "map every removed status that holds issues to a status of the draft; the code after required: is the issue count"}
		}
		if err := s.writePublished(ctx, tx, w.ID, d, func() error {
			return s.issues.MoveStatuses(ctx, tx, actor(t), moves)
		}); err != nil {
			return err
		}
		w, err = q.MarkPublished(ctx, db.MarkPublishedParams{WorkspaceID: t.WorkspaceID, ID: w.ID, PublishedAt: s.now()})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeWorkflow, w.ID, int64(w.Version), "WorkflowPublished",
			map[string]any{"added": res.Added, "removed": res.Removed, "mapping": moves}))
	})
	return res, err
}

// writePublished replaces a workflow's published statuses and transitions
// with d. beforeDelete runs once the new statuses exist and before removed
// ones go: it moves the issues out of them.
func (s *Service) writePublished(ctx context.Context, tx pgx.Tx, workflowID uuid.UUID, d Draft, beforeDelete func() error) error {
	q := db.New(tx)
	keep := make([]uuid.UUID, len(d.Statuses))
	for i, st := range d.Statuses {
		keep[i] = st.ID
		err := q.UpsertStatus(ctx, db.UpsertStatusParams{WorkflowID: workflowID, ID: st.ID, Name: strings.TrimSpace(st.Name), CategoryID: st.CategoryID, Position: int32(i), IsInitial: st.Initial})
		if err != nil {
			return err
		}
	}
	if beforeDelete != nil {
		if err := beforeDelete(); err != nil {
			return err
		}
	}
	if err := q.DeleteTransitions(ctx, workflowID); err != nil {
		return err
	}
	if err := q.DeleteStatusesExcept(ctx, db.DeleteStatusesExceptParams{WorkflowID: workflowID, Keep: keep}); err != nil {
		return err
	}
	for _, tr := range d.Transitions {
		rules, _ := json.Marshal(tr.Rules)
		err := q.CreateTransition(ctx, db.CreateTransitionParams{
			WorkflowID: workflowID, ID: tr.ID, FromStatusID: tr.From, ToStatusID: tr.To, Name: strings.TrimSpace(tr.Name), Rules: rules,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
