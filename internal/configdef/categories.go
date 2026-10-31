package configdef

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Categories lists the categories seen from the target, in board order:
// the workspace's (inherited, maybe switched off) then the project's own.
func (s *Service) Categories(ctx context.Context, t Target) ([]Category, error) {
	q := db.New(s.pool)
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	rows, err := q.Categories(ctx, db.CategoriesParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, err
	}
	usage, err := q.CategoryUsage(ctx, t.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		c := categoryFrom(r)
		c.Origin = origin(t, r.ProjectID, off[TypeCategory], r.ID)
		workflows := map[uuid.UUID]bool{}
		seen := map[string]bool{}
		for _, u := range usage {
			if u.CategoryID != r.ID {
				continue
			}
			workflows[u.WorkflowID] = true
			if !seen[u.StatusName] {
				seen[u.StatusName] = true
				c.Statuses = append(c.Statuses, u.StatusName)
			}
		}
		c.Workflows = len(workflows)
		out[i] = c
	}
	return out, nil
}

// CategoryInput creates or changes a category; nil fields are unchanged.
type CategoryInput struct {
	Name    *string
	Colour  *string
	Outcome *string
}

// CreateCategory adds a category in the target's scope.
func (s *Service) CreateCategory(ctx context.Context, t Target, in CategoryInput) (Category, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Category{}, err
	}
	var fields apperr.Fields
	n := name(&fields, "name", deref(in.Name), 40)
	col := "grey"
	if in.Colour != nil {
		col = *in.Colour
		colour(&fields, col)
	}
	if in.Outcome == nil || !validOutcome(*in.Outcome) {
		fields.Add("outcome", "invalid")
	}
	if err := fields.Err(); err != nil {
		return Category{}, err
	}
	var c db.StatusCategory
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		pos, err := q.NextCategoryPosition(ctx, db.NextCategoryPositionParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
		if err != nil {
			return err
		}
		c, err = q.CreateCategory(ctx, db.CreateCategoryParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: n, Colour: col,
			Outcome: *in.Outcome, Position: pos, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCategory, c.ID, int64(c.Version), "StatusCategoryCreated",
			map[string]any{"outcome": c.Outcome}))
	})
	if err != nil {
		return Category{}, err
	}
	return categoryFrom(c), nil
}

// lockOwn loads a category of the target's own scope for an edit.
func (s *Service) lockOwnCategory(ctx context.Context, q *db.Queries, t Target, id uuid.UUID) (db.StatusCategory, error) {
	c, err := q.LockCategory(ctx, db.LockCategoryParams{WorkspaceID: t.WorkspaceID, ID: id})
	if err != nil {
		return c, notFound(err, "category_not_found")
	}
	if !sameScope(t, c.ProjectID) {
		return c, conflict("inherited_read_only")
	}
	return c, nil
}

// UpdateCategory renames, recolours or changes the outcome of a category. A
// category holding a workflow's initial status stays open.
func (s *Service) UpdateCategory(ctx context.Context, t Target, id uuid.UUID, version int32, in CategoryInput) (Category, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Category{}, err
	}
	var c db.StatusCategory
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.lockOwnCategory(ctx, q, t, id)
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return conflict("version_conflict")
		}
		p := db.UpdateCategoryParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Name: cur.Name, Colour: cur.Colour, Outcome: cur.Outcome, UpdatedAt: s.now()}
		var fields apperr.Fields
		if in.Name != nil {
			p.Name = name(&fields, "name", *in.Name, 40)
		}
		if in.Colour != nil {
			p.Colour = *in.Colour
			colour(&fields, p.Colour)
		}
		if in.Outcome != nil {
			if !validOutcome(*in.Outcome) {
				fields.Add("outcome", "invalid")
			}
			p.Outcome = *in.Outcome
		}
		if err := fields.Err(); err != nil {
			return err
		}
		if Open(cur.Outcome) && !Open(p.Outcome) {
			if holds, err := s.holdsInitial(ctx, tx, t.WorkspaceID, cur.ID); err != nil {
				return err
			} else if holds {
				return conflict("initial_status_in_category")
			}
		}
		if p.Name == cur.Name && p.Colour == cur.Colour && p.Outcome == cur.Outcome {
			c = cur
			return nil
		}
		c, err = q.UpdateCategory(ctx, p)
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCategory, c.ID, int64(c.Version), "StatusCategoryUpdated",
			map[string]any{"outcome": c.Outcome}))
	})
	if err != nil {
		return Category{}, err
	}
	return categoryFrom(c), nil
}

