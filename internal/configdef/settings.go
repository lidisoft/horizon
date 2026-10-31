package configdef

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Settings are scalar configuration: a project value replaces the
// workspace's, which replaces the built-in default
// (docs/configuration.md#resolution). They live in the settings JSONB of
// workspaces and projects, one key each.

const settingEstimation = "estimation"

// Built-in scales.
var (
	fibonacci = []ScaleValue{{"0", 0}, {"1", 1}, {"2", 2}, {"3", 3}, {"5", 5}, {"8", 8}, {"13", 13}, {"21", 21}}
	linear    = []ScaleValue{{"0", 0}, {"1", 1}, {"2", 2}, {"3", 3}, {"4", 4}, {"5", 5}, {"6", 6}, {"7", 7}, {"8", 8}}
	tshirt    = []ScaleValue{{"XS", 1}, {"S", 2}, {"M", 3}, {"L", 5}, {"XL", 8}}
)

// DefaultScale is the built-in default: Fibonacci points.
var DefaultScale = Scale{Kind: ScaleFibonacci, Values: fibonacci}

// maxScaleValues bounds custom scales.
const maxScaleValues = 20

func scaleFrom(raw json.RawMessage) (Scale, bool) {
	var all map[string]json.RawMessage
	if json.Unmarshal(raw, &all) != nil || all[settingEstimation] == nil {
		return Scale{}, false
	}
	var sc Scale
	if json.Unmarshal(all[settingEstimation], &sc) != nil {
		return Scale{}, false
	}
	return withValues(sc), true
}

// withValues fills the fixed values of built-in scales.
func withValues(sc Scale) Scale {
	switch sc.Kind {
	case ScaleFibonacci:
		sc.Values = fibonacci
	case ScaleLinear:
		sc.Values = linear
	case ScaleNone:
		sc.Values = []ScaleValue{}
	case ScaleTShirt:
		if len(sc.Values) == 0 {
			sc.Values = tshirt
		}
	}
	if sc.Values == nil {
		sc.Values = []ScaleValue{}
	}
	return sc
}

func estimation(ctx context.Context, q *db.Queries, t Target) (Estimation, error) {
	wsRaw, err := q.WorkspaceSettings(ctx, t.WorkspaceID)
	if err != nil {
		return Estimation{}, err
	}
	e := Estimation{Workspace: DefaultScale}
	if sc, ok := scaleFrom(wsRaw); ok {
		e.Workspace = sc
	}
	e.Scale = e.Workspace
	if t.ProjectID.Valid {
		pRaw, err := q.ProjectSettings(ctx, t.ProjectID.UUID)
		if err != nil {
			return Estimation{}, err
		}
		if sc, ok := scaleFrom(pRaw); ok {
			e.Scale, e.Overridden = sc, true
		}
	}
	return e, nil
}

// Estimation returns the effective estimation scale of the target.
func (s *Service) Estimation(ctx context.Context, t Target) (Estimation, error) {
	return estimation(ctx, db.New(s.pool), t)
}

// SetEstimation sets the target's scale. At project scope, nil removes the
// override: the project follows the workspace again.
func (s *Service) SetEstimation(ctx context.Context, t Target, sc *Scale) (Estimation, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return Estimation{}, err
	}
	if sc == nil && !t.ProjectID.Valid {
		return Estimation{}, apperr.Invalid(apperr.Field{Field: "scale", Code: "required"})
	}
	var raw []byte
	if sc != nil {
		v, err := validateScale(*sc)
		if err != nil {
			return Estimation{}, err
		}
		raw, _ = json.Marshal(v)
	}
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		switch {
		case !t.ProjectID.Valid:
			err = q.SetWorkspaceSetting(ctx, db.SetWorkspaceSettingParams{ID: t.WorkspaceID, Key: settingEstimation, Value: raw})
		case sc == nil:
			err = q.ClearProjectSetting(ctx, db.ClearProjectSettingParams{ID: t.ProjectID.UUID, Key: settingEstimation})
		default:
			err = q.SetProjectSetting(ctx, db.SetProjectSettingParams{ID: t.ProjectID.UUID, Key: settingEstimation, Value: raw})
		}
		if err != nil {
			return err
		}
		aggregate, id := "workspace", t.WorkspaceID
		if t.ProjectID.Valid {
			aggregate, id = "project", t.ProjectID.UUID
		}
		return eventlog.Append(ctx, tx, s.event(t, aggregate, id, 0, "SettingChanged",
			map[string]any{"setting": settingEstimation, "value": json.RawMessage(nullIfEmpty(raw))}))
	})
	if err != nil {
		return Estimation{}, err
	}
	return s.Estimation(ctx, t)
}

func nullIfEmpty(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

func validateScale(sc Scale) (Scale, error) {
	var fields apperr.Fields
	switch sc.Kind {
	case ScaleFibonacci, ScaleLinear, ScaleNone:
		return Scale{Kind: sc.Kind}, nil
	case ScaleTShirt, ScaleCustom:
	default:
		fields.Add("kind", "invalid")
		return Scale{}, fields.Err()
	}
	if len(sc.Values) == 0 {
		fields.Add("values", "required")
	}
	if len(sc.Values) > maxScaleValues {
		fields.Add("values", "too_many")
	}
	seen := map[string]bool{}
	out := Scale{Kind: sc.Kind, Values: []ScaleValue{}}
	for i, v := range sc.Values {
		at := fmt.Sprintf("values[%d]", i)
		v.Label = strings.TrimSpace(v.Label)
		if v.Label == "" || len([]rune(v.Label)) > 10 {
			fields.Add(at+".label", "invalid")
		}
		if seen[strings.ToLower(v.Label)] {
			fields.Add(at+".label", "taken")
		}
		seen[strings.ToLower(v.Label)] = true
		if v.Weight < 0 || v.Weight > 1000 {
			fields.Add(at+".weight", "invalid")
		}
		out.Values = append(out.Values, v)
	}
	return out, fields.Err()
}
