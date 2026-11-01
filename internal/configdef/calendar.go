package configdef

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/calendar"
	"github.com/lidisoft/horizon/internal/configdef/db"
	"github.com/lidisoft/horizon/internal/eventlog"
)

// Working calendar settings (docs/configuration.md#time-zone-and-working-calendar).
// The workspace's time zone is a column of workspaces; a project overrides
// it with a setting.
const (
	settingTimeZone     = "time_zone"
	settingWorkingDays  = "working_days"
	settingHolidays     = "holidays"
	settingHolidaysMode = "holidays_mode"
	maxHolidays         = 500
	maxHolidayName      = 100
)

// HolidayModes are how a project's non-working dates combine with the
// workspace's.
var HolidayModes = []string{"add", "replace"}

// ZoneSetting is the effective time zone and where it comes from.
type ZoneSetting struct {
	Value      string `json:"value" example:"Europe/Paris"`
	Workspace  string `json:"workspace" example:"UTC"`
	Overridden bool   `json:"overridden"`
}

// WeekdaysSetting is the effective working weekdays (ISO: Monday 1 …
// Sunday 7) and where they come from.
type WeekdaysSetting struct {
	Value      []int `json:"value"`
	Workspace  []int `json:"workspace"`
	Overridden bool  `json:"overridden"`
}

// HolidaysSetting is the effective non-working dates, the workspace's,
// and the project's own with how they combine (inherit, add, replace).
type HolidaysSetting struct {
	Value     []calendar.Holiday `json:"value"`
	Workspace []calendar.Holiday `json:"workspace"`
	Own       []calendar.Holiday `json:"own"`
	Mode      string             `json:"mode" enums:"inherit,add,replace"`
}

// CalendarSettings is a scope's working calendar.
type CalendarSettings struct {
	TimeZone    ZoneSetting     `json:"time_zone"`
	WorkingDays WeekdaysSetting `json:"working_days"`
	Holidays    HolidaysSetting `json:"holidays"`
}

// Calendar is the calendar the settings describe.
func (c CalendarSettings) Calendar() calendar.Calendar {
	return calendar.New(c.TimeZone.Value, c.WorkingDays.Value, c.Holidays.Value)
}

func readWeekdays(all map[string]json.RawMessage) ([]int, bool) {
	var v []int
	if all[settingWorkingDays] == nil || json.Unmarshal(all[settingWorkingDays], &v) != nil || len(v) == 0 || len(v) > 7 {
		return nil, false
	}
	for _, d := range v {
		if d < 1 || d > 7 {
			return nil, false
		}
	}
	slices.Sort(v)
	if len(slices.Compact(slices.Clone(v))) != len(v) {
		return nil, false
	}
	return v, true
}

// readHolidays reads and checks a list of non-working dates, sorted.
func readHolidays(all map[string]json.RawMessage) ([]calendar.Holiday, bool) {
	var v []calendar.Holiday
	if all[settingHolidays] == nil || json.Unmarshal(all[settingHolidays], &v) != nil || len(v) > maxHolidays {
		return nil, false
	}
	seen := map[string]bool{}
	for i, h := range v {
		h.Name = strings.TrimSpace(h.Name)
		if _, err := time.Parse(time.DateOnly, h.Date); err != nil || seen[h.Date] || h.Name == "" || len([]rune(h.Name)) > maxHolidayName {
			return nil, false
		}
		seen[h.Date] = true
		v[i] = h
	}
	return calendar.Merge(v, nil, "inherit"), true
}

func readZone(all map[string]json.RawMessage) (string, bool) {
	var v string
	if all[settingTimeZone] == nil || json.Unmarshal(all[settingTimeZone], &v) != nil || !validZone(v) {
		return "", false
	}
	return v, true
}

func validZone(tz string) bool {
	if strings.TrimSpace(tz) == "" || tz == "Local" {
		return false
	}
	_, err := time.LoadLocation(tz)
	return err == nil
}

