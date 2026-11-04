package notify

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/notify/db"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
)

// Emails are English only for now (ADR-0017).

// line is an inbox row as an email tells it.
type line struct {
	workspace, key, title, reason string
	count                         int
	event                         *issue.PublicEvent
}

// SendDue emails each person their rows unread for EmailDelay that no
// email told of yet, one email per person (a periodic job). Every worker
// may run it: rows are claimed with SKIP LOCKED, and marked once sent, so
// a crash between the two sends them again (at least once).
func (s *Service) SendDue(ctx context.Context) error {
	if s.o.Mail == nil {
		return nil
	}
	now := s.now()
	users, err := db.New(s.o.Pool).DueEmailUsers(ctx, db.DueEmailUsersParams{QuietSince: now.Add(-EmailDelay), Now: now, MaxRows: 100})
	if err != nil {
		return err
	}
	for _, id := range users {
		if err := s.sendDue(ctx, id, now); err != nil {
			s.o.Log.Warn().Err(err).Stringer("user", id).Msg("could not email notifications")
		}
	}
	return nil
}

func (s *Service) sendDue(ctx context.Context, userID uuid.UUID, now time.Time) error {
	tx, err := s.o.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(s.o.Pool).WithTx(tx)
	rows, err := q.ClaimDueEmails(ctx, db.ClaimDueEmailsParams{UserID: userID, QuietSince: now.Add(-EmailDelay), Now: now})
	if err != nil || len(rows) == 0 {
		return err
	}
	ids := make([]uuid.UUID, len(rows))
	items := make([]db.InboxItem, len(rows))
	refs := make([]itemRef, len(rows))
	for i, r := range rows {
		ids[i], items[i] = r.InboxItem.ID, r.InboxItem
		refs[i] = itemRef{slug: r.WorkspaceSlug, key: issue.KeyOf(r.ProjectKey, r.Number), title: r.Title}
	}
	u, lines, err := s.lines(ctx, userID, items, refs)
	if err != nil {
		return err
	}
	if len(lines) > 0 {
		if err := s.o.Mail.Send(ctx, s.message(u, lines, false)); err != nil {
			return err
		}
	}
	// Rows no longer visible are marked too: they won't be told of.
	if err := q.MarkEmailed(ctx, ids); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SendDigests emails the daily digest to the people who take it, once a
// day from DigestHour in their time zone (UTC when unset): their unread
// rows, every workspace (a periodic job).
func (s *Service) SendDigests(ctx context.Context) error {
	if s.o.Mail == nil {
		return nil
	}
	now := s.now()
	q := db.New(s.o.Pool)
	// Anyone not sent one in the last 20 hours may be due.
	due, err := q.DigestUsers(ctx, now.Add(-20*time.Hour))
	if err != nil {
		return err
	}
	for _, d := range due {
		morning := digestMorning(now, d.Timezone)
		if now.Before(morning) {
			continue
		}
		if d.DigestSentAt != nil && !d.DigestSentAt.Before(morning) {
			continue
		}
		if err := s.sendDigest(ctx, d.ID, now, morning); err != nil {
			s.o.Log.Warn().Err(err).Stringer("user", d.ID).Msg("could not email the digest")
		}
	}
	return nil
}

// digestMorning is today's DigestHour in the zone (UTC when unknown).
func digestMorning(now time.Time, zone *string) time.Time {
	loc := time.UTC
	if zone != nil {
		if l, err := time.LoadLocation(*zone); err == nil {
			loc = l
		}
	}
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), DigestHour, 0, 0, 0, loc)
}

