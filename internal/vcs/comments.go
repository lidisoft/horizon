package vcs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/vcs/db"
)

// Comment writing limits.
const (
	commentLease    = time.Minute
	commentBatch    = 10
	maxCommentTries = 8
)

// commentRetries are the delays after each failure (the last repeats).
var commentRetries = []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// Consumer is the durable vcs consumer: issue changes a pull request's
// comment shows (status, title, key) mark the comment for rewriting.
func (s *Service) Consumer() eventlog.Consumer {
	return eventlog.Consumer{Name: "vcs", Filter: []string{"issue.IssueStatusUpdated", "issue.IssueUpdated", "issue.IssueMoved"}, Handle: s.Handle}
}

// Handle marks the comments of the issue's pull requests dirty.
func (s *Service) Handle(ctx context.Context, e eventlog.Event) error {
	if !e.WorkspaceID.Valid {
		return nil
	}
	q := db.New(s.o.Pool)
	prs, err := q.PullRequestsOf(ctx, e.AggregateID)
	if err != nil {
		return err
	}
	for _, pr := range prs {
		if err := q.MarkComment(ctx, db.MarkCommentParams{WorkspaceID: e.WorkspaceID.UUID, ConnectionID: pr.ConnectionID, Repository: pr.Repository,
			Number: pr.ExternalID, At: s.now()}); err != nil {
			return err
		}
	}
	return nil
}

