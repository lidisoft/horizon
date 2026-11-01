package issue

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/document"
)

// Limits of custom field values.
const (
	maxTextValue = 1000
	maxURLValue  = 2000
)

// validateFields merges changes (field id → JSON value, null removes) into
// the issue's current values: only fields attached to the type, each value
// of its field's type. creating also requires the type's required fields.
func (s *Service) validateFields(ctx context.Context, tx pgx.Tx, fields *apperr.Fields, r *configdef.Resolved, typeID uuid.UUID,
	current json.RawMessage, changes map[string]json.RawMessage, creating bool) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(current, &values); err != nil {
		return nil, err
	}
	attached := map[uuid.UUID]configdef.TypeField{}
	for _, tf := range r.TypeFields[typeID] {
		attached[tf.FieldID] = tf
	}
	for key, raw := range changes {
		at := "fields." + key
		id, err := uuid.Parse(key)
		if _, ok := attached[id]; err != nil || !ok {
			fields.Add(at, "not_on_type")
			continue
		}
		if isNull(raw) {
			delete(values, key)
			continue
		}
		if code := s.checkValue(ctx, tx, r.Fields[id], raw); code != "" {
			fields.Add(at, code)
			continue
		}
		values[key] = raw
	}
	if creating {
		for id, tf := range attached {
			if tf.Required && isEmptyValue(values[id.String()]) {
				fields.Add("fields."+id.String(), "required")
			}
		}
	}
	out, err := json.Marshal(values)
	return out, err
}

func isNull(raw json.RawMessage) bool {
	return len(bytes.TrimSpace(raw)) == 0 || string(bytes.TrimSpace(raw)) == "null"
}

// isEmptyValue: missing, null, "", [] or false count as not set.
func isEmptyValue(raw json.RawMessage) bool {
	switch string(bytes.TrimSpace(raw)) {
	case "", "null", `""`, "[]", "false":
		return true
	}
	return false
}

// checkValue returns a field error code, or "".
func (s *Service) checkValue(ctx context.Context, tx pgx.Tx, f configdef.Field, raw json.RawMessage) string {
	option := func(id string) bool {
		for _, o := range f.Options {
			if o.ID.String() == id {
				return true
			}
		}
		return false
	}
	switch f.Type {
	case "text":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
		if len([]rune(v)) > maxTextValue {
			return "too_long"
		}
	case "number":
		var v float64
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
	case "checkbox":
		var v bool
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
	case "date":
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
		if _, err := time.Parse(time.DateOnly, v); err != nil {
			return "invalid"
		}
	case "url":
		var v string
		if json.Unmarshal(raw, &v) != nil || len(v) > maxURLValue {
			return "invalid"
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return "invalid"
		}
	case "select":
		var v string
		if json.Unmarshal(raw, &v) != nil || !option(v) {
			return "invalid"
		}
	case "multi_select":
		var v []string
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
		for _, id := range v {
			if !option(id) {
				return "invalid"
			}
		}
	case "user":
		var v uuid.UUID
		if json.Unmarshal(raw, &v) != nil {
			return "invalid"
		}
		var member bool
		err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM workspace_members m JOIN custom_fields f ON f.workspace_id = m.workspace_id
			WHERE f.id = $1 AND m.user_id = $2)`, f.ID, v).Scan(&member)
		if err != nil || !member {
			return "not_member"
		}
	default:
		return "invalid"
	}
	return ""
}

// isSet reports whether a field (built-in name or custom field id) has a
// value on the issue, for workflow validators.
func isSet(e *edit, field string) bool {
	i := e.next
	switch field {
	case "assignee":
		return i.AssigneeID.Valid
	case "estimate":
		return i.Estimate != nil
	case "priority":
		return i.Priority != "none"
	case "due_date":
		return i.DueDate.Valid
	case "start_date":
		return i.StartDate.Valid
	case "iteration":
		return i.IterationID.Valid
	case "release":
		return i.ReleaseID.Valid
	case "description":
		return len(bytes.TrimSpace([]byte(i.Description))) > 0
	case "labels":
		if e.labels != nil {
			return len(*e.labels) > 0
		}
		var n int
		_ = e.tx.QueryRow(context.Background(), `SELECT count(*) FROM issue_labels WHERE issue_id = $1`, i.ID).Scan(&n)
		return n > 0
	}
	values := map[string]json.RawMessage{}
	_ = json.Unmarshal(i.Fields, &values)
	return !isEmptyValue(values[field])
}

// clearField empties a field (workflow post-action).
func clearField(e *edit, field string) {
	i := e.next
	switch field {
	case "assignee":
		i.AssigneeID = uuid.NullUUID{}
	case "estimate":
		i.Estimate = nil
	case "priority":
		i.Priority = "none"
	case "due_date":
		i.DueDate.Valid = false
	case "start_date":
		i.StartDate.Valid = false
	case "iteration":
		i.IterationID = uuid.NullUUID{}
	case "release":
		i.ReleaseID = uuid.NullUUID{}
	case "description":
		i.Description = ""
		e.description = document.FromText(docSchema, "")
	case "labels":
		empty := []uuid.UUID{}
		e.labels = &empty
	default:
		values := map[string]json.RawMessage{}
		_ = json.Unmarshal(i.Fields, &values)
		delete(values, field)
		i.Fields, _ = json.Marshal(values)
	}
}
