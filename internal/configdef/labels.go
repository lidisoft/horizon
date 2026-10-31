package configdef

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

func labelFrom(l db.Label) Label {
	return Label{ID: l.ID, ProjectID: l.ProjectID, Name: l.Name, Colour: l.Colour, Description: l.Description, Version: l.Version}
}

// Labels lists the labels seen from the target, with how many issues carry
// each (docs/configuration.md#labels).
func (s *Service) Labels(ctx context.Context, t Target) ([]Label, error) {
	q := db.New(s.pool)
	off, err := disabled(ctx, q, t)
	if err != nil {
		return nil, err
	}
	rows, err := q.Labels(ctx, db.LabelsParams{WorkspaceID: t.WorkspaceID, ProjectID: t.ProjectID})
	if err != nil {
		return nil, err
	}
	counts, err := s.issues.CountByLabel(ctx, s.pool, t.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Label, len(rows))
	for i, r := range rows {
		out[i] = labelFrom(r)
		out[i].Origin = origin(t, r.ProjectID, off[TypeLabel], r.ID)
		out[i].Issues = counts[r.ID]
	}
	return out, nil
}

// LabelInput creates or changes a label; nil fields are unchanged.
type LabelInput struct {
	Name        *string
	Colour      *string
	Description *string
}

func validateLabel(in LabelInput, l *db.Label, fields *apperr.Fields) {
	if in.Name != nil {
		l.Name = name(fields, "name", *in.Name, 40)
	}
	if in.Colour != nil {
		l.Colour = *in.Colour
		colour(fields, l.Colour)
	}
	if in.Description != nil {
		l.Description = strings.TrimSpace(*in.Description)
		if len([]rune(l.Description)) > 200 {
			fields.Add("description", "too_long")
		}
	}
}

// CreateLabel adds a label in the target's scope.
func (s *Service) CreateLabel(ctx context.Context, t Target, in LabelInput) (Label, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Label{}, err
	}
	var fields apperr.Fields
	v := db.Label{Colour: "grey"}
	if in.Name == nil {
		in.Name = new(string)
	}
	validateLabel(in, &v, &fields)
	if err := fields.Err(); err != nil {
		return Label{}, err
	}
	var l db.Label
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		l, err = db.New(tx).CreateLabel(ctx, db.CreateLabelParams{
			WorkspaceID: t.WorkspaceID, ID: uuid.Must(uuid.NewV7()), ProjectID: t.ProjectID, Name: v.Name, Colour: v.Colour,
			Description: v.Description, CreatedAt: s.now(),
		})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeLabel, l.ID, int64(l.Version), "LabelCreated", nil))
	})
	if err != nil {
		return Label{}, err
	}
	return labelFrom(l), nil
}

func (s *Service) lockOwnLabel(ctx context.Context, q *db.Queries, t Target, id uuid.UUID) (db.Label, error) {
	l, err := q.LockLabel(ctx, db.LockLabelParams{WorkspaceID: t.WorkspaceID, ID: id})
	if err != nil {
		return l, notFound(err, "label_not_found")
	}
	if !sameScope(t, l.ProjectID) {
		return l, conflict("inherited_read_only")
	}
	return l, nil
}

// UpdateLabel renames, recolours or redescribes a label.
func (s *Service) UpdateLabel(ctx context.Context, t Target, id uuid.UUID, version int32, in LabelInput) (Label, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Label{}, err
	}
	var l db.Label
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		cur, err := s.lockOwnLabel(ctx, q, t, id)
		if err != nil {
			return err
		}
		if version != 0 && cur.Version != version {
			return conflict("version_conflict")
		}
		v := cur
		var fields apperr.Fields
		validateLabel(in, &v, &fields)
		if err := fields.Err(); err != nil {
			return err
		}
		l, err = q.UpdateLabel(ctx, db.UpdateLabelParams{WorkspaceID: cur.WorkspaceID, ID: cur.ID, Name: v.Name, Colour: v.Colour, Description: v.Description, UpdatedAt: s.now()})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeLabel, l.ID, int64(l.Version), "LabelUpdated", nil))
	})
	if err != nil {
		return Label{}, err
	}
	return labelFrom(l), nil
}

// DeleteLabel removes a label; issues lose it (the issue module scrubs it).
func (s *Service) DeleteLabel(ctx context.Context, t Target, id uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		l, err := s.lockOwnLabel(ctx, q, t, id)
		if err != nil {
			return err
		}
		return s.deleteLabel(ctx, tx, t, l)
	})
}

func (s *Service) deleteLabel(ctx context.Context, tx pgx.Tx, t Target, l db.Label) error {
	if _, err := tx.Exec(ctx, `DELETE FROM config_disables WHERE definition_type = $1 AND definition_id = $2`, TypeLabel, l.ID); err != nil {
		return err
	}
	if err := db.New(tx).DeleteLabel(ctx, db.DeleteLabelParams{WorkspaceID: t.WorkspaceID, ID: l.ID}); err != nil {
		return err
	}
	return eventlog.Append(ctx, tx, s.event(t, TypeLabel, l.ID, int64(l.Version)+1, "LabelDeleted", nil))
}

// MergeLabel relabels every issue carrying id with into (a label the
// target sees), recording it in their history, then deletes id.
func (s *Service) MergeLabel(ctx context.Context, t Target, id, into uuid.UUID) error {
	if err := s.requireManage(ctx, t); err != nil {
		return err
	}
	if id == into {
		return apperr.Invalid(apperr.Field{Field: "into", Code: "same"})
	}
	return inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		from, err := s.lockOwnLabel(ctx, q, t, id)
		if err != nil {
			return err
		}
		r, err := s.Resolve(ctx, tx, t.WorkspaceID, t.ProjectID)
		if err != nil {
			return err
		}
		target, ok := r.Labels[into]
		// A workspace label is used in every project: only another
		// workspace label can take its place everywhere.
		if !ok || (!from.ProjectID.Valid && target.ProjectID.Valid) {
			return apperr.Invalid(apperr.Field{Field: "into", Code: "unknown"})
		}
		if err := s.issues.Relabel(ctx, tx, actor(t), id, into); err != nil {
			return err
		}
		if err := eventlog.Append(ctx, tx, s.event(t, TypeLabel, into, int64(target.Version), "LabelMerged",
			map[string]any{"from": id})); err != nil {
			return err
		}
		return s.deleteLabel(ctx, tx, t, from)
	})
}

// MoveLabelToWorkspace promotes one of the target project's labels to the
// workspace, where every project sees it.
func (s *Service) MoveLabelToWorkspace(ctx context.Context, t Target, id uuid.UUID) (Label, error) {
	if !t.ProjectID.Valid {
		return Label{}, conflict("not_in_project")
	}
	// Workspace definitions are the workspace's to change.
	ws := Target{Principal: t.Principal, WorkspaceID: t.WorkspaceID}
	if err := s.requireManage(ctx, ws); err != nil {
		return Label{}, err
	}
	var l db.Label
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if _, err := s.lockOwnLabel(ctx, q, t, id); err != nil {
			return err
		}
		var err error
		l, err = q.MoveLabelToWorkspace(ctx, db.MoveLabelToWorkspaceParams{WorkspaceID: t.WorkspaceID, ID: id, UpdatedAt: s.now()})
		if isUniqueViolation(err) {
			return apperr.Invalid(apperr.Field{Field: "name", Code: "taken"})
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(t, TypeLabel, l.ID, int64(l.Version), "LabelMovedToWorkspace", nil))
	})
	if err != nil {
		return Label{}, err
	}
	return labelFrom(l), nil
}