// WriteComments writes the comments due (a periodic job): each pull request
// gets one comment listing its issues and their status, created once and
// then edited. A comment someone deleted on the forge is posted again.
// Leased like webhook deliveries, so every worker may run it.
func (s *Service) WriteComments(ctx context.Context) error {
	now := s.now()
	q := db.New(s.o.Pool)
	due, err := q.LeaseComments(ctx, db.LeaseCommentsParams{Now: now, LeaseUntil: now.Add(commentLease), MaxRows: commentBatch})
	if err != nil {
		return err
	}
	for _, cm := range due {
		if err := s.writeComment(ctx, cm, now); err != nil {
			s.o.Log.Warn().Err(err).Str("repository", cm.Repository).Str("number", cm.Number).Msg("could not write a pull request comment")
			retry := cm.Attempts+1 < maxCommentTries
			next := now.Add(commentRetries[min(int(cm.Attempts), len(commentRetries)-1)])
			if err := q.CommentFailed(context.WithoutCancel(ctx), db.CommentFailedParams{ConnectionID: cm.ConnectionID, Repository: cm.Repository,
				Number: cm.Number, Error: err.Error(), NextAttemptAt: next, Retry: retry}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) writeComment(ctx context.Context, cm db.VcsComment, leasedAt time.Time) error {
	q := db.New(s.o.Pool)
	c, err := q.ConnectionByID(ctx, cm.ConnectionID)
	if err != nil {
		return err
	}
	done := func(forgeID *string, body string) error {
		return q.CommentWritten(context.WithoutCancel(ctx), db.CommentWrittenParams{ConnectionID: cm.ConnectionID, Repository: cm.Repository,
			Number: cm.Number, ForgeID: forgeID, Body: body, LeasedAt: leasedAt})
	}
	if c.Token == nil {
		return done(cm.ForgeID, cm.Body) // no token any more: nothing to write with
	}
	token, err := s.o.Secrets.Open(*c.Token, tokenAD(c.ID))
	if err != nil {
		return err
	}
	issues, err := q.CommentIssues(ctx, db.CommentIssuesParams{ConnectionID: cm.ConnectionID, Repository: cm.Repository, Number: cm.Number})
	if err != nil {
		return err
	}
	body := s.commentBody(issues)
	if cm.ForgeID != nil && body == cm.Body {
		return done(cm.ForgeID, body)
	}
	if cm.ForgeID == nil && len(issues) == 0 {
		return done(nil, cm.Body)
	}
	api := forgeAPI{client: s.o.Client, provider: c.Provider, base: apiBase(c.Provider, c.BaseUrl), token: token}
	id, err := api.upsertComment(ctx, cm, body)
	if err != nil {
		return err
	}
	return done(&id, body)
}

// Marker opens every comment Horizon writes, so people know where it comes
// from and that it changes.
const Marker = "<!-- horizon:issues -->"

// commentBody lists the issues in Markdown, with links back to Horizon.
func (s *Service) commentBody(issues []db.CommentIssuesRow) string {
	var b strings.Builder
	b.WriteString(Marker + "\n")
	if len(issues) == 0 {
		b.WriteString("**Horizon**: this pull request names no issue any more.\n")
		return b.String()
	}
	b.WriteString("**Horizon** issues\n\n| Issue | Status |\n| --- | --- |\n")
	base := ""
	if s.o.BaseURL != nil {
		base = strings.TrimRight(s.o.BaseURL.String(), "/")
	}
	for _, i := range issues {
		k := issue.KeyOf(i.ProjectKey, i.Number)
		title := strings.NewReplacer("|", `\|`, "\n", " ", "[", `\[`, "]", `\]`).Replace(i.Title)
		fmt.Fprintf(&b, "| [%s](%s/w/%s/issues/%s) %s | %s |\n", k, base, i.WorkspaceSlug, k, title, i.StatusName)
	}
	b.WriteString("\n<sub>Kept up to date by Horizon as the issues change.</sub>\n")
	return b.String()
}

// apiBase is the forge's REST API root.
func apiBase(provider, base string) string {
	switch provider {
	case GitLab:
		return base + "/api/v4"
	case Forgejo:
		return base + "/api/v1"
	}
	if u, err := url.Parse(base); err == nil && (u.Host == "github.com" || u.Host == "www.github.com") {
		return "https://api.github.com"
	}
	return base + "/api/v3" // GitHub Enterprise Server
}

// forgeAPI writes comments through a forge's REST API.
type forgeAPI struct {
	client   *http.Client
	provider string
	base     string
	token    string
}

var errNotFound = errors.New("vcs: not found on the forge")

// upsertComment edits the comment, or posts it when there is none (or it
// was deleted), and returns its id.
func (a forgeAPI) upsertComment(ctx context.Context, cm db.VcsComment, body string) (string, error) {
	if cm.ForgeID != nil {
		err := a.send(ctx, a.editMethod(), a.commentURL(cm, *cm.ForgeID), body, nil)
		if err == nil {
			return *cm.ForgeID, nil
		}
		if !errors.Is(err, errNotFound) {
			return "", err
		}
	}
	var created struct {
		ID json.Number `json:"id"`
	}
	if err := a.send(ctx, http.MethodPost, a.commentsURL(cm), body, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("vcs: the forge answered no comment id")
	}
	return created.ID.String(), nil
}

func (a forgeAPI) editMethod() string {
	if a.provider == GitLab {
		return http.MethodPut
	}
	return http.MethodPatch
}

func (a forgeAPI) project(cm db.VcsComment) string {
	if cm.ProjectRef != "" {
		return cm.ProjectRef
	}
	return url.PathEscape(cm.Repository)
}

func (a forgeAPI) commentsURL(cm db.VcsComment) string {
	if a.provider == GitLab {
		return a.base + "/projects/" + a.project(cm) + "/merge_requests/" + cm.Number + "/notes"
	}
	return a.base + "/repos/" + cm.Repository + "/issues/" + cm.Number + "/comments"
}

func (a forgeAPI) commentURL(cm db.VcsComment, id string) string {
	if a.provider == GitLab {
		return a.commentsURL(cm) + "/" + id
	}
	return a.base + "/repos/" + cm.Repository + "/issues/comments/" + id
}

func (a forgeAPI) send(ctx context.Context, method, target, body string, out any) error {
	payload, _ := json.Marshal(map[string]string{"body": body})
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	switch a.provider {
	case GitLab:
		req.Header.Set("PRIVATE-TOKEN", a.token)
	case Forgejo:
		req.Header.Set("Authorization", "token "+a.token)
	default:
		req.Header.Set("Authorization", "Bearer "+a.token)
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return errNotFound
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fmt.Errorf("vcs: %s %s: %s %s", method, target, strconv.Itoa(resp.StatusCode), strings.TrimSpace(string(b[:min(len(b), 200)])))
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