func calendarSettings(ctx context.Context, conn db.DBTX, t Target) (CalendarSettings, error) {
	q := db.New(conn)
	var tz string
	if err := conn.QueryRow(ctx, `SELECT time_zone FROM workspaces WHERE id = $1`, t.WorkspaceID).Scan(&tz); err != nil {
		return CalendarSettings{}, err
	}
	wsRaw, err := q.WorkspaceSettings(ctx, t.WorkspaceID)
	if err != nil {
		return CalendarSettings{}, err
	}
	ws := settingsMap(wsRaw)
	out := CalendarSettings{
		TimeZone:    ZoneSetting{Value: tz, Workspace: tz},
		WorkingDays: WeekdaysSetting{Workspace: calendar.DefaultWeekdays},
		Holidays:    HolidaysSetting{Workspace: []calendar.Holiday{}, Own: []calendar.Holiday{}, Mode: "inherit"},
	}
	if v, ok := readWeekdays(ws); ok {
		out.WorkingDays.Workspace = v
	}
	if v, ok := readHolidays(ws); ok {
		out.Holidays.Workspace = v
	}
	out.WorkingDays.Value = out.WorkingDays.Workspace
	out.Holidays.Value = out.Holidays.Workspace
	if !t.ProjectID.Valid {
		return out, nil
	}
	pRaw, err := q.ProjectSettings(ctx, t.ProjectID.UUID)
	if err != nil {
		return CalendarSettings{}, err
	}
	p := settingsMap(pRaw)
	if v, ok := readZone(p); ok {
		out.TimeZone.Value, out.TimeZone.Overridden = v, true
	}
	if v, ok := readWeekdays(p); ok {
		out.WorkingDays.Value, out.WorkingDays.Overridden = v, true
	}
	if v, ok := readHolidays(p); ok {
		var mode string
		if json.Unmarshal(p[settingHolidaysMode], &mode) != nil || !slices.Contains(HolidayModes, mode) {
			mode = "add"
		}
		out.Holidays.Own, out.Holidays.Mode = v, mode
		out.Holidays.Value = calendar.Merge(out.Holidays.Workspace, v, mode)
	}
	return out, nil
}

// CalendarSettings returns the working calendar of the target. It reads
// through conn, so commands can read inside their transaction.
func (s *Service) CalendarSettings(ctx context.Context, conn db.DBTX, t Target) (CalendarSettings, error) {
	return calendarSettings(ctx, conn, t)
}

// CalendarInput changes the working calendar: a key that is absent is
// left alone; JSON null removes the value (a project then follows the
// workspace, the workspace the built-in default). Keys: time_zone
// (projects; the workspace's is a workspace property), working_days,
// holidays, holidays_mode (projects: add or replace).
type CalendarInput map[string]json.RawMessage

// SetCalendar applies a CalendarInput.
func (s *Service) SetCalendar(ctx context.Context, t Target, in CalendarInput) (CalendarSettings, error) {
	if err := s.requireManage(ctx, t); err != nil {
		return CalendarSettings{}, err
	}
	var fields apperr.Fields
	values := map[string][]byte{} // by setting key; nil removes it
	for key, raw := range in {
		one := map[string]json.RawMessage{key: raw}
		ok := true
		switch key {
		case settingTimeZone:
			if !t.ProjectID.Valid {
				fields.Add(key, "workspace_property") // PATCH /w/{ws}
				continue
			}
			if string(raw) != "null" {
				_, ok = readZone(one)
			}
		case settingWorkingDays:
			if string(raw) != "null" {
				var v []int
				if v, ok = readWeekdays(one); ok {
					raw, _ = json.Marshal(v)
				}
			}
		case settingHolidays:
			if string(raw) != "null" {
				var v []calendar.Holiday
				if v, ok = readHolidays(one); ok {
					raw, _ = json.Marshal(v)
				}
			}
		case settingHolidaysMode:
			var mode string
			ok = t.ProjectID.Valid && (string(raw) == "null" || (json.Unmarshal(raw, &mode) == nil && slices.Contains(HolidayModes, mode)))
		default:
			fields.Add(key, "unknown")
			continue
		}
		if !ok {
			fields.Add(key, "invalid")
			continue
		}
		if string(raw) == "null" {
			values[key] = nil
		} else {
			values[key] = raw
		}
	}
	if err := fields.Err(); err != nil {
		return CalendarSettings{}, err
	}
	// A project that stops overriding its dates drops how they combined.
	if t.ProjectID.Valid {
		if h, ok := values[settingHolidays]; ok && h == nil {
			values[settingHolidaysMode] = nil
		}
	}
	err := inTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, key := range keys {
			raw := values[key]
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
		return CalendarSettings{}, err
	}
	return s.CalendarSettings(ctx, s.pool, t)
}
