package issue

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/project"
)

// What to do with the children of a moved issue.
const (
	ChildrenMove  = "move"  // the subtree moves, with the same mapping
	ChildrenLeave = "leave" // they stay; their parent is then elsewhere
)

// MoveInput moves an issue to another project (docs/api.md#moving-issues).
// Mappings override the automatic matches (same definition, else same
// name); a field mapped to uuid.Nil drops its values.
type MoveInput struct {
	Project  string
	Children string
	Types    map[uuid.UUID]uuid.UUID
	Statuses map[uuid.UUID]uuid.UUID
	Fields   map[uuid.UUID]uuid.UUID
}

// MoveMatch is one definition of the moved issues and what it becomes in
// the target project; Match is null when nothing matches (types and
// statuses must then be mapped; field values and labels are dropped).
type MoveMatch struct {
	ID     uuid.UUID  `json:"id"`
	Name   string     `json:"name"`
	Match  *uuid.UUID `json:"match"`
	Issues int        `json:"issues"`
}

// MovePreview lists what a move would map, for the confirmation dialog.
type MovePreview struct {
	Project  string `json:"project" example:"OPS"`
	Issues   int    `json:"issues"`
	Children int    `json:"children"`
	// The lists cover the issue and, with children=move, its subtree.
	Types            []MoveMatch `json:"types"`
	Statuses         []MoveMatch `json:"statuses"`
	Fields           []MoveMatch `json:"fields"`
	Labels           []MoveMatch `json:"labels"`
	EstimatesCleared int         `json:"estimates_cleared"`
}

// movePlan is the outcome of matching: per issue, its new row values.
type movePlan struct {
	preview MovePreview
	issues  []movedIssue
	missing apperr.Fields
}

type movedIssue struct {
	cur      db.Issue
	key      string
	typeID   uuid.UUID
	statusID uuid.UUID
	estimate *string
	fields   json.RawMessage
	labels   []uuid.UUID
	dropped  []string // what didn't make it, for the event
}

// PreviewMove shows the automatic matches for moving the issue (and, with
// children=move, its subtree) to the project.
func (s *Service) PreviewMove(ctx context.Context, ic Context, projectKey, children string) (MovePreview, error) {
	if children == "" {
		children = ChildrenMove
	}
	var out MovePreview
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		plan, _, err := s.planMove(ctx, tx, ic, MoveInput{Project: projectKey, Children: children})
		out = plan.preview
		return err
	})
	return out, err
}

// Move moves the issue to another project (issue.move on what moves,
// issue.create on the target): new keys from the target's sequence, the
// old ones kept as aliases; iteration and release cleared; links,
// comments and history kept.
func (s *Service) Move(ctx context.Context, ic Context, in MoveInput) (Issue, error) {
	var moved db.Issue
	var key string
	var labels []uuid.UUID
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		plan, target, err := s.planMove(ctx, tx, ic, in)
		if err != nil {
			return err
		}
		if len(plan.missing) > 0 {
			return &apperr.Error{Status: 422, Code: "mapping_required", Fields: plan.missing}
		}
		q := db.New(tx)
		// Parents first, so the rank order of a subtree is kept.
		for k, m := range plan.issues {
			if _, err := q.LockIssue(ctx, db.LockIssueParams{WorkspaceID: m.cur.WorkspaceID, ID: m.cur.ID}); err != nil {
				return err
			}
			saved, err := s.moveOne(ctx, tx, ic, target, plan, m)
			if err != nil {
				return err
			}
			if k == 0 {
				moved, key, labels = saved, keyOf(target.Project.Key, saved.Number), m.labels
			}
		}
		return nil
	})
	if err != nil {
		return Issue{}, err
	}
	i := issueFrom(moved, "", labels)
	i.Key = key
	return i, nil
}

