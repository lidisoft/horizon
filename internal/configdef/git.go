package configdef

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Git settings (docs/configuration.md#catalogue): what a merged pull
// request does to the issues it names.
const settingGitMerge = "git_merge_status"

// Merge behaviours: a status name, or one of these.
const (
	// MergeCompleted moves to the nearest completed status.
	MergeCompleted = "@completed"
	// MergeOff leaves the issue alone (the default).
	MergeOff = "@off"
)

// TextSetting is an effective text value and where it comes from.
type TextSetting struct {
	Value     string `json:"value"`
	Workspace string `json:"workspace"`
	// Overridden is true when the project sets its own value.
	Overridden bool `json:"overridden"`
}

// GitSettings are the settings Git integrations follow.
type GitSettings struct {
	// MergeStatus is where a merged pull request moves the issues it names:
	// "@off" (the default), "@completed" (the nearest completed status), or
	// a status name (where the issue's workflow has it).
	MergeStatus TextSetting `json:"merge_status"`
}

func readText(all map[string]json.RawMessage, key string) (string, bool) {
	var v string
	if all[key] == nil || json.Unmarshal(all[key], &v) != nil || strings.TrimSpace(v) == "" || len(v) > 100 {
		return "", false
	}
	return strings.TrimSpace(v), true
}

// GitSettings returns the effective Git settings of the target.
func (s *Service) GitSettings(ctx context.Context, conn db.DBTX, t Target) (GitSettings, error) {
	q := db.New(conn)
	wsRaw, err := q.WorkspaceSettings(ctx, t.WorkspaceID)
	if err != nil {
		return GitSettings{}, err
	}
	out := GitSettings{MergeStatus: TextSetting{Workspace: MergeOff}}
	if v, ok := readText(settingsMap(wsRaw), settingGitMerge); ok {
		out.MergeStatus.Workspace = v
	}
	out.MergeStatus.Value = out.MergeStatus.Workspace
	if !t.ProjectID.Valid {
		return out, nil
	}
	pRaw, err := q.ProjectSettings(ctx, t.ProjectID.UUID)
	if err != nil {
		return GitSettings{}, err
	}
	if v, ok := readText(settingsMap(pRaw), settingGitMerge); ok {
		out.MergeStatus.Value, out.MergeStatus.Overridden = v, true
	}
	return out, nil
}

// GitSettingsInput changes Git settings: an absent key is left alone, JSON
// null removes the value.
type GitSettingsInput struct {
	MergeStatus json.RawMessage `json:"merge_status,omitempty" swaggertype:"string" example:"@completed"`
}

// SetGitSettings applies a GitSettingsInput.
func (s *Service) SetGitSettings(ctx context.Context, t Target, in GitSettingsInput) (GitSettings, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return GitSettings{}, err
	}
	if in.MergeStatus == nil {
		return s.GitSettings(ctx, s.pool, t)
	}
	var raw []byte
	if string(in.MergeStatus) != "null" {
		if _, ok := readText(map[string]json.RawMessage{settingGitMerge: in.MergeStatus}, settingGitMerge); !ok {
			return GitSettings{}, apperr.Invalid(apperr.Field{Field: "merge_status", Code: "invalid"})
		}
		raw = in.MergeStatus
	}
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		var err error
		switch {
		case !t.ProjectID.Valid && raw == nil:
			_, err = tx.Exec(ctx, `UPDATE workspaces SET settings = settings - $2::text, updated_at = now() WHERE id = $1`, t.WorkspaceID, settingGitMerge)
		case !t.ProjectID.Valid:
			err = q.SetWorkspaceSetting(ctx, db.SetWorkspaceSettingParams{ID: t.WorkspaceID, Key: settingGitMerge, Value: raw})
		case raw == nil:
			err = q.ClearProjectSetting(ctx, db.ClearProjectSettingParams{ID: t.ProjectID.UUID, Key: settingGitMerge})
		default:
			err = q.SetProjectSetting(ctx, db.SetProjectSettingParams{ID: t.ProjectID.UUID, Key: settingGitMerge, Value: raw})
		}
		if err != nil {
			return err
		}
		aggregate, id := "workspace", t.WorkspaceID
		if t.ProjectID.Valid {
			aggregate, id = "project", t.ProjectID.UUID
		}
		return eventlog.Append(ctx, tx, s.event(t, aggregate, id, 0, "SettingChanged",
			map[string]any{"setting": settingGitMerge, "value": json.RawMessage(nullIfEmpty(raw))}))
	})
	if err != nil {
		return GitSettings{}, err
	}
	return s.GitSettings(ctx, s.pool, t)
}
