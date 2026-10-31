package configdef

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// IssueTypes lists the issue types seen from the target: inherited ones
// (maybe switched off) first, then the project's own.
func (s *Service) IssueTypes(ctx context.Context, t Target) ([]IssueType, error) {
	q := db.New(s.pool)
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	rows, err := q.IssueTypes(ctx, db.IssueTypesParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, err
	}
	out := make([]IssueType, len(rows))
	for i, r := range rows {
		out[i] = issueTypeFrom(r)
		out[i].Origin = origin(t, r.ProjectID, off[TypeIssueType], r.ID)
	}
	return out, nil
}

// IssueTypeInput creates or changes an issue type; nil fields are unchanged.
type IssueTypeInput struct {
	Name          *string
	Icon          *string
	Colour        *string
	WorkflowID    *uuid.UUID
	ParentRule    *string
	ParentTypeIDs *[]uuid.UUID
}

// validateType checks values against what the target sees.
func (s *Service) validateType(ctx context.Context, tx pgx.Tx, t Target, v *db.IssueType, in IssueTypeInput) error {
	var fields apperr.Fields
	if in.Name != nil {
		v.Name = name(&fields, "name", *in.Name, 40)
	}
	if in.Icon != nil {
		v.Icon = *in.Icon
	}
	if !slices.Contains(Icons, v.Icon) {
		fields.Add("icon", "invalid")
	}
	if in.Colour != nil {
		v.Colour = *in.Colour
	}
	colour(&fields, v.Colour)
	if in.ParentRule != nil {
		v.ParentRule = *in.ParentRule
	}
	if v.ParentRule != ParentAny && v.ParentRule != ParentNone && v.ParentRule != ParentRequired {
		fields.Add("parent_rule", "invalid")
	}
	r, err := s.Resolve(ctx, tx, t.WorkspaceID, t.ProjectID)
	if err != nil {
		return err
	}
	if in.WorkflowID != nil {
		v.WorkflowID = *in.WorkflowID
	}
	if _, ok := r.Workflows[v.WorkflowID]; !ok {
		fields.Add("workflow_id", "unknown")
	}
	if in.ParentTypeIDs != nil {
		v.AllowedParentTypeIds = []uuid.UUID{}
		for _, id := range *in.ParentTypeIDs {
			if !slices.ContainsFunc(r.IssueTypes, func(it IssueType) bool { return it.ID == id }) {
				fields.Add("parent_type_ids", "unknown")
				break
			}
			if !slices.Contains(v.AllowedParentTypeIds, id) {
				v.AllowedParentTypeIds = append(v.AllowedParentTypeIds, id)
			}
		}
	}
	if v.ParentRule == ParentNone {
		v.AllowedParentTypeIds = []uuid.UUID{}
	}
	return fields.Err()
}

// CreateIssueType adds an issue type in the target's scope.
func (s *Service) CreateIssueType(ctx context.Context, t Target, in IssueTypeInput) (IssueType, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return IssueType{}, err
	}
	var created db.IssueType
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		v := db.IssueType{Icon: "check", Colour: "blue", ParentRule: ParentAny, AllowedParentTypeIds: []uuid.UUID{}}
		if in.Name == nil {
			in.Name = new(string)
		}
		if err := s.validateType(ctx, tx, t, &v, in); err != nil {
			return err
		}
		pos, err := q.NextIssueTypePosition(ctx, db.NextIssueTypePositionParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
		if err != nil {
			return err
		}
		created, err = q.CreateIssueType(ctx, db.CreateIssueTypeParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: v.Name, Icon: v.Icon,
			Colour: v.Colour, WorkflowID: v.WorkflowID, ParentRule: v.ParentRule, AllowedParentTypeIds: v.AllowedParentTypeIds,
			Position: pos, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeIssueType, created.ID, int64(created.Version), "IssueTypeCreated",
			map[string]any{"workflow_id": created.WorkflowID}))
	})
	if err != nil {
		return IssueType{}, err
	}
	return issueTypeFrom(created), nil
}

func (s *Service) lockOwnType(ctx context.Context, q *db.Queries, t Target, id uuid.UUID) (db.IssueType, error) {
	it, err := q.LockIssueType(ctx, db.LockIssueTypeParams{WorkspaceID: t.WorkspaceID, ID: id})
	if err != nil {
		return it, notFound(err, "issue_type_not_found")
	}
	if !sameScope(t, it.ProjectID) {
		return it, conflict("inherited_read_only")
	}
	return it, nil
}