func (s *Service) moveOne(ctx context.Context, tx pgx.Tx, ic Context, target project.Context, plan movePlan, m movedIssue) (db.Issue, error) {
	q := db.New(tx)
	srcCfg, err := s.config.Resolve(ctx, tx, m.cur.WorkspaceID, uuid.NullUUID{UUID: m.cur.ProjectID, Valid: true})
	if err != nil {
		return db.Issue{}, err
	}
	dstCfg, err := s.config.Resolve(ctx, tx, target.Workspace.ID, uuid.NullUUID{UUID: target.Project.ID, Valid: true})
	if err != nil {
		return db.Issue{}, err
	}
	n, err := q.NextIssueNumber(ctx, db.NextIssueNumberParams{WorkspaceID: target.Workspace.ID, ID: target.Project.ID})
	if err != nil {
		return db.Issue{}, err
	}
	last, err := q.LastRank(ctx, db.LastRankParams{WorkspaceID: target.Workspace.ID, ProjectID: target.Project.ID})
	if err != nil {
		return db.Issue{}, err
	}
	rank, err := Between(last, "")
	if err != nil {
		return db.Issue{}, err
	}
	started, resolved := m.cur.StartedAt, m.cur.ResolvedAt
	now := s.now()
	to, from := outcomeOf(dstCfg, m.statusID), outcomeOf(srcCfg, m.cur.StatusID)
	if to == configdef.OutcomeStarted && started == nil {
		started = &now
	}
	if configdef.Open(to) {
		resolved = nil
	} else if configdef.Open(from) || resolved == nil {
		resolved = &now
	}
	// A parent the type no longer accepts is let go (listed in the preview).
	parent := m.cur.ParentID
	if it, _ := findType(dstCfg, m.typeID); it.ParentRule == configdef.ParentNone && parent.Valid {
		parent = uuid.NullUUID{}
		m.dropped = append(m.dropped, "parent")
	}
	saved, err := q.MoveIssue(ctx, db.MoveIssueParams{
		WorkspaceID: m.cur.WorkspaceID, ID: m.cur.ID, ProjectID: target.Project.ID, Number: n, TypeID: m.typeID, StatusID: m.statusID,
		Rank: rank, Estimate: m.estimate, Fields: m.fields, ParentID: parent, StartedAt: started, ResolvedAt: resolved, UpdatedAt: now,
	})
	if err != nil {
		return db.Issue{}, err
	}
	if err := q.AddIssueAlias(ctx, db.AddIssueAliasParams{WorkspaceID: m.cur.WorkspaceID, ProjectID: m.cur.ProjectID, Number: m.cur.Number,
		IssueID: m.cur.ID, CreatedAt: now}); err != nil {
		return db.Issue{}, err
	}
	if err := q.ClearIssueLabels(ctx, m.cur.ID); err != nil {
		return db.Issue{}, err
	}
	for _, l := range m.labels {
		if err := q.AddIssueLabel(ctx, db.AddIssueLabelParams{WorkspaceID: m.cur.WorkspaceID, IssueID: m.cur.ID, LabelID: l}); err != nil {
			return db.Issue{}, err
		}
	}
	payload := map[string]any{
		"from_project_id": m.cur.ProjectID, "to_project_id": target.Project.ID, "from_key": m.key, "to_key": keyOf(target.Project.Key, n),
		"type_id": m.typeID, "status_id": m.statusID,
	}
	if len(m.dropped) > 0 {
		payload["dropped"] = m.dropped
	}
	return saved, eventlog.Append(ctx, tx, s.event(ic.Context.Context, saved, "IssueMoved", payload))
}

