package configdef

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Iteration settings (docs/configuration.md#catalogue): each is a setting
// of its own, so a project overrides one and inherits the others.
const (
	settingCompletion = "iteration_completion"
	settingMaxActive  = "max_active_iterations"
	settingLength     = "iteration_length"
)

// Iteration completion policies (docs/domain.md#iteration-completion-policy).
const (
	CompletionArchiveAll  = "archive_all"
	CompletionRollover    = "archive_done_rollover_rest"
	CompletionNone        = "none"
	DefaultCompletion     = CompletionRollover
	DefaultMaxActive      = 1
	DefaultIterationWeeks = 2
	maxMaxActive          = 10
	maxIterationWeeks     = 8
)

// CompletionPolicies lists the policies.
var CompletionPolicies = []string{CompletionRollover, CompletionArchiveAll, CompletionNone}

// PolicySetting is the effective completion policy and where it comes from.
type PolicySetting struct {
	Value     string `json:"value" enums:"archive_done_rollover_rest,archive_all,none"`
	Workspace string `json:"workspace" enums:"archive_done_rollover_rest,archive_all,none"`
	// Overridden is true when the project sets its own value.
	Overridden bool `json:"overridden"`
}

// CountSetting is an effective number and where it comes from.
type CountSetting struct {
	Value      int  `json:"value"`
	Workspace  int  `json:"workspace"`
	Overridden bool `json:"overridden"`
}

// IterationSettings are the settings planning follows.
type IterationSettings struct {
	CompletionPolicy PolicySetting `json:"completion_policy"`
	MaxActive        CountSetting  `json:"max_active_iterations"`
	// LengthWeeks is the default length of new iterations, in weeks.
	LengthWeeks CountSetting `json:"iteration_length_weeks"`
}

func settingsMap(raw json.RawMessage) map[string]json.RawMessage {
	var all map[string]json.RawMessage
	_ = json.Unmarshal(raw, &all)
	return all
}

func readPolicy(all map[string]json.RawMessage) (string, bool) {
	var v string
	if all[settingCompletion] == nil || json.Unmarshal(all[settingCompletion], &v) != nil || !slices.Contains(CompletionPolicies, v) {
		return "", false
	}
	return v, true
}

func readCount(all map[string]json.RawMessage, key string, hi int) (int, bool) {
	var v int
	if all[key] == nil || json.Unmarshal(all[key], &v) != nil || v < 1 || v > hi {
		return 0, false
	}
	return v, true
}

func iterationSettings(ctx context.Context, q *db.Queries, t Target) (IterationSettings, error) {
	wsRaw, err := q.WorkspaceSettings(ctx, t.WorkspaceID)
	if err != nil {
		return IterationSettings{}, err
	}
	ws := settingsMap(wsRaw)
	out := IterationSettings{
		CompletionPolicy: PolicySetting{Workspace: DefaultCompletion},
		MaxActive:        CountSetting{Workspace: DefaultMaxActive},
		LengthWeeks:      CountSetting{Workspace: DefaultIterationWeeks},
	}
	if v, ok := readPolicy(ws); ok {
		out.CompletionPolicy.Workspace = v
	}
	if v, ok := readCount(ws, settingMaxActive, maxMaxActive); ok {
		out.MaxActive.Workspace = v
	}
	if v, ok := readCount(ws, settingLength, maxIterationWeeks); ok {
		out.LengthWeeks.Workspace = v
	}
	out.CompletionPolicy.Value = out.CompletionPolicy.Workspace
	out.MaxActive.Value = out.MaxActive.Workspace
	out.LengthWeeks.Value = out.LengthWeeks.Workspace
	if !t.ProjectID.Valid {
		return out, nil
	}
	pRaw, err := q.ProjectSettings(ctx, t.ProjectID.UUID)
	if err != nil {
		return IterationSettings{}, err
	}
	p := settingsMap(pRaw)
	if v, ok := readPolicy(p); ok {
		out.CompletionPolicy.Value, out.CompletionPolicy.Overridden = v, true
	}
	if v, ok := readCount(p, settingMaxActive, maxMaxActive); ok {
		out.MaxActive.Value, out.MaxActive.Overridden = v, true
	}
	if v, ok := readCount(p, settingLength, maxIterationWeeks); ok {
		out.LengthWeeks.Value, out.LengthWeeks.Overridden = v, true
	}
	return out, nil
}

// IterationSettings returns the effective iteration settings of the
// target. It reads through conn, so commands can read inside their
// transaction.
func (s *Service) IterationSettings(ctx context.Context, conn db.DBTX, t Target) (IterationSettings, error) {
	return iterationSettings(ctx, db.New(conn), t)
}

// IterationSettingsInput changes iteration settings: a key that is absent
// is left alone, JSON null removes the value (a project then follows the
// workspace, the workspace the built-in default).
type IterationSettingsInput map[string]json.RawMessage

// SetIterationSettings applies an IterationSettingsInput.
func (s *Service) SetIterationSettings(ctx context.Context, t Target, in IterationSettingsInput) (IterationSettings, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return IterationSettings{}, err
	}
	var fields apperr.Fields
	values := map[string][]byte{} // by setting key; nil removes it
	for public, raw := range in {
		key, hi := "", 0
		switch public {
		case "completion_policy":
			key = settingCompletion
		case "max_active_iterations":
			key, hi = settingMaxActive, maxMaxActive
		case "iteration_length_weeks":
			key, hi = settingLength, maxIterationWeeks
		default:
			fields.Add(public, "unknown")
			continue
		}
		if string(raw) == "null" {
			values[key] = nil
			continue
		}
		one := map[string]json.RawMessage{key: raw}
		ok := false
		if hi == 0 {
			_, ok = readPolicy(one)
		} else {
			_, ok = readCount(one, key, hi)
		}
		if !ok {
			fields.Add(public, "invalid")
			continue
		}
		values[key] = raw
	}
	if err := fields.Err(); err != nil {
		return IterationSettings{}, err
	}
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		for key, raw := range values {
			var err error
			switch {
			case !t.ProjectID.Valid && raw == nil:
				_, err = tx.Exec(ctx, `UPDATE workspaces SET settings = settings - $2::text, updated_at = now() WHERE id = $1`, t.WorkspaceID, key)
			case !t.ProjectID.Valid:
				err = q.SetWorkspaceSetting(ctx, db.SetWorkspaceSettingParams{ID: t.WorkspaceID, Key: key, Value: raw})
			case raw == nil:
				err = q.ClearProjectSetting(ctx, db.ClearProjectSettingParams{ID: t.ProjectID.UUID, Key: key})
			default:
				err = q.SetProjectSetting(ctx, db.SetProjectSettingParams{ID: t.ProjectID.UUID, Key: key, Value: raw})
			}
			if err != nil {
				return err
			}
			aggregate, id := "workspace", t.WorkspaceID
			if t.ProjectID.Valid {
				aggregate, id = "project", t.ProjectID.UUID
			}
			if err := eventlog.Append(ctx, tx, s.event(t, aggregate, id, 0, "SettingChanged",
				map[string]any{"setting": key, "value": json.RawMessage(nullIfEmpty(raw))})); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return IterationSettings{}, err
	}
	return s.IterationSettings(ctx, s.pool, t)
}