func (s *Service) sendDigest(ctx context.Context, userID uuid.UUID, now, morning time.Time) error {
	tx, err := s.o.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(s.o.Pool).WithTx(tx)
	if _, err := q.ClaimDigest(ctx, db.ClaimDigestParams{UserID: userID, Now: now, SentBefore: morning}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // another worker took it
		}
		return err
	}
	rows, err := q.DigestItems(ctx, db.DigestItemsParams{UserID: userID, Now: now})
	if err != nil {
		return err
	}
	items := make([]db.InboxItem, len(rows))
	refs := make([]itemRef, len(rows))
	for i, r := range rows {
		items[i] = r.InboxItem
		refs[i] = itemRef{slug: r.WorkspaceSlug, key: issue.KeyOf(r.ProjectKey, r.Number), title: r.Title}
	}
	u, lines, err := s.lines(ctx, userID, items, refs)
	if err != nil {
		return err
	}
	if len(lines) > 0 {
		if err := s.o.Mail.Send(ctx, s.message(u, lines, true)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

type itemRef struct{ slug, key, title string }

// lines describes the rows the user still sees.
func (s *Service) lines(ctx context.Context, userID uuid.UUID, items []db.InboxItem, refs []itemRef) (identity.User, []line, error) {
	p, ok, err := s.principal(ctx, userID)
	if err != nil || !ok {
		return identity.User{}, nil, err
	}
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.EventID
	}
	events, err := s.describe(ctx, ids)
	if err != nil {
		return identity.User{}, nil, err
	}
	var out []line
	for i, it := range items {
		ok, err := s.sees(ctx, userID, refs[i].slug, refs[i].key)
		if err != nil {
			return identity.User{}, nil, err
		}
		if !ok {
			continue
		}
		l := line{workspace: refs[i].slug, key: refs[i].key, title: refs[i].title, reason: it.Reason, count: int(it.UnreadCount)}
		if e, ok := events[it.EventID]; ok {
			l.event = &e
		}
		out = append(out, l)
	}
	return p.User, out, nil
}

// message writes the email of the lines.
func (s *Service) message(u identity.User, lines []line, digest bool) platformmail.Message {
	var b strings.Builder
	fmt.Fprintf(&b, "Hello %s,\n\n", u.Name)
	if digest {
		b.WriteString("Here is what is waiting in your Horizon inbox.\n\n")
	}
	for _, l := range lines {
		fmt.Fprintf(&b, "%s %s\n", l.key, l.title)
		if l.event != nil {
			fmt.Fprintf(&b, "  %s\n", Summary(*l.event, u.ID, l.reason))
		}
		if l.count > 1 {
			fmt.Fprintf(&b, "  and %d more updates\n", l.count-1)
		}
		fmt.Fprintf(&b, "  %s\n\n", s.link("/w/"+l.workspace+"/issues/"+l.key))
	}
	fmt.Fprintf(&b, "Your inbox: %s\n", s.link("/w/"+lines[0].workspace+"/inbox"))
	fmt.Fprintf(&b, "Choose what Horizon emails you: %s\n", s.link("/account"))
	subject := fmt.Sprintf("%s %s", lines[0].key, lines[0].title)
	switch {
	case digest:
		subject = fmt.Sprintf("Your Horizon digest: %d unread", len(lines))
	case len(lines) > 1:
		subject = fmt.Sprintf("%d issues have news in Horizon", len(lines))
	}
	return platformmail.Message{To: &mail.Address{Name: u.Name, Address: u.Email}, Subject: subject, Body: b.String()}
}

func (s *Service) link(path string) string {
	if s.o.BaseURL == nil {
		return path
	}
	return strings.TrimRight(s.o.BaseURL.String(), "/") + path
}

// Summary tells an event in a line, as "Ada moved it from Todo to Done",
// to the user me.
func Summary(e issue.PublicEvent, me uuid.UUID, reason string) string {
	who := "Horizon" // the system: an automation
	if e.Actor != nil {
		who = e.Actor.Name
		if who == "" {
			who = "Someone"
		}
	}
	p := e.Payload
	var what string
	switch e.Type {
	case "issue.created":
		what = "created the issue"
	case "issue.commented":
		what = "commented"
		if reason == Mentioned {
			what = "mentioned you in a comment"
		}
	case "issue.described":
		what = "edited the description"
		if reason == Mentioned {
			what = "mentioned you in the description"
		}
	case "issue.status_updated":
		what = fmt.Sprintf("moved it from %s to %s", nameOf(p["from"]), nameOf(p["to"]))
	case "issue.assigned":
		switch to := p["to"]; {
		case to == nil:
			what = "unassigned it"
		case refID(to) == me:
			what = "assigned it to you"
		default:
			what = "assigned it to " + nameOf(to)
		}
	case "issue.prioritized":
		what = "changed the priority"
	case "issue.estimated":
		what = "estimated it"
	case "issue.updated":
		what = "edited it"
	case "issue.moved":
		what = fmt.Sprintf("moved it from %v to %v", p["from_key"], p["to_key"])
	case "issue.archived":
		what = "archived it"
	case "issue.unarchived":
		what = "unarchived it"
	case "issue.linked":
		what = "linked it to " + nameOf(p["issue_id"])
	case "issue.attachment_added":
		what = "attached a file"
	case "issue.git_linked":
		what = fmt.Sprintf("linked pull request %v#%v %v", p["repository"], p["ref"], p["title"])
	case "issue.git_updated":
		what = fmt.Sprintf("saw pull request %v#%v %v", p["repository"], p["ref"], p["state"])
	default:
		what = strings.ReplaceAll(strings.TrimPrefix(e.Type, "issue."), "_", " ")
	}
	return who + " " + what
}

func nameOf(v any) string {
	switch x := v.(type) {
	case *issue.Ref:
		if x.Key != "" {
			return strings.TrimSpace(x.Key + " " + x.Name)
		}
		if x.Name != "" {
			return x.Name
		}
		return x.ID.String()
	case string:
		return x
	case nil:
		return "none"
	}
	return fmt.Sprint(v)
}

func refID(v any) uuid.UUID {
	switch x := v.(type) {
	case *issue.Ref:
		return x.ID
	case string:
		id, _ := uuid.Parse(x)
		return id
	}
	return uuid.Nil
}
