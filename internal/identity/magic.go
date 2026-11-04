package identity

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity/db"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Magic links (docs/security.md#authentication-methods): a single-use link
// emailed on request, valid MagicLinkLifetime, that only works in the
// browser that asked for it (the nonce in its cookie).

// Requests per window, so nobody floods an inbox.
const (
	maxMagicPerEmail = 5
	maxMagicPerIP    = 50
)

// NewNonce returns a nonce for the requesting browser's cookie.
func NewNonce() string {
	n, _ := newToken()
	return n
}

// RequestMagicLink mails a sign-in link to an active account the sign-in
// policy lets use one, bound to nonce. It answers the same whether or not
// the email belongs to an account; requests are throttled per email and IP.
func (s *Service) RequestMagicLink(ctx context.Context, email, nonce string, client Client) error {
	email = strings.TrimSpace(email)
	if _, ok := normalizeEmail(email); !ok {
		return apperr.Invalid(apperr.Field{Field: "email", Code: "invalid"})
	}
	if len(nonce) < 32 {
		return apperr.BadRequest("missing browser nonce")
	}
	keys := []throttleKey{{"magic:" + strings.ToLower(email), maxMagicPerEmail}}
	if client.IP.IsValid() {
		keys = append(keys, throttleKey{"magic-ip:" + client.IP.String(), maxMagicPerIP})
	}
	q := db.New(s.pool)
	if err := s.checkThrottle(ctx, q, keys); err != nil {
		return err
	}
	if err := s.recordFailure(ctx, q, keys); err != nil { // every request counts
		return err
	}
	u, err := q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.DeactivatedAt != nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.allowed(ctx, u, MethodMagicLink) != nil {
		return nil // SSO only: no link, and nothing said
	}
	if s.mail == nil || s.baseURL == nil {
		return errors.New("identity: magic links need SMTP and BASE_URL")
	}
	token, hash := newToken()
	now := s.now()
	if err := q.CreateMagicLink(ctx, db.CreateMagicLinkParams{ID: uuid.Must(uuid.NewV7()), UserID: u.ID, TokenHash: hash,
		NonceHash: hashToken(nonce), CreatedAt: now, ExpiresAt: now.Add(MagicLinkLifetime)}); err != nil {
		return err
	}
	link := s.baseURL.JoinPath("magic-link")
	link.Fragment = "token=" + token // the fragment never reaches server logs
	msg := magicMessage(u, link.String())
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := s.mail.Send(ctx, msg); err != nil {
			s.log.Error().Err(err).Str("user_id", u.ID.String()).Msg("sending a magic link")
		}
	}()
	return nil
}

// SignInWithMagicLink opens a session with a link's token, in the browser
// holding its nonce.
func (s *Service) SignInWithMagicLink(ctx context.Context, token, nonce string, client Client) (SignedIn, error) {
	var out SignedIn
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		userID, err := q.ClaimMagicLink(ctx, db.ClaimMagicLinkParams{TokenHash: hashToken(token), NonceHash: hashToken(nonce), Now: s.now()})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Invalid(apperr.Field{Field: "token", Code: "invalid_or_expired"})
		}
		if err != nil {
			return err
		}
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		if u.DeactivatedAt != nil {
			return apperr.Invalid(apperr.Field{Field: "token", Code: "invalid_or_expired"})
		}
		if err := s.allowed(ctx, u, MethodMagicLink); err != nil {
			return err
		}
		out, err = s.openSession(ctx, q, u, client, MethodMagicLink)
		return err
	})
	return out, err
}

func magicMessage(u db.User, link string) platformmail.Message {
	return platformmail.Message{
		To:      &mail.Address{Name: u.Name, Address: u.Email},
		Subject: "Your Horizon sign-in link",
		Body: fmt.Sprintf(`Hello %s,

Open this link to sign in to Horizon. It works once, within 15 minutes, in
the browser where you asked for it:

%s

If it wasn't you, ignore this email: nobody can use the link from another
browser.
`, u.Name, link),
	}
}