// holdsInitial reports whether a published or draft initial status is in
// the category.
func (s *Service) holdsInitial(ctx context.Context, tx pgx.Tx, workspaceID, categoryID uuid.UUID) (bool, error) {
	var n int
	err := tx.QueryRow(ctx, `
		SELECT count(*) FROM statuses s JOIN workflows w ON w.id = s.workflow_id
		WHERE w.workspace_id = $1 AND s.category_id = $2 AND s.is_initial`, workspaceID, categoryID).Scan(&n)
	if err != nil || n > 0 {
		return n > 0, err
	}
	drafts, err := db.New(tx).WorkflowsWithDrafts(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	for _, w := range drafts {
		var d Draft
		if json.Unmarshal(w.Draft, &d) == nil {
			for _, st := range d.Statuses {
				if st.Initial && st.CategoryID == categoryID {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// ReorderCategories sets the board order of the target's own categories.
func (s *Service) ReorderCategories(ctx context.Context, t Target, ids []uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		for i, id := range ids {
			if _, err := s.lockOwnCategory(ctx, q, t, id); err != nil {
				return err
			}
			if err := q.SetCategoryPosition(ctx, db.SetCategoryPositionParams{WorkspaceID: t.WorkspaceID, ID: id, Position: int32(i)}); err != nil {
				return err
			}
		}
		return eventlog.Append(ctx, tx, s.event(t, "workspace", t.WorkspaceID, 0, "StatusCategoriesReordered",
			map[string]any{"order": ids}))
	})
}

// DeleteCategory removes a category. If statuses use it, replacement — a
// category with the same kind of outcome (open or final), visible wherever
// the deleted one was — takes them over in every workflow, drafts included,
// in the same step.
func (s *Service) DeleteCategory(ctx context.Context, t Target, id uuid.UUID, replacement uuid.NullUUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.lockOwnCategory(ctx, q, t, id)
		if err != nil {
			return err
		}
		inUse, err := s.categoryInUse(ctx, tx, t.WorkspaceID, id)
		if err != nil {
			return err
		}
		if inUse {
			if !replacement.Valid {
				return conflict("replacement_required")
			}
			rep, err := q.LockCategory(ctx, db.LockCategoryParams{WorkspaceID: t.WorkspaceID, ID: replacement.UUID})
			if err != nil || rep.ID == cur.ID {
				return apperr.Invalid(apperr.Field{Field: "replacement", Code: "unknown"})
			}
			// A workspace category's statuses may be in any project's
			// workflows: only a workspace category reaches them all.
			if Open(rep.Outcome) != Open(cur.Outcome) || (rep.ProjectID.Valid && rep.ProjectID != cur.ProjectID) {
				return apperr.Invalid(apperr.Field{Field: "replacement", Code: "incompatible"})
			}
			if _, err := q.MoveStatusesToCategory(ctx, db.MoveStatusesToCategoryParams{ToID: rep.ID, FromID: cur.ID, WorkspaceID: t.WorkspaceID}); err != nil {
				return err
			}
			if err := s.replaceInDrafts(ctx, tx, t.WorkspaceID, cur.ID, rep.ID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM config_disables WHERE definition_type = $1 AND definition_id = $2`, TypeCategory, id); err != nil {
			return err
		}
		if err := q.DeleteCategory(ctx, db.DeleteCategoryParams{WorkspaceID: t.WorkspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCategory, id, int64(cur.Version)+1, "StatusCategoryDeleted",
			map[string]any{"replacement": replacement}))
	})
}

func (s *Service) categoryInUse(ctx context.Context, tx pgx.Tx, workspaceID, id uuid.UUID) (bool, error) {
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM statuses WHERE category_id = $1`, id).Scan(&n); err != nil || n > 0 {
		return n > 0, err
	}
	drafts, err := db.New(tx).WorkflowsWithDrafts(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	for _, w := range drafts {
		var d Draft
		if json.Unmarshal(w.Draft, &d) == nil {
			for _, st := range d.Statuses {
				if st.CategoryID == id {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func (s *Service) replaceInDrafts(ctx context.Context, tx pgx.Tx, workspaceID, from, to uuid.UUID) error {
	q := db.New(tx)
	drafts, err := q.WorkflowsWithDrafts(ctx, workspaceID)
	if err != nil {
		return err
	}
	for _, w := range drafts {
		var d Draft
		if err := json.Unmarshal(w.Draft, &d); err != nil {
			return err
		}
		changed := false
		for i := range d.Statuses {
			if d.Statuses[i].CategoryID == from {
				d.Statuses[i].CategoryID = to
				changed = true
			}
		}
		if !changed {
			continue
		}
		raw, _ := json.Marshal(d)
		if _, err := q.SaveDraft(ctx, db.SaveDraftParams{WorkspaceID: w.WorkspaceID, ID: w.ID, Draft: raw, DraftUpdatedBy: w.DraftUpdatedBy, DraftUpdatedAt: w.DraftUpdatedAt}); err != nil {
			return err
		}
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
