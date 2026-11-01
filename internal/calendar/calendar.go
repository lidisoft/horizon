// Package calendar is a project's working calendar
// (docs/configuration.md#time-zone-and-working-calendar): its time zone,
// working weekdays and non-working dates, and arithmetic in working days.
// Dates are calendar dates, held as midnight UTC. web/app/components/
// calendar.js mirrors it for the timeline.
package calendar

import (
	"slices"
	"time"
)

// Holiday is a non-working date.
type Holiday struct {
	Date string `json:"date" example:"2026-12-25"`
	Name string `json:"name" example:"Christmas"`
}

// DefaultWeekdays are Monday to Friday, as ISO weekday numbers (Monday 1 … Sunday 7).
var DefaultWeekdays = []int{1, 2, 3, 4, 5}

// maxScan bounds the walks: ten years of days, in case a calendar has
// (almost) no working days left.
const maxScan = 3660

// Calendar answers which days are worked.
type Calendar struct {
	Location *time.Location
	working  [8]bool // by ISO weekday
	holidays map[string]string
}

// New builds a calendar; an unknown time zone is UTC, no weekdays are
// the defaults.
func New(tz string, weekdays []int, holidays []Holiday) Calendar {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	c := Calendar{Location: loc, holidays: map[string]string{}}
	if len(weekdays) == 0 {
		weekdays = DefaultWeekdays
	}
	for _, d := range weekdays {
		if d >= 1 && d <= 7 {
			c.working[d] = true
		}
	}
	for _, h := range holidays {
		c.holidays[h.Date] = h.Name
	}
	return c
}

// Date is the calendar date of t, at midnight UTC.
func Date(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Today is the date now in the calendar's time zone.
func (c Calendar) Today(now time.Time) time.Time {
	return Date(now.In(c.Location))
}

// ISOWeekday is 1 (Monday) to 7 (Sunday).
func ISOWeekday(d time.Time) int {
	if w := int(d.Weekday()); w != 0 {
		return w
	}
	return 7
}

// Holiday names the non-working date d, if it is one.
func (c Calendar) Holiday(d time.Time) (string, bool) {
	name, ok := c.holidays[d.Format(time.DateOnly)]
	return name, ok
}

// Working reports whether d is a working day.
func (c Calendar) Working(d time.Time) bool {
	if !c.working[ISOWeekday(d)] {
		return false
	}
	_, holiday := c.Holiday(d)
	return !holiday
}

// Next is d if it is a working day, else the first one after (dir 1) or
// before (dir -1) it.
func (c Calendar) Next(d time.Time, dir int) time.Time {
	for i := 0; i < maxScan && !c.Working(d); i++ {
		d = d.AddDate(0, 0, dir)
	}
	return d
}

// Add moves n working days from d (back when n < 0): from a working day,
// Add(d, 1) is the next working day; from a non-working day it counts
// from the nearest working day in that direction.
func (c Calendar) Add(d time.Time, n int) time.Time {
	dir := 1
	if n < 0 {
		dir, n = -1, -n
	}
	for i := 0; n > 0 && i < maxScan; i++ {
		d = d.AddDate(0, 0, dir)
		if c.Working(d) {
			n--
		}
	}
	return d
}

// Between counts the working days from from to to, both included (0 when
// to is before from).
func (c Calendar) Between(from, to time.Time) int {
	n := 0
	for d := from; !d.After(to) && n <= maxScan; d = d.AddDate(0, 0, 1) {
		if c.Working(d) {
			n++
		}
	}
	return n
}

// Span is a period of weeks from start: the first working day from start,
// to the last working day before the same weekday weeks later.
func (c Calendar) Span(start time.Time, weeks int) (time.Time, time.Time) {
	start = c.Next(start, 1)
	end := c.Next(start.AddDate(0, 0, 7*weeks-1), -1)
	if end.Before(start) {
		end = start
	}
	return start, end
}

// Weekdays lists the working ISO weekdays.
func (c Calendar) Weekdays() []int {
	var out []int
	for d := 1; d <= 7; d++ {
		if c.working[d] {
			out = append(out, d)
		}
	}
	return out
}

// Merge combines a workspace's holidays with a project's: "add" keeps
// both (the project's name wins on the same date), "replace" keeps the
// project's, anything else the workspace's. The result is in date order.
func Merge(workspace, project []Holiday, mode string) []Holiday {
	var out []Holiday
	switch mode {
	case "replace":
		out = slices.Clone(project)
	case "add":
		byDate := map[string]Holiday{}
		for _, h := range workspace {
			byDate[h.Date] = h
		}
		for _, h := range project {
			byDate[h.Date] = h
		}
		for _, h := range byDate {
			out = append(out, h)
		}
	default:
		out = slices.Clone(workspace)
	}
	slices.SortFunc(out, func(a, b Holiday) int {
		if a.Date < b.Date {
			return -1
		}
		if a.Date > b.Date {
			return 1
		}
		return 0
	})
	return out
}