// planMove checks permissions and matches the configuration of what moves
// against the target project's.
func (s *Service) planMove(ctx context.Context, tx pgx.Tx, ic Context, in MoveInput) (movePlan, project.Context, error) {
	var plan movePlan
	target, err := s.projects.Enter(ctx, ic.Context.Context, in.Project)
	if err != nil {
		return plan, target, apperr.Invalid(apperr.Field{Field: "project", Code: "unknown"})
	}
	if target.Project.ID == ic.Issue.ProjectID {
		return plan, target, apperr.Invalid(apperr.Field{Field: "project", Code: "same"})
	}
	if target.Project.ArchivedAt != nil {
		return plan, target, apperr.Conflict("project_archived")
	}
	if err := s.require(ctx, ic.Context, access.IssueMove); err != nil {
		return plan, target, err
	}
	if err := s.require(ctx, target, access.IssueCreate); err != nil {
		return plan, target, err
	}
	if ic.Issue.ArchivedAt != nil {
		return plan, target, apperr.Conflict("issue_archived")
	}
	q := db.New(tx)
	issues := []movedIssue{{cur: ic.Issue, key: ic.key()}}
	subtree, err := q.Descendants(ctx, uuid.NullUUID{UUID: ic.Issue.ID, Valid: true})
	if err != nil {
		return plan, target, err
	}
	plan.preview.Children = len(subtree)
	if len(subtree) > 0 {
		switch in.Children {
		case ChildrenMove:
			// Shallowest first: parents move before their children.
			for k := len(subtree) - 1; k >= 0; k-- {
				dic, err := s.enterByID(ctx, ic.Context.Context, subtree[k])
				if err != nil {
					return plan, target, apperr.Forbidden("forbidden")
				}
				if err := s.require(ctx, dic.Context, access.IssueMove); err != nil {
					return plan, target, err
				}
				issues = append(issues, movedIssue{cur: dic.Issue, key: dic.key()})
			}
		case ChildrenLeave:
		default:
			plan.missing.Add("children", "required")
		}
	}
	plan.preview.Project = target.Project.Key
	plan.preview.Issues = len(issues)

	dst, err := s.config.Resolve(ctx, tx, target.Workspace.ID, uuid.NullUUID{UUID: target.Project.ID, Valid: true})
	if err != nil {
		return plan, target, err
	}
	srcByProject := map[uuid.UUID]*configdef.Resolved{}
	types, statuses, fields, labels := matches{}, matches{}, matches{}, matches{}
	for k := range issues {
		m := &issues[k]
		src, ok := srcByProject[m.cur.ProjectID]
		if !ok {
			if src, err = s.config.Resolve(ctx, tx, m.cur.WorkspaceID, uuid.NullUUID{UUID: m.cur.ProjectID, Valid: true}); err != nil {
				return plan, target, err
			}
			srcByProject[m.cur.ProjectID] = src
		}

		// Type, then a status of its workflow.
		srcType, _ := findType(src, m.cur.TypeID)
		tt, ok := matchType(dst, srcType, in.Types)
		types.add(srcType.ID, srcType.Name, tt.ID, ok)
		srcStatus, _, _ := src.Status(m.cur.StatusID)
		if !ok {
			plan.missing.Add("mapping.types."+srcType.ID.String(), "required")
			statuses.add(srcStatus.ID, srcStatus.Name, uuid.Nil, false)
			continue
		}
		m.typeID = tt.ID
		st, ok := matchStatus(dst.Workflows[tt.WorkflowID], srcStatus, in.Statuses)
		statuses.add(srcStatus.ID, srcStatus.Name, st, ok)
		if !ok {
			plan.missing.Add("mapping.statuses."+srcStatus.ID.String(), "required")
			continue
		}
		m.statusID = st

		// Estimate: kept when the target scale has it.
		if m.cur.Estimate != nil {
			if dst.Estimation.Kind != configdef.ScaleNone && slices.ContainsFunc(dst.Estimation.Values, func(v configdef.ScaleValue) bool { return v.Label == *m.cur.Estimate }) {
				m.estimate = m.cur.Estimate
			} else {
				plan.preview.EstimatesCleared++
				m.dropped = append(m.dropped, "estimate")
			}
		}

		// Custom field values, onto the fields of the target type.
		values := map[string]json.RawMessage{}
		_ = json.Unmarshal(m.cur.Fields, &values)
		out := map[string]json.RawMessage{}
		for key, raw := range values {
			id, err := uuid.Parse(key)
			if err != nil {
				continue
			}
			f := src.Fields[id]
			tf, ok := matchField(dst, tt.ID, f, in.Fields)
			fields.add(f.ID, f.Name, tf.ID, ok)
			if !ok {
				m.dropped = append(m.dropped, "fields."+key)
				continue
			}
			if v, ok := convertValue(f, tf, raw); ok {
				out[tf.ID.String()] = v
			} else {
				m.dropped = append(m.dropped, "fields."+key)
			}
		}
		m.fields, _ = json.Marshal(out)

		// Labels: the same label when the target sees it, else one of the
		// same name.
		cur, err := s.labelsOf(ctx, tx, []uuid.UUID{m.cur.ID})
		if err != nil {
			return plan, target, err
		}
		m.labels = []uuid.UUID{}
		for _, id := range cur[m.cur.ID] {
			l := src.Labels[id]
			to, ok := matchLabel(dst, l)
			labels.add(l.ID, l.Name, to, ok)
			if ok && !slices.Contains(m.labels, to) {
				m.labels = append(m.labels, to)
			} else if !ok {
				m.dropped = append(m.dropped, "labels."+id.String())
			}
		}
	}
	plan.issues = issues
	plan.preview.Types, plan.preview.Statuses = types.list(), statuses.list()
	plan.preview.Fields, plan.preview.Labels = fields.list(), labels.list()
	return plan, target, nil
}

