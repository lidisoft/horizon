// Package notify tells people what happens to the issues they follow
// (docs/domain.md#notifications): subscriptions, the inbox, and email,
// sent for what stays unread and as a daily digest.
//
// The notify consumer reads issue events: it subscribes the people an
// event concerns (the creator, the assignee, those mentioned, commenters)
// and bumps one inbox row per follower and issue. Two jobs send email.
package notify

import (
	"context"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/workspace"
)

// Email timing.
const (
	// EmailDelay is how long a row stays unread before it is emailed: what
	// is read in the app meanwhile needs no email.
	EmailDelay = 10 * time.Minute
	// DigestHour is the local hour from which the daily digest goes out.
	DigestHour = 8
)

// Subscription reasons.
const (
	ReasonCreator   = "creator"
	ReasonAssignee  = "assignee"
	ReasonMention   = "mention"
	ReasonCommenter = "commenter"
	ReasonManual    = "manual"
)

// Inbox reasons, weakest first.
const (
	Subscribed = "subscribed"
	Mentioned  = "mentioned"
	Assigned   = "assigned"
)

// Options are the service's dependencies.
type Options struct {
	Pool       *pgxpool.Pool
	Log        zerolog.Logger
	Identity   *identity.Service
	Access     *access.Service
	Workspaces *workspace.Service
	Issues     *issue.Service
	// Mail sends email; nil turns email off (no SMTP configured).
	Mail    platformmail.Sender
	BaseURL *url.URL
	// Publish tells the user's open pages of a change to their inbox
	// (realtime, best effort); nil does nothing.
	Publish func(userID uuid.UUID, data []byte)
	Now     func() time.Time
}

// Service manages subscriptions and inboxes.
type Service struct {
	o Options
}

// New returns a Service.
func New(o Options) *Service {
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Service{o: o}
}

func (s *Service) now() time.Time { return s.o.Now().UTC() }

// principal acts as the user, for permission checks; ok is false for an
// unknown or deactivated account.
func (s *Service) principal(ctx context.Context, userID uuid.UUID) (identity.Principal, bool, error) {
	u, err := s.o.Identity.User(ctx, userID)
	if err != nil {
		if isNotFound(err) {
			return identity.Principal{}, false, nil
		}
		return identity.Principal{}, false, err
	}
	if u.DeactivatedAt != nil {
		return identity.Principal{}, false, nil
	}
	return identity.Principal{User: u, Scope: identity.ScopeAdmin}, true, nil
}

// sees reports whether the user may see the issue (workspace slug, issue
// key) now: an active member with issue.view on its project.
func (s *Service) sees(ctx context.Context, userID uuid.UUID, slug, key string) (bool, error) {
	p, ok, err := s.principal(ctx, userID)
	if err != nil || !ok {
		return false, err
	}
	wc, err := s.o.Workspaces.Enter(ctx, p, slug)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if _, err := s.o.Issues.Enter(ctx, wc, key); err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
