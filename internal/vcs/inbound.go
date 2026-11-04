package vcs

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/vcs/db"
)

// Link kinds.
const (
	KindBranch      = "branch"
	KindPullRequest = "pull_request"
	KindCommit      = "commit"
)

// maxCommits bounds the commits of one push that are looked at.
const maxCommits = 100

// keyPattern finds issue keys in branch names, titles and messages, in any
// case ("hzn-42-fix", "Fixes HZN-42").
var keyPattern = regexp.MustCompile(`(?i)\b([a-z][a-z0-9]{1,9})-([1-9][0-9]{0,8})\b`)

type key struct {
	project string
	number  int64
}

// keys lists the distinct issue keys in the texts, in order.
func keys(texts ...string) []key {
	var out []key
	seen := map[key]bool{}
	for _, t := range texts {
		for _, m := range keyPattern.FindAllStringSubmatch(t, -1) {
			n, _ := strconv.ParseInt(m[2], 10, 64)
			k := key{strings.ToUpper(m[1]), n}
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}

// target is an issue a delivery names.
type target struct {
	id, project uuid.UUID
	version     int32
}

// Receive handles an inbound delivery for a connection: it checks the
// signature, then links what the delivery names and moves the issues of a
// pull request that merged. A delivery Horizon has no use for is accepted
// and dropped.
func (s *Service) Receive(ctx context.Context, connectionID uuid.UUID, h http.Header, body []byte) error {
	q := db.New(s.o.Pool)
	c, err := q.ConnectionByID(ctx, connectionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("connection_not_found")
	}
	if err != nil {
		return err
	}
	secret, err := s.o.Secrets.Open(c.WebhookSecret, secretAD(c.ID))
	if err != nil {
		return err
	}
	if verify(c.Provider, secret, h, body) != nil {
		return apperr.Unauthorized("bad_signature")
	}
	d, err := parse(c.Provider, eventName(c.Provider, h), body)
	if errors.Is(err, errIgnored) {
		return nil
	}
	if err != nil {
		return apperr.BadRequest("unreadable delivery")
	}
	if err := q.TouchConnection(ctx, db.TouchConnectionParams{ID: c.ID, At: s.now()}); err != nil {
		return err
	}
	if d.Ping || d.Repository == "" {
		return nil
	}
	var merged []target
	err = postgres.InTx(ctx, s.o.Pool, func(tx pgx.Tx) error {
		r := &recorder{s: s, tx: tx, q: db.New(tx), c: c, d: d, resolved: map[key]*target{}}
		if d.Branch != "" {
			if err := r.branch(ctx); err != nil {
				return err
			}
		}
		for i, cm := range d.Commits {
			if i == maxCommits {
				break
			}
			if err := r.commit(ctx, cm); err != nil {
				return err
			}
		}
		if d.PullRequest != nil {
			var err error
			if merged, err = r.pullRequest(ctx); err != nil {
				return err
			}
		}
		return eventlog.Append(ctx, tx, r.events...)
	})
	if err != nil {
		return err
	}
	for _, t := range merged {
		if err := s.moveMerged(ctx, c.WorkspaceID, t, d.PullRequest.URL); err != nil {
			return err
		}
	}
	return nil
}

// moveMerged applies the project's merge setting to an issue.
func (s *Service) moveMerged(ctx context.Context, workspaceID uuid.UUID, t target, url string) error {
	st, err := s.o.Config.GitSettings(ctx, s.o.Pool, configdef.Target{WorkspaceID: workspaceID, ProjectID: uuid.NullUUID{UUID: t.project, Valid: true}})
	if err != nil {
		return err
	}
	if st.MergeStatus.Value == configdef.MergeOff {
		return nil
	}
	_, err = s.o.Issues.SystemTransition(ctx, workspaceID, t.id, st.MergeStatus.Value, map[string]any{"by": "pull_request_merged", "url": url})
	return err
}

// recorder writes one delivery's links and their events.
type recorder struct {
	s        *Service
	tx       pgx.Tx
	q        *db.Queries
	c        db.VcsConnection
	d        delivery
	resolved map[key]*target
	events   []eventlog.Event
}

func (r *recorder) resolve(ctx context.Context, ks []key) ([]target, error) {
	var out []target
	for _, k := range ks {
		t, ok := r.resolved[k]
		if !ok {
			row, err := r.q.ResolveKey(ctx, db.ResolveKeyParams{WorkspaceID: r.c.WorkspaceID, ProjectKey: k.project, Number: k.number})
			switch {
			case errors.Is(err, pgx.ErrNoRows):
				t = nil
			case err != nil:
				return nil, err
			default:
				t = &target{id: row.ID, project: row.ProjectID, version: row.Version}
			}
			r.resolved[k] = t
		}
		if t != nil {
			out = append(out, *t)
		}
	}
	return out, nil
}

// link records a link; it notes a new one, or a changed state, in the
// issue's history. prev is the state before ("" for a new link).
func (r *recorder) link(ctx context.Context, t target, kind, ref, url, title, state, author string) (prev string, inserted bool, err error) {
	prev, err = r.q.LinkState(ctx, db.LinkStateParams{IssueID: t.id, ConnectionID: r.c.ID, Kind: kind, Repository: r.d.Repository, ExternalID: ref})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	row, err := r.q.UpsertLink(ctx, db.UpsertLinkParams{WorkspaceID: r.c.WorkspaceID, ID: uuid.Must(uuid.NewV7()), IssueID: t.id, ConnectionID: r.c.ID,
		Kind: kind, Repository: r.d.Repository, ExternalID: ref, Url: url, Title: title, State: state, Author: author, At: r.s.now()})
	if err != nil {
		return "", false, err
	}
	payload := map[string]any{"kind": kind, "provider": r.c.Provider, "repository": r.d.Repository, "ref": ref, "title": title, "url": url, "state": state}
	switch {
	case row.Inserted:
		r.events = append(r.events, r.event(t, "IssueGitLinked", payload))
	case prev != state:
		payload["from"] = prev
		r.events = append(r.events, r.event(t, "IssueGitUpdated", payload))
	}
	return prev, row.Inserted, nil
}

func (r *recorder) event(t target, typ string, payload map[string]any) eventlog.Event {
	return eventlog.Event{
		WorkspaceID: uuid.NullUUID{UUID: r.c.WorkspaceID, Valid: true}, AggregateType: "issue", AggregateID: t.id,
		AggregateVersion: int64(t.version), Type: typ, OccurredAt: r.s.now(), Payload: eventlog.Payload(payload),
	}
}

// branch links the issues a branch's name gives, or marks them deleted.
func (r *recorder) branch(ctx context.Context) error {
	ts, err := r.resolve(ctx, keys(r.d.Branch))
	if err != nil {
		return err
	}
	state := StateOpen
	if r.d.BranchDeleted {
		state = StateDeleted
	}
	for _, t := range ts {
		if _, _, err := r.link(ctx, t, KindBranch, r.d.Branch, branchURL(r.c.Provider, r.d.RepoURL, r.d.Branch), r.d.Branch, state, ""); err != nil {
			return err
		}
	}
	return nil
}

// branchURL is the forge's page of a branch.
func branchURL(provider, repo, branch string) string {
	switch provider {
	case GitLab:
		return repo + "/-/tree/" + branch
	case Forgejo:
		return repo + "/src/branch/" + branch
	}
	return repo + "/tree/" + branch
}

// commit links the issues a commit message names.
func (r *recorder) commit(ctx context.Context, cm commit) error {
	ts, err := r.resolve(ctx, keys(cm.Message))
	if err != nil {
		return err
	}
	title, _, _ := strings.Cut(strings.TrimSpace(cm.Message), "\n")
	for _, t := range ts {
		if _, _, err := r.link(ctx, t, KindCommit, cm.SHA, cm.URL, title, "", cm.Author); err != nil {
			return err
		}
	}
	return nil
}

// pullRequest links the issues a pull request names (title, description,
// branch) and keeps the ones it named before; it returns those whose link
// just turned merged, and asks for the comment to be written (the job
// skips it when its text didn't change).
func (r *recorder) pullRequest(ctx context.Context) ([]target, error) {
	pr := r.d.PullRequest
	number := strconv.Itoa(pr.Number)
	ts, err := r.resolve(ctx, keys(pr.Title, pr.Body, pr.Branch))
	if err != nil {
		return nil, err
	}
	before, err := r.q.RefLinks(ctx, db.RefLinksParams{ConnectionID: r.c.ID, Kind: KindPullRequest, Repository: r.d.Repository, ExternalID: number})
	if err != nil {
		return nil, err
	}
	seen := map[uuid.UUID]bool{}
	for _, t := range ts {
		seen[t.id] = true
	}
	for _, b := range before {
		if !seen[b.IssueID] {
			row, err := r.q.IssueRef(ctx, b.IssueID)
			if err != nil {
				return nil, err
			}
			ts = append(ts, target{id: row.ID, project: row.ProjectID, version: row.Version})
			seen[b.IssueID] = true
		}
	}
	var merged []target
	for _, t := range ts {
		prev, _, err := r.link(ctx, t, KindPullRequest, number, pr.URL, pr.Title, pr.State, pr.Author)
		if err != nil {
			return nil, err
		}
		if pr.State == StateMerged && prev != StateMerged {
			merged = append(merged, t)
		}
	}
	if len(ts) > 0 && r.c.Token != nil {
		if err := r.q.MarkComment(ctx, db.MarkCommentParams{WorkspaceID: r.c.WorkspaceID, ConnectionID: r.c.ID, Repository: r.d.Repository,
			Number: number, ProjectRef: r.d.ProjectRef, At: r.s.now()}); err != nil {
			return nil, err
		}
	}
	return merged, nil
}