func matchType(dst *configdef.Resolved, src configdef.IssueType, mapping map[uuid.UUID]uuid.UUID) (configdef.IssueType, bool) {
	if id, ok := mapping[src.ID]; ok {
		return findType(dst, id)
	}
	if it, ok := findType(dst, src.ID); ok {
		return it, true
	}
	for _, it := range dst.IssueTypes {
		if strings.EqualFold(it.Name, src.Name) {
			return it, true
		}
	}
	return configdef.IssueType{}, false
}

func matchStatus(wf configdef.Workflow, src configdef.Status, mapping map[uuid.UUID]uuid.UUID) (uuid.UUID, bool) {
	in := func(id uuid.UUID) bool {
		return slices.ContainsFunc(wf.Statuses, func(st configdef.Status) bool { return st.ID == id })
	}
	if id, ok := mapping[src.ID]; ok {
		return id, in(id)
	}
	if in(src.ID) {
		return src.ID, true
	}
	for _, st := range wf.Statuses {
		if strings.EqualFold(st.Name, src.Name) {
			return st.ID, true
		}
	}
	return uuid.Nil, false
}

// matchField finds where a value goes: the mapped field, the same field,
// or one of the same name and type — attached to the target type.
func matchField(dst *configdef.Resolved, typeID uuid.UUID, src configdef.Field, mapping map[uuid.UUID]uuid.UUID) (configdef.Field, bool) {
	attached := func(id uuid.UUID) (configdef.Field, bool) {
		f, ok := dst.Fields[id]
		if !ok || f.Type != src.Type || !slices.ContainsFunc(dst.TypeFields[typeID], func(tf configdef.TypeField) bool { return tf.FieldID == id }) {
			return configdef.Field{}, false
		}
		return f, true
	}
	if id, ok := mapping[src.ID]; ok {
		if id == uuid.Nil {
			return configdef.Field{}, false
		}
		return attached(id)
	}
	if f, ok := attached(src.ID); ok {
		return f, true
	}
	for _, tf := range dst.TypeFields[typeID] {
		if f := dst.Fields[tf.FieldID]; strings.EqualFold(f.Name, src.Name) && f.Type == src.Type {
			return f, true
		}
	}
	return configdef.Field{}, false
}

// convertValue carries a value to another field of the same type: select
// options go by name.
func convertValue(from, to configdef.Field, raw json.RawMessage) (json.RawMessage, bool) {
	if from.ID == to.ID || (from.Type != "select" && from.Type != "multi_select") {
		return raw, true
	}
	option := func(id string) (string, bool) {
		for _, o := range from.Options {
			if o.ID.String() == id {
				for _, p := range to.Options {
					if strings.EqualFold(p.Name, o.Name) {
						return p.ID.String(), true
					}
				}
			}
		}
		return "", false
	}
	if from.Type == "select" {
		var v string
		if json.Unmarshal(raw, &v) != nil {
			return nil, false
		}
		id, ok := option(v)
		if !ok {
			return nil, false
		}
		out, _ := json.Marshal(id)
		return out, true
	}
	var vs []string
	if json.Unmarshal(raw, &vs) != nil {
		return nil, false
	}
	ids := []string{}
	for _, v := range vs {
		if id, ok := option(v); ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, false
	}
	out, _ := json.Marshal(ids)
	return out, true
}

func matchLabel(dst *configdef.Resolved, src configdef.Label) (uuid.UUID, bool) {
	if _, ok := dst.Labels[src.ID]; ok {
		return src.ID, true
	}
	for _, l := range dst.Labels {
		if strings.EqualFold(l.Name, src.Name) {
			return l.ID, true
		}
	}
	return uuid.Nil, false
}

// matches collects MoveMatch rows by source id.
type matches map[uuid.UUID]*MoveMatch

func (ms matches) add(id uuid.UUID, name string, to uuid.UUID, ok bool) {
	m, seen := ms[id]
	if !seen {
		m = &MoveMatch{ID: id, Name: name}
		if ok {
			m.Match = &to
		}
		ms[id] = m
	}
	m.Issues++
}

func (ms matches) list() []MoveMatch {
	out := make([]MoveMatch, 0, len(ms))
	for _, m := range ms {
		out = append(out, *m)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}
