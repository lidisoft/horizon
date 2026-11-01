package issue

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/lidisoft/horizon/internal/query"
	"github.com/lidisoft/horizon/internal/workspace"
)

// The workspace roadmap (docs/frontend.md#roadmap): top-level issues with
// children, across projects, drawn over their own dates or their
// children's span.

// roadmapWhere keeps issues without a parent that have children.
const roadmapWhere = `i.parent_id IS NULL AND EXISTS (SELECT 1 FROM issues c WHERE c.parent_id = i.id AND c.archived_at IS NULL)`

// Children sums up an issue's children (archived ones left out).
type Children struct {
	Count int `json:"count"`
	// Completed and Abandoned count children by their status's outcome.
	Completed int `json:"completed"`
	Abandoned int `json:"abandoned"`
	// StartDate and DueDate span the children's dates.
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
}

// RoadmapItem is a top-level issue and its bar.
type RoadmapItem struct {
	Issue    Issue    `json:"issue"`
	Children Children `json:"children"`
	// StartDate and DueDate are the bar: the issue's own, else its children's.
	StartDate *string `json:"start_date"`
	DueDate   *string `json:"due_date"`
	// Derived is true when the bar comes from the children's dates.
	Derived bool `json:"derived"`
}

// Roadmap is a page of the roadmap.
type Roadmap struct {
	Items []RoadmapItem `json:"items"`
	Total int           `json:"total"`
	Next  string        `json:"next_cursor,omitempty"`
}

// Roadmap lists the top-level issues with children the query matches,
// in the projects named (none: all the caller sees).
func (s *Service) Roadmap(ctx context.Context, wc workspace.Context, q query.Query, projects []string, cursor string) (Roadmap, error) {
	if len(q.Sort) == 0 {
		q.Sort = []query.Sort{{Field: "rank"}}
	}
	res, err := s.query(ctx, wc, q, projects, cursor, roadmapWhere)
	if err != nil {
		return Roadmap{}, err
	}
	out := Roadmap{Items: make([]RoadmapItem, 0, len(res.Items)), Total: res.Total, Next: res.Next}
	if len(res.Items) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(res.Items))
	for k, i := range res.Items {
		ids[k] = i.ID
	}
	rows, err := s.pool.Query(ctx, `
		SELECT c.parent_id, count(*), count(*) FILTER (WHERE sc.outcome = 'completed'), count(*) FILTER (WHERE sc.outcome = 'abandoned'),
		       min(coalesce(c.start_date, c.due_date)), max(coalesce(c.due_date, c.start_date))
		FROM issues c JOIN statuses s ON s.id = c.status_id JOIN status_categories sc ON sc.id = s.category_id
		WHERE c.workspace_id = $1 AND c.parent_id = ANY($2::uuid[]) AND c.archived_at IS NULL
		GROUP BY c.parent_id`, wc.Workspace.ID, ids)
	if err != nil {
		return Roadmap{}, err
	}
	defer rows.Close()
	children := map[uuid.UUID]Children{}
	for rows.Next() {
		var id uuid.UUID
		var c Children
		var from, to pgtype.Date
		if err := rows.Scan(&id, &c.Count, &c.Completed, &c.Abandoned, &from, &to); err != nil {
			return Roadmap{}, err
		}
		c.StartDate, c.DueDate = dateString(from), dateString(to)
		children[id] = c
	}
	if err := rows.Err(); err != nil {
		return Roadmap{}, err
	}
	for _, i := range res.Items {
		item := RoadmapItem{Issue: i, Children: children[i.ID], StartDate: i.StartDate, DueDate: i.DueDate}
		if item.StartDate == nil && item.DueDate == nil {
			item.StartDate, item.DueDate, item.Derived = item.Children.StartDate, item.Children.DueDate, item.Children.StartDate != nil
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}
