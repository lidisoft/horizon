package configdef

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Limits of custom fields.
const (
	maxOptions      = 100
	maxTypeFields   = 50
	maxPrecision    = 6
	maxFieldsInType = maxTypeFields
)

func fieldFrom(f db.CustomField) Field {
	var opts FieldOptions
	_ = json.Unmarshal(f.Options, &opts)
	return Field{
		ID: f.ID, ProjectID: f.ProjectID, Name: f.Name, Type: f.Type, Description: f.Description,
		FieldOptions: opts, Position: f.Position, UsedBy: []uuid.UUID{}, Version: f.Version,
	}
}

// fields loads the fields seen from t, with origins and the types each is
// attached to, and every attachment of the workspace.
func (s *Service) fields(ctx context.Context, q *db.Queries, t Target, off map[string]map[uuid.UUID]bool) ([]Field, []db.IssueTypeField, error) {
	rows, err := q.Fields(ctx, db.FieldsParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, nil, err
	}
	attached, err := q.TypeFields(ctx, t.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	out := make([]Field, len(rows))
	for i, r := range rows {
		out[i] = fieldFrom(r)
		out[i].Origin = origin(t, r.ProjectID, off[TypeCustomField], r.ID)
		for _, a := range attached {
			if a.FieldID == r.ID && !off[TypeIssueType][a.IssueTypeID] {
				out[i].UsedBy = append(out[i].UsedBy, a.IssueTypeID)
			}
		}
	}
	return out, attached, nil
}

// Fields lists the custom fields seen from the target.
func (s *Service) Fields(ctx context.Context, t Target) ([]Field, error) {
	q := db.New(s.pool)
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	fields, _, err := s.fields(ctx, q, t, off)
	return fields, err
}

// FieldInput creates or changes a field; nil fields are unchanged. Type is
// only read on creation: a field's type is fixed.
type FieldInput struct {
	Name        *string
	Type        *string
	Description *string
	Options     *[]Option
	Unit        *string
	Precision   *int
}

func validateField(fieldType string, in FieldInput, cur FieldOptions, fields *apperr.Fields) FieldOptions {
	opts := cur
	if in.Options != nil {
		if fieldType != "select" && fieldType != "multi_select" {
			fields.Add("options", "not_select")
		}
		if len(*in.Options) > maxOptions {
			fields.Add("options", "too_many")
		}
		seen := map[string]bool{}
		opts.Options = []Option{}
		for i, o := range *in.Options {
			at := fmt.Sprintf("options[%d]", i)
			o.Name = strings.TrimSpace(o.Name)
			if o.Name == "" || len([]rune(o.Name)) > 60 {
				fields.Add(at+".name", "invalid")
			}
			if seen[strings.ToLower(o.Name)] {
				fields.Add(at+".name", "taken")
			}
			seen[strings.ToLower(o.Name)] = true
			if o.Colour == "" {
				o.Colour = "grey"
			}
			if !slices.Contains(Colours, o.Colour) {
				fields.Add(at+".colour", "invalid")
			}
			if o.ID == uuid.Nil {
				o.ID = uuid.Must(uuid.NewV7())
			}
			opts.Options = append(opts.Options, o)
		}
	}
	if in.Unit != nil || in.Precision != nil {
		if fieldType != "number" {
			fields.Add("unit", "not_number")
		}
		if in.Unit != nil {
			opts.Unit = strings.TrimSpace(*in.Unit)
			if len([]rune(opts.Unit)) > 10 {
				fields.Add("unit", "too_long")
			}
		}
		if in.Precision != nil {
			opts.Precision = *in.Precision
			if opts.Precision < 0 || opts.Precision > maxPrecision {
				fields.Add("precision", "invalid")
			}
		}
	}
	return opts
}

// CreateField adds a custom field in the target's scope.
func (s *Service) CreateField(ctx context.Context, t Target, in FieldInput) (Field, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Field{}, err
	}
	var fields apperr.Fields
	n := name(&fields, "name", deref(in.Name), 60)
	ft := deref(in.Type)
	if !slices.Contains(FieldTypes, ft) {
		fields.Add("type", "invalid")
	}
	opts := validateField(ft, in, FieldOptions{}, &fields)
	desc := strings.TrimSpace(deref(in.Description))
	if len([]rune(desc)) > 200 {
		fields.Add("description", "too_long")
	}
	if err := fields.Err(); err != nil {
		return Field{}, err
	}
	raw, _ := json.Marshal(opts)
	var f db.CustomField
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		pos, err := q.NextFieldPosition(ctx, db.NextFieldPositionParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
		if err != nil {
			return err
		}
		f, err = q.CreateField(ctx, db.CreateFieldParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: n, Type: ft,
			Description: desc, Options: raw, Position: pos, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCustomField, f.ID, int64(f.Version), "CustomFieldCreated", map[string]any{"type": ft}))
	})
	if err != nil {
		return Field{}, err
	}
	return fieldFrom(f), nil
}

