package calendar_test

import (
	"slices"
	"testing"
	"time"

	"github.com/lidisoft/horizon/internal/calendar"
)

func day(s string) time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestWorkingDays(t *testing.T) {
	// 2026-12-21 is a Monday; Christmas is a Friday.
	c := calendar.New("Europe/Paris", nil, []calendar.Holiday{{Date: "2026-12-25", Name: "Christmas"}})
	for d, want := range map[string]bool{"2026-12-24": true, "2026-12-25": false, "2026-12-26": false, "2026-12-28": true} {
		if got := c.Working(day(d)); got != want {
			t.Errorf("Working(%s) = %v", d, got)
		}
	}
	if name, ok := c.Holiday(day("2026-12-25")); !ok || name != "Christmas" {
		t.Errorf("Holiday = %q, %v", name, ok)
	}
	cases := []struct {
		from string
		n    int
		want string
	}{
		{"2026-12-24", 1, "2026-12-28"},  // over Christmas and the weekend
		{"2026-12-28", -1, "2026-12-24"}, // and back
		{"2026-12-21", 0, "2026-12-21"},
		{"2026-12-26", 1, "2026-12-28"}, // from a Saturday
		{"2026-12-21", 5, "2026-12-29"},
	}
	for _, tc := range cases {
		if got := c.Add(day(tc.from), tc.n).Format(time.DateOnly); got != tc.want {
			t.Errorf("Add(%s, %d) = %s, want %s", tc.from, tc.n, got, tc.want)
		}
	}
	if got := c.Between(day("2026-12-21"), day("2027-01-03")); got != 9 {
		t.Errorf("Between = %d, want 9", got)
	}
	if got := c.Between(day("2026-12-22"), day("2026-12-21")); got != 0 {
		t.Errorf("Between backwards = %d", got)
	}
	start, end := c.Span(day("2026-12-26"), 1)
	if start.Format(time.DateOnly) != "2026-12-28" || end.Format(time.DateOnly) != "2027-01-01" {
		t.Errorf("Span = %s – %s", start, end)
	}

	// Sunday to Thursday weeks.
	gulf := calendar.New("Asia/Dubai", []int{7, 1, 2, 3, 4}, nil)
	if !gulf.Working(day("2026-12-27")) || gulf.Working(day("2026-12-25")) {
		t.Error("Sunday–Thursday week")
	}
	if !slices.Equal(gulf.Weekdays(), []int{1, 2, 3, 4, 7}) {
		t.Errorf("Weekdays = %v", gulf.Weekdays())
	}

	// No working day at all doesn't loop forever.
	none := calendar.New("UTC", []int{9}, nil)
	_ = none.Add(day("2026-12-21"), 3)
}

func TestToday(t *testing.T) {
	now := time.Date(2026, 12, 31, 23, 30, 0, 0, time.UTC)
	if got := calendar.New("Europe/Paris", nil, nil).Today(now).Format(time.DateOnly); got != "2027-01-01" {
		t.Errorf("Paris today = %s", got)
	}
	if got := calendar.New("nowhere", nil, nil).Today(now).Format(time.DateOnly); got != "2026-12-31" {
		t.Errorf("unknown zone today = %s", got)
	}
}

func TestMerge(t *testing.T) {
	ws := []calendar.Holiday{{Date: "2026-12-25", Name: "Christmas"}, {Date: "2027-01-01", Name: "New Year"}}
	p := []calendar.Holiday{{Date: "2026-12-24", Name: "Eve"}, {Date: "2026-12-25", Name: "Noël"}}
	names := func(hs []calendar.Holiday) []string {
		var out []string
		for _, h := range hs {
			out = append(out, h.Date+" "+h.Name)
		}
		return out
	}
	if got := names(calendar.Merge(ws, p, "add")); !slices.Equal(got, []string{"2026-12-24 Eve", "2026-12-25 Noël", "2027-01-01 New Year"}) {
		t.Errorf("add = %v", got)
	}
	if got := names(calendar.Merge(ws, p, "replace")); !slices.Equal(got, []string{"2026-12-24 Eve", "2026-12-25 Noël"}) {
		t.Errorf("replace = %v", got)
	}
	if got := names(calendar.Merge(ws, p, "inherit")); !slices.Equal(got, []string{"2026-12-25 Christmas", "2027-01-01 New Year"}) {
		t.Errorf("inherit = %v", got)
	}
}
