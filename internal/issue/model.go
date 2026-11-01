// Package issue owns issues: creation, field edits, transitions through
// their workflow, assignment, estimates, priority, rank, hierarchy, links
// and archiving (docs/domain.md#issue). Every command writes the issue row
// and its domain events in one transaction (docs/architecture.md#write-path).
package issue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lidisoft/horizon/internal/issue/db"
)

// Priorities, most urgent first.
var Priorities = []string{"urgent", "high", "medium", "low", "none"}

// Issue is the public view of an issue.
type Issue struct {
	ID          uuid.UUID     `json:"id"`
	Key         string        `json:"key" example:"HZN-42"`
	ProjectID   uuid.UUID     `json:"project_id"`
	Number      int64         `json:"number"`
	TypeID      uuid.UUID     `json:"type_id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	StatusID    uuid.UUID     `json:"status_id"`
	Priority    string        `json:"priority" enums:"none,urgent,high,medium,low"`
	Rank        string        `json:"rank"`
	Estimate    *string       `json:"estimate"`
	AssigneeID  uuid.NullUUID `json:"assignee_id" swaggertype:"string"`
	ReporterID  uuid.NullUUID `json:"reporter_id" swaggertype:"string"`
	ParentID    uuid.NullUUID `json:"parent_id" swaggertype:"string"`
	IterationID uuid.NullUUID `json:"iteration_id" swaggertype:"string"`
	ReleaseID   uuid.NullUUID `json:"release_id" swaggertype:"string"`
	// Parent names the parent when the caller sees it (GET only).
	Parent     *Ref            `json:"parent,omitempty"`
	Labels     []uuid.UUID     `json:"labels"`
	StartDate  *string         `json:"start_date" example:"2026-11-02"`
	DueDate    *string         `json:"due_date" example:"2026-11-13"`
	Fields     json.RawMessage `json:"fields" swaggertype:"object"`
	StartedAt  *time.Time      `json:"started_at"`
	ResolvedAt *time.Time      `json:"resolved_at"`
	ArchivedAt *time.Time      `json:"archived_at"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
	Version    int32           `json:"version"`
}

func issueFrom(i db.Issue, projectKey string, labels []uuid.UUID) Issue {
	if labels == nil {
		labels = []uuid.UUID{}
	}
	return Issue{
		ID: i.ID, Key: fmt.Sprintf("%s-%d", projectKey, i.Number), ProjectID: i.ProjectID, Number: i.Number, TypeID: i.TypeID,
		Title: i.Title, Description: i.Description, StatusID: i.StatusID, Priority: i.Priority, Rank: i.Rank, Estimate: i.Estimate,
		AssigneeID: i.AssigneeID, ReporterID: i.ReporterID, ParentID: i.ParentID, IterationID: i.IterationID, ReleaseID: i.ReleaseID,
		Labels:    labels,
		StartDate: dateString(i.StartDate), DueDate: dateString(i.DueDate), Fields: i.Fields,
		StartedAt: i.StartedAt, ResolvedAt: i.ResolvedAt, ArchivedAt: i.ArchivedAt,
		CreatedAt: i.CreatedAt, UpdatedAt: i.UpdatedAt, Version: i.Version,
	}
}

func dateString(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(time.DateOnly)
	return &s
}

// parseDate reads "YYYY-MM-DD"; empty clears.
func parseDate(s string) (pgtype.Date, bool) {
	if s == "" {
		return pgtype.Date{}, true
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return pgtype.Date{}, false
	}
	return pgtype.Date{Time: t, Valid: true}, true
}

// ParseKey splits "HZN-42" into the project key and the number.
func ParseKey(key string) (string, int64, bool) {
	i := strings.LastIndexByte(key, '-')
	if i < 1 {
		return "", 0, false
	}
	n, err := strconv.ParseInt(key[i+1:], 10, 64)
	if err != nil || n < 1 {
		return "", 0, false
	}
	return strings.ToUpper(key[:i]), n, true
}

// Link types as the API names them; stored links use the first of each
// pair, from its source (blocked_by is a blocks link the other way).
var linkInverse = map[string]string{
	"relates_to": "relates_to",
	"blocks":     "blocked_by", "blocked_by": "blocks",
	"duplicates": "duplicated_by", "duplicated_by": "duplicates",
	"precedes": "follows", "follows": "precedes",
}

var storedLinkType = map[string]bool{"relates_to": true, "blocks": true, "duplicates": true, "precedes": true}

// Link is a link seen from one of its issues.
type Link struct {
	ID uuid.UUID `json:"id"`
	// Type is seen from the issue asked about: blocked_by when the other
	// issue blocks it.
	Type    string    `json:"type" enums:"relates_to,blocks,blocked_by,duplicates,duplicated_by,precedes,follows"`
	IssueID uuid.UUID `json:"issue_id"`
	Key     string    `json:"key" example:"HZN-7"`
	Title   string    `json:"title"`
	LagDays *int32    `json:"lag_days,omitempty"`
}