// UpdateField renames a field or changes its description and options.
// Removing a select option leaves issue values pointing at it; they show
// as unknown until changed.
func (s *Service) UpdateField(ctx context.Context, t Target, id uuid.UUID, version int32, in FieldInput) (Field, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Field{}, err
	}
	var f db.CustomField
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := q.LockField(ctx, db.LockFieldParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "field_not_found")
		}
		if !sameScope(t, cur.ProjectID) {
			return conflict("inherited_read_only")
		}
		if version != 0 && cur.Version != version {
			return conflict("version_conflict")
		}
		var fields apperr.Fields
		p := db.UpdateFieldParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Name: cur.Name, Description: cur.Description, UpdatedAt: s.now()}
		if in.Name != nil {
			p.Name = name(&fields, "name", *in.Name, 60)
		}
		if in.Description != nil {
			p.Description = strings.TrimSpace(*in.Description)
			if len([]rune(p.Description)) > 200 {
				fields.Add("description", "too_long")
			}
		}
		if in.Type != nil && *in.Type != cur.Type {
			fields.Add("type", "fixed")
		}
		opts := validateField(cur.Type, in, fieldFrom(cur).FieldOptions, &fields)
		if err := fields.Err(); err != nil {
			return err
		}
		p.Options, _ = json.Marshal(opts)
		f, err = q.UpdateField(ctx, p)
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCustomField, f.ID, int64(f.Version), "CustomFieldUpdated", nil))
	})
	if err != nil {
		return Field{}, err
	}
	return fieldFrom(f), nil
}

// DeleteField removes a field, detaching it from every issue type and
// dropping it from workflow rules. Issue values are scrubbed by the issue
// module.
func (s *Service) DeleteField(ctx context.Context, t Target, id uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		f, err := q.LockField(ctx, db.LockFieldParams{WorkspaceID: t.WorkspaceID, ID: id})
		if err != nil {
			return notFound(err, "field_not_found")
		}
		if !sameScope(t, f.ProjectID) {
			return conflict("inherited_read_only")
		}
		if err := s.dropFieldFromRules(ctx, tx, t.WorkspaceID, id.String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM config_disables WHERE definition_type = $1 AND definition_id = $2`, TypeCustomField, id); err != nil {
			return err
		}
		if err := q.DeleteField(ctx, db.DeleteFieldParams{WorkspaceID: t.WorkspaceID, ID: id}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeCustomField, id, int64(f.Version)+1, "CustomFieldDeleted", nil))
	})
}

// dropFieldFromRules removes a field from the validators and post-actions
// of published transitions and drafts.
func (s *Service) dropFieldFromRules(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, field string) error {
	clean := func(r *Rules) bool {
		before := len(r.Validators) + len(r.PostActions)
		r.Validators = slices.DeleteFunc(r.Validators, func(v string) bool { return v == field })
		r.PostActions = slices.DeleteFunc(r.PostActions, func(p PostAction) bool { return p.Field == field })
		return len(r.Validators)+len(r.PostActions) != before
	}
	rows, err := tx.Query(ctx, `SELECT t.id, t.rules FROM transitions t JOIN workflows w ON w.id = t.workflow_id WHERE w.workspace_id = $1`, workspaceID)
	if err != nil {
		return err
	}
	type update struct {
		id    uuid.UUID
		rules []byte
	}
	var updates []update
	for rows.Next() {
		var id uuid.UUID
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			rows.Close()
			return err
		}
		r := rulesFrom(raw)
		if clean(&r) {
			b, _ := json.Marshal(r)
			updates = append(updates, update{id, b})
		}
	}
	rows.Close()
	for _, u := range updates {
		if _, err := tx.Exec(ctx, `UPDATE transitions SET rules = $2 WHERE id = $1`, u.id, u.rules); err != nil {
			return err
		}
	}
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
		for i := range d.Transitions {
			changed = clean(&d.Transitions[i].Rules) || changed
		}
		if changed {
			raw, _ := json.Marshal(d)
			if _, err := q.SaveDraft(ctx, db.SaveDraftParams{WorkspaceID: w.WorkspaceID, ID: w.ID, Draft: raw, DraftUpdatedBy: w.DraftUpdatedBy, DraftUpdatedAt: w.DraftUpdatedAt}); err != nil {
				return err
			}
		}
	}
	return nil
}

// TypeFields returns the fields attached to an issue type, in order.
func (s *Service) TypeFields(ctx context.Context, t Target, typeID uuid.UUID) ([]TypeField, error) {
	rows, err := db.New(s.pool).TypeFields(ctx, t.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := []TypeField{}
	for _, r := range rows {
		if r.IssueTypeID == typeID {
			out = append(out, TypeField{FieldID: r.FieldID, Required: r.Required})
		}
	}
	return out, nil
}

// SetTypeFields replaces the fields of one of the target's own issue types:
// fields the type's scope sees, in display order. Required fields are
// checked when an issue is created.
func (s *Service) SetTypeFields(ctx context.Context, t Target, typeID uuid.UUID, list []TypeField) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	if len(list) > maxFieldsInType {
		return apperr.Invalid(apperr.Field{Field: "fields", Code: "too_many"})
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		it, err := s.lockOwnType(ctx, q, t, typeID)
		if err != nil {
			return err
		}
		r, err := s.Resolve(ctx, tx, t.WorkspaceID, it.ProjectID)
		if err != nil {
			return err
		}
		var fields apperr.Fields
		seen := map[uuid.UUID]bool{}
		for i, f := range list {
			if _, ok := r.Fields[f.FieldID]; !ok || seen[f.FieldID] {
				fields.Add(fmt.Sprintf("fields[%d].field_id", i), "unknown")
			}
			seen[f.FieldID] = true
		}
		if err := fields.Err(); err != nil {
			return err
		}
		if err := q.ClearTypeFields(ctx, typeID); err != nil {
			return err
		}
		for i, f := range list {
			if err := q.AttachField(ctx, db.AttachFieldParams{WorkspaceID: t.WorkspaceID, IssueTypeID: typeID, FieldID: f.FieldID, Required: f.Required, Position: int32(i)}); err != nil {
				return err
			}
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeIssueType, typeID, int64(it.Version), "IssueTypeFieldsSet",
			map[string]any{"fields": list}))
	})
}