// UpdateIssueType edits an issue type. Moving it to another workflow needs
// statusMapping for every status of the old workflow that holds issues of
// the type (409 unmapped_statuses otherwise), as publishing does.
func (s *Service) UpdateIssueType(ctx context.Context, t Target, id uuid.UUID, version int32, in IssueTypeInput, statusMapping map[uuid.UUID]uuid.UUID) (IssueType, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return IssueType{}, err
	}
	var updated db.IssueType
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.lockOwnType(ctx, q, t, id)
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return conflict("version_conflict")
		}
		v := cur
		if err := s.validateType(ctx, tx, t, &v, in); err != nil {
			return err
		}
		if v.WorkflowID != cur.WorkflowID {
			if err := s.moveTypeWorkflow(ctx, tx, t, cur, v.WorkflowID, statusMapping); err != nil {
				return err
			}
		}
		updated, err = q.UpdateIssueType(ctx, db.UpdateIssueTypeParams{
			WorkspaceID: cur.WorkspaceID, ID: cur.ID, Name: v.Name, Icon: v.Icon, Colour: v.Colour, WorkflowID: v.WorkflowID,
			ParentRule: v.ParentRule, AllowedParentTypeIds: v.AllowedParentTypeIds, UpdatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeIssueType, updated.ID, int64(updated.Version), "IssueTypeUpdated",
			map[string]any{"workflow_id": updated.WorkflowID}))
	})
	if err != nil {
		return IssueType{}, err
	}
	return issueTypeFrom(updated), nil
}

func (s *Service) moveTypeWorkflow(ctx context.Context, tx pgx.Tx, t Target, cur db.IssueType, to uuid.UUID, mapping map[uuid.UUID]uuid.UUID) error {
	counts, err := s.issues.CountTypeByStatus(ctx, tx, cur.ID)
	if err != nil {
		return err
	}
	target, err := db.New(tx).Statuses(ctx, []uuid.UUID{to})
	if err != nil {
		return err
	}
	inTarget := map[uuid.UUID]bool{}
	for _, st := range target {
		inTarget[st.ID] = true
	}
	moves := map[uuid.UUID]uuid.UUID{}
	var unmapped apperr.Fields
	for status, n := range counts {
		dest, ok := mapping[status]
		if !ok || !inTarget[dest] {
			unmapped.Add("status_mapping."+status.String(), fmt.Sprintf("required:%d", n))
			continue
		}
		moves[status] = dest
	}
	if len(unmapped) > 0 {
		return &apperr.Error{Status: 409, Code: "unmapped_statuses", Fields: unmapped,
			Detail: "map every status holding issues of this type to a status of the new workflow"}
	}
	return s.issues.MoveTypeStatuses(ctx, tx, actor(t), cur.ID, moves)
}

// DeleteIssueType removes an issue type no issue has.
func (s *Service) DeleteIssueType(ctx context.Context, t Target, id uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		it, err := s.lockOwnType(ctx, q, t, id)
		if err != nil {
			return err
		}
		n, err := s.issues.CountByType(ctx, tx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return conflict("issue_type_in_use")
		}
		if _, err := tx.Exec(ctx, `UPDATE issue_types SET allowed_parent_type_ids = array_remove(allowed_parent_type_ids, $1) WHERE workspace_id = $2`, id, t.WorkspaceID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM config_disables WHERE definition_type = $1 AND definition_id = $2`, TypeIssueType, id); err != nil {
			return err
		}
		if err := q.DeleteIssueType(ctx, db.DeleteIssueTypeParams{WorkspaceID: t.WorkspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeIssueType, id, int64(it.Version)+1, "IssueTypeDeleted", nil))
	})
}

// CopyIssueType copies an inherited issue type into the target project and
// switches the original off there (ADR-0011): the copy is then editable.
func (s *Service) CopyIssueType(ctx context.Context, t Target, id uuid.UUID) (IssueType, error) {
	if !t.ProjectID.Valid {
		return IssueType{}, conflict("not_inherited")
	}
	if err := s.requireManage(ctx, t); err != nil {
		return IssueType{}, err
	}
	var created db.IssueType
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		src, err := q.LockIssueType(ctx, db.LockIssueTypeParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "issue_type_not_found")
		}
		if src.ProjectID.Valid {
			return conflict("not_inherited")
		}
		pos, err := q.NextIssueTypePosition(ctx, db.NextIssueTypePositionParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
		if err != nil {
			return err
		}
		created, err = q.CreateIssueType(ctx, db.CreateIssueTypeParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: src.Name, Icon: src.Icon,
			Colour: src.Colour, WorkflowID: src.WorkflowID, ParentRule: src.ParentRule, AllowedParentTypeIds: src.AllowedParentTypeIds,
			Position: pos, CopiedFrom: uuid.NullUUID{UUID: src.ID, Valid: true}, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		if err := q.Disable(ctx, db.DisableParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID.UUID, DefinitionType: TypeIssueType, DefinitionID: src.ID}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeIssueType, created.ID, int64(created.Version), "IssueTypeCreated",
			map[string]any{"copied_from": src.ID, "workflow_id": created.WorkflowID}))
	})
	if err != nil {
		return IssueType{}, err
	}
	return issueTypeFrom(created), nil
}

// ReorderIssueTypes sets the order of the target's own issue types.
func (s *Service) ReorderIssueTypes(ctx context.Context, t Target, ids []uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		for i, id := range ids {
			if _, err := s.lockOwnType(ctx, q, t, id); err != nil {
				return err
			}
			if err := q.SetIssueTypePosition(ctx, db.SetIssueTypePositionParams{WorkspaceID: t.WorkspaceID, ID: id, Position: int32(i)}); err != nil {
				return err
			}
		}
		return nil
	})
}
