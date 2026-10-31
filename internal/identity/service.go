// Package identity owns users, password sign-in, sessions and password
// resets (docs/security.md). Users are instance-wide; workspace membership
// lives in internal/workspace.
//
// Accounts are created by an instance admin or by accepting an invitation;
// there is no public sign-up.
package identity

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"net/netip"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata" // validate IANA zones on any base image

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity/db"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Lifetimes (docs/security.md).
const (
	SessionIdleTimeout = 30 * 24 * time.Hour
	// sessionTouchEvery limits how often a busy session's expiry is written.
	sessionTouchEvery  = 5 * time.Minute
	ResetTokenLifetime = time.Hour
)

// Sign-in throttling: failures allowed per key within ThrottleWindow.
const (
	ThrottleWindow      = 15 * time.Minute
	maxFailuresPerEmail = 10
	maxFailuresPerIP    = 100
)

// Locales the UI ships (ADR-0017).
var Locales = []string{"en"}

// aggregateType of user events; users are instance-level, so their events
// carry no workspace.
const aggregateType = "user"

// User is the public view of an account.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	Name            string     `json:"name"`
	Locale          string     `json:"locale"`
	Timezone        *string    `json:"timezone"`
	IsInstanceAdmin bool       `json:"is_instance_admin"`
	HasPassword     bool       `json:"has_password"`
	DeactivatedAt   *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	Version         int32      `json:"version"`
}

func userFrom(u db.User) User {
	return User{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		Locale:          u.Locale,
		Timezone:        u.Timezone,
		IsInstanceAdmin: u.IsInstanceAdmin,
		HasPassword:     u.PasswordHash != nil,
		DeactivatedAt:   u.DeactivatedAt,
		CreatedAt:       u.CreatedAt,
		Version:         u.Version,
	}
}

// Session is the public view of a sign-in session.
type Session struct {
	ID         uuid.UUID   `json:"id"`
	CreatedAt  time.Time   `json:"created_at"`
	LastSeenAt time.Time   `json:"last_seen_at"`
	ExpiresAt  time.Time   `json:"expires_at"`
	UserAgent  string      `json:"user_agent"`
	IP         *netip.Addr `json:"ip" swaggertype:"string"`
	Current    bool        `json:"current"`
}

// Options configure a Service.
type Options struct {
	Pool *pgxpool.Pool
	Log  zerolog.Logger
	// Mail sends password reset links; nil disables them (admin CLI).
	Mail platformmail.Sender
	// BaseURL builds links in emails.
	BaseURL *url.URL
	// SessionSecret signs CSRF tokens.
	SessionSecret string
	// Now is the clock; time.Now when nil.
	Now func() time.Time
}

// Service implements the identity commands and queries.
type Service struct {
	pool          *pgxpool.Pool
	log           zerolog.Logger
	mail          platformmail.Sender
	baseURL       *url.URL
	sessionSecret string
	now           func() time.Time
}

// New returns a Service.
func New(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		pool:          o.Pool,
		log:           o.Log,
		mail:          o.Mail,
		baseURL:       o.BaseURL,
		sessionSecret: o.SessionSecret,
		now:           func() time.Time { return now().UTC().Truncate(time.Microsecond) },
	}
}

// BaseURL is the instance's public URL, nil when not configured.
func (s *Service) BaseURL() *url.URL { return s.baseURL }

// --- Users ------------------------------------------------------------------

// NewUser is the input of CreateUser.
type NewUser struct {
	Email         string
	Name          string
	Password      string // empty: no password yet (set through a reset link)
	InstanceAdmin bool
}

// normalizeEmail trims email and checks it is a bare address.
func normalizeEmail(email string) (string, bool) {
	email = strings.TrimSpace(email)
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 320 {
		return email, false
	}
	return email, true
}

// CreateUser creates an account. actor is the admin or inviter, if any.
func (s *Service) CreateUser(ctx context.Context, actor uuid.NullUUID, in NewUser) (User, error) {
	var u User
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		u, err = s.CreateUserInTx(ctx, tx, actor, in)
		return err
	})
	return u, err
}

// CreateUserInTx creates an account within the caller's transaction, for
// commands that create one along with something else (accepting an
// invitation).
func (s *Service) CreateUserInTx(ctx context.Context, tx pgx.Tx, actor uuid.NullUUID, in NewUser) (User, error) {
	var fields apperr.Fields
	email, ok := normalizeEmail(in.Email)
	if !ok {
		fields.Add("email", "invalid")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		fields.Add("name", "required")
	} else if len([]rune(name)) > 200 {
		fields.Add("name", "too_long")
	}
	var hash *string
	if in.Password != "" {
		if code := passwordProblem(in.Password); code != "" {
			fields.Add("password", code)
		}
	}
	if err := fields.Err(); err != nil {
		return User{}, err
	}
	if in.Password != "" {
		h, err := hashPassword(in.Password)
		if err != nil {
			return User{}, err
		}
		hash = &h
	}

	created, err := db.New(tx).CreateUser(ctx, db.CreateUserParams{
		ID:              uuid.Must(uuid.NewV7()),
		Email:           email,
		Name:            name,
		PasswordHash:    hash,
		IsInstanceAdmin: in.InstanceAdmin,
		CreatedAt:       s.now(),
	})
	if isUniqueViolation(err) {
		return User{}, apperr.Invalid(apperr.Field{Field: "email", Code: "taken"})
	}
	if err != nil {
		return User{}, err
	}
	err = eventlog.Append(ctx, tx, s.event(actor, created, "UserCreated",
		map[string]any{"instance_admin": created.IsInstanceAdmin}))
	if err != nil {
		return User{}, err
	}
	return userFrom(created), nil
}

// OpenSessionInTx signs a user in within the caller's transaction, after
// they proved who they are some other way (accepting an invitation).
func (s *Service) OpenSessionInTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, client Client) (SignedIn, error) {
	q := db.New(tx)
	u, err := q.GetUser(ctx, userID)
	if err != nil {
		return SignedIn{}, err
	}
	if u.DeactivatedAt != nil {
		return SignedIn{}, apperr.Unauthorized("invalid_credentials")
	}
	return s.openSession(ctx, q, u, client)
}

// User returns an account by id.
func (s *Service) User(ctx context.Context, id uuid.UUID) (User, error) {
	u, err := db.New(s.pool).GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.NotFound("user_not_found")
	}
	if err != nil {
		return User{}, err
	}
	return userFrom(u), nil
}

// UserByEmail returns an account by email (case-insensitive).
func (s *Service) UserByEmail(ctx context.Context, email string) (User, error) {
	u, err := db.New(s.pool).GetUserByEmail(ctx, strings.TrimSpace(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.NotFound("user_not_found")
	}
	if err != nil {
		return User{}, err
	}
	return userFrom(u), nil
}

// ProfileUpdate changes the fields that are set.
type ProfileUpdate struct {
	Name     *string
	Locale   *string
	Timezone *string // "" clears it: follow the browser
}

// UpdateProfile edits the user's own profile. version is the If-Match value
// (0 skips the check).
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, version int32, in ProfileUpdate) (User, error) {
	var updated db.User
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		u, err := q.LockUser(ctx, userID)
		if err != nil {
			return err
		}
		if version != 0 && u.Version != version {
			return apperr.Conflict("version_conflict")
		}
		p := db.UpdateUserProfileParams{ID: u.ID, Name: u.Name, Locale: u.Locale, Timezone: u.Timezone, UpdatedAt: s.now()}
		var fields apperr.Fields
		var changed []string
		if in.Name != nil {
			name := strings.TrimSpace(*in.Name)
			switch {
			case name == "":
				fields.Add("name", "required")
			case len([]rune(name)) > 200:
				fields.Add("name", "too_long")
			case name != u.Name:
				p.Name = name
				changed = append(changed, "name")
			}
		}
		if in.Locale != nil && *in.Locale != u.Locale {
			if !contains(Locales, *in.Locale) {
				fields.Add("locale", "unsupported")
			}
			p.Locale = *in.Locale
			changed = append(changed, "locale")
		}
		if in.Timezone != nil {
			tz := strings.TrimSpace(*in.Timezone)
			switch {
			case tz == "" && u.Timezone != nil:
				p.Timezone = nil
				changed = append(changed, "timezone")
			case tz != "" && (u.Timezone == nil || *u.Timezone != tz):
				if _, err := time.LoadLocation(tz); err != nil || tz == "Local" {
					fields.Add("timezone", "unknown")
				}
				p.Timezone = &tz
				changed = append(changed, "timezone")
			}
		}
		if err := fields.Err(); err != nil {
			return err
		}
		if len(changed) == 0 {
			updated = u
			return nil
		}
		if updated, err = q.UpdateUserProfile(ctx, p); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(uuid.NullUUID{UUID: userID, Valid: true}, updated,
			"UserProfileUpdated", map[string]any{"fields": changed}))
	})
	if err != nil {
		return User{}, err
	}
	return userFrom(updated), nil
}

// SetDeactivated deactivates (true) or reactivates an account instance-wide.
// Deactivation revokes every session and API token.
func (s *Service) SetDeactivated(ctx context.Context, actor uuid.NullUUID, userID uuid.UUID, deactivated bool) (User, error) {
	var updated db.User
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		u, err := q.LockUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("user_not_found")
		}
		if err != nil {
			return err
		}
		if (u.DeactivatedAt != nil) == deactivated {
			updated = u
			return nil
		}
		var at *time.Time
		typ := "UserReactivated"
		if deactivated {
			now := s.now()
			at = &now
			typ = "UserDeactivated"
			if _, err := q.DeleteUserSessions(ctx, db.DeleteUserSessionsParams{UserID: u.ID, Keep: uuid.Nil}); err != nil {
				return err
			}
			if _, err := q.DeleteUserAPITokens(ctx, u.ID); err != nil {
				return err
			}
		}
		if updated, err = q.SetUserDeactivated(ctx, db.SetUserDeactivatedParams{ID: u.ID, DeactivatedAt: at, UpdatedAt: s.now()}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(actor, updated, typ, nil))
	})
	if err != nil {
		return User{}, err
	}
	return userFrom(updated), nil
}

// SetInstanceAdmin grants or removes instance administration.
func (s *Service) SetInstanceAdmin(ctx context.Context, actor uuid.NullUUID, userID uuid.UUID, admin bool) (User, error) {
	var updated db.User
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		u, err := q.LockUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("user_not_found")
		}
		if err != nil {
			return err
		}
		if u.IsInstanceAdmin == admin {
			updated = u
			return nil
		}
		if updated, err = q.SetUserInstanceAdmin(ctx, db.SetUserInstanceAdminParams{ID: u.ID, IsInstanceAdmin: admin, UpdatedAt: s.now()}); err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.event(actor, updated, "UserInstanceAdminChanged",
			map[string]any{"instance_admin": admin}))
	})
	if err != nil {
		return User{}, err
	}
	return userFrom(updated), nil
}

// --- Sign-in and sessions ---------------------------------------------------

// Client describes where a request comes from, recorded on sessions.
type Client struct {
	UserAgent string
	IP        netip.Addr
}

// SignedIn is the result of a successful sign-in. Token goes in the session
// cookie and CSRF in the CSRF cookie; neither is stored.
type SignedIn struct {
	User    User
	Session Session
	Token   string
	CSRF    string
}

// Login checks email and password, applying the sign-in throttle, and opens
// a session. Failures are indistinguishable: unknown email, wrong password,
// no password and deactivated account all return invalid_credentials.
func (s *Service) Login(ctx context.Context, email, password string, client Client) (SignedIn, error) {
	email = strings.TrimSpace(email)
	keys := []throttleKey{{"email:" + strings.ToLower(email), maxFailuresPerEmail}}
	if client.IP.IsValid() {
		keys = append(keys, throttleKey{"ip:" + client.IP.String(), maxFailuresPerIP})
	}
	q := db.New(s.pool)
	if err := s.checkThrottle(ctx, q, keys); err != nil {
		return SignedIn{}, err
	}

	u, err := q.GetUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return SignedIn{}, err
	}
	hash := dummyHash
	if err == nil && u.PasswordHash != nil {
		hash = *u.PasswordHash
	}
	match, verr := verifyPassword(hash, password)
	if verr != nil {
		return SignedIn{}, fmt.Errorf("user %s: %w", u.ID, verr)
	}
	if err != nil || u.PasswordHash == nil || !match || u.DeactivatedAt != nil {
		if terr := s.recordFailure(ctx, q, keys); terr != nil {
			return SignedIn{}, terr
		}
		return SignedIn{}, apperr.Unauthorized("invalid_credentials")
	}
	if err := q.ClearThrottle(ctx, keys[0].key); err != nil {
		return SignedIn{}, err
	}
	return s.openSession(ctx, q, u, client)
}

func (s *Service) openSession(ctx context.Context, q *db.Queries, u db.User, client Client) (SignedIn, error) {
	token, hash := newToken()
	now := s.now()
	var ip *netip.Addr
	if client.IP.IsValid() {
		addr := client.IP.Unmap()
		ip = &addr
	}
	ua := client.UserAgent
	if len(ua) > 512 {
		ua = ua[:512]
	}
	sess := db.CreateSessionParams{
		ID:        uuid.Must(uuid.NewV7()),
		UserID:    u.ID,
		TokenHash: hash,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionIdleTimeout),
		UserAgent: ua,
		Ip:        ip,
	}
	if err := q.CreateSession(ctx, sess); err != nil {
		return SignedIn{}, err
	}
	return SignedIn{
		User: userFrom(u),
		Session: Session{
			ID: sess.ID, CreatedAt: now, LastSeenAt: now, ExpiresAt: sess.ExpiresAt,
			UserAgent: ua, IP: ip, Current: true,
		},
		Token: token,
		CSRF:  csrfToken(s.sessionSecret, hash),
	}, nil
}

// Principal is the authenticated caller of a request: a session or an API
// token.
type Principal struct {
	User      User
	SessionID uuid.UUID // uuid.Nil for API tokens
	TokenID   uuid.UUID // uuid.Nil for sessions
	// Scope bounds what the caller may do; sessions have ScopeAdmin.
	Scope Scope
	// tokenHash of the session, for the CSRF check.
	tokenHash []byte
	// renewed is set when the session's expiry slid, so the cookie's should.
	renewed bool
}

// Authenticate resolves a session token, sliding its expiry. It returns
// unauthenticated for unknown, expired or revoked sessions and deactivated
// users.
func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, apperr.Unauthorized("unauthenticated")
	}
	hash := hashToken(token)
	now := s.now()
	q := db.New(s.pool)
	row, err := q.SessionByTokenHash(ctx, db.SessionByTokenHashParams{TokenHash: hash, Now: now})
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, apperr.Unauthorized("unauthenticated")
	}
	if err != nil {
		return Principal{}, err
	}
	renewed := now.Sub(row.Session.LastSeenAt) >= sessionTouchEvery
	if renewed {
		err := q.TouchSession(ctx, db.TouchSessionParams{ID: row.Session.ID, LastSeenAt: now, ExpiresAt: now.Add(SessionIdleTimeout)})
		if err != nil {
			return Principal{}, err
		}
	}
	return Principal{User: userFrom(row.User), SessionID: row.Session.ID, Scope: ScopeAdmin, tokenHash: hash, renewed: renewed}, nil
}

// CSRFToken is the token the SPA sends back in X-CSRF-Token.
func (s *Service) CSRFToken(p Principal) string {
	if p.tokenHash == nil {
		return ""
	}
	return csrfToken(s.sessionSecret, p.tokenHash)
}

// ValidCSRF checks the double-submit token sent with a state-changing
// request made with the session cookie.
func (s *Service) ValidCSRF(p Principal, header string) bool {
	return p.tokenHash != nil && validCSRF(s.sessionSecret, p.tokenHash, header)
}

// Logout revokes the caller's session.
func (s *Service) Logout(ctx context.Context, p Principal) error {
	if p.SessionID == uuid.Nil {
		return nil
	}
	_, err := db.New(s.pool).DeleteSession(ctx, db.DeleteSessionParams{ID: p.SessionID, UserID: p.User.ID})
	return err
}

// Sessions lists the user's live sessions, most recently used first.
func (s *Service) Sessions(ctx context.Context, p Principal) ([]Session, error) {
	rows, err := db.New(s.pool).ListUserSessions(ctx, db.ListUserSessionsParams{UserID: p.User.ID, Now: s.now()})
	if err != nil {
		return nil, err
	}
	out := make([]Session, len(rows))
	for i, r := range rows {
		out[i] = Session{
			ID: r.ID, CreatedAt: r.CreatedAt, LastSeenAt: r.LastSeenAt, ExpiresAt: r.ExpiresAt,
			UserAgent: r.UserAgent, IP: r.Ip, Current: r.ID == p.SessionID,
		}
	}
	return out, nil
}

// RevokeSession signs one of the user's sessions out.
func (s *Service) RevokeSession(ctx context.Context, p Principal, id uuid.UUID) error {
	n, err := db.New(s.pool).DeleteSession(ctx, db.DeleteSessionParams{ID: id, UserID: p.User.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return apperr.NotFound("session_not_found")
	}
	return nil
}

// RevokeOtherSessions signs out everywhere but the current session.
func (s *Service) RevokeOtherSessions(ctx context.Context, p Principal) (int64, error) {
	return db.New(s.pool).DeleteUserSessions(ctx, db.DeleteUserSessionsParams{UserID: p.User.ID, Keep: p.SessionID})
}

// --- Passwords --------------------------------------------------------------

// ChangePassword replaces the caller's password after checking the current
// one, and signs out their other sessions.
func (s *Service) ChangePassword(ctx context.Context, p Principal, current, next string) error {
	if code := passwordProblem(next); code != "" {
		return apperr.Invalid(apperr.Field{Field: "new_password", Code: code})
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		u, err := q.LockUser(ctx, p.User.ID)
		if err != nil {
			return err
		}
		if u.PasswordHash == nil {
			return apperr.Invalid(apperr.Field{Field: "current_password", Code: "incorrect"})
		}
		ok, err := verifyPassword(*u.PasswordHash, current)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.Invalid(apperr.Field{Field: "current_password", Code: "incorrect"})
		}
		return s.setPassword(ctx, tx, u, next, p.SessionID, false)
	})
}

// setPassword stores a new hash, revokes sessions except keep and pending
// resets, and records the change.
func (s *Service) setPassword(ctx context.Context, tx pgx.Tx, u db.User, password string, keep uuid.UUID, reset bool) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	q := db.New(tx)
	updated, err := q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: u.ID, PasswordHash: &hash, UpdatedAt: s.now()})
	if err != nil {
		return err
	}
	if _, err := q.DeleteUserSessions(ctx, db.DeleteUserSessionsParams{UserID: u.ID, Keep: keep}); err != nil {
		return err
	}
	if err := q.DeleteUserPasswordResets(ctx, u.ID); err != nil {
		return err
	}
	return eventlog.Append(ctx, tx, s.event(uuid.NullUUID{UUID: u.ID, Valid: true}, updated,
		"UserPasswordChanged", map[string]any{"reset": reset}))
}

// IssuePasswordReset creates a single-use reset link for a user, valid for
// ResetTokenLifetime. The admin CLI prints it; RequestPasswordReset mails it.
func (s *Service) IssuePasswordReset(ctx context.Context, userID uuid.UUID) (string, error) {
	if s.baseURL == nil {
		return "", errors.New("identity: HORIZON_BASE_URL is needed to build reset links")
	}
	token, hash := newToken()
	now := s.now()
	err := db.New(s.pool).CreatePasswordReset(ctx, db.CreatePasswordResetParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, TokenHash: hash, CreatedAt: now, ExpiresAt: now.Add(ResetTokenLifetime),
	})
	if err != nil {
		return "", err
	}
	link := s.baseURL.JoinPath("reset-password")
	link.Fragment = "token=" + token // the fragment never reaches server logs
	return link.String(), nil
}

// RequestPasswordReset mails a reset link when email belongs to an active
// account. It answers the same either way, so it can't probe for accounts;
// delivery happens in the background.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if _, ok := normalizeEmail(email); !ok {
		return apperr.Invalid(apperr.Field{Field: "email", Code: "invalid"})
	}
	u, err := db.New(s.pool).GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && u.DeactivatedAt != nil) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.mail == nil {
		return errors.New("identity: no mail sender configured")
	}
	link, err := s.IssuePasswordReset(ctx, u.ID)
	if err != nil {
		return err
	}
	msg := resetMessage(u, link)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		if err := s.mail.Send(ctx, msg); err != nil {
			s.log.Error().Err(err).Str("user_id", u.ID.String()).Msg("sending password reset")
		}
	}()
	return nil
}

// ResetPassword sets a new password with a reset token and signs out every
// session.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if code := passwordProblem(password); code != "" {
		return apperr.Invalid(apperr.Field{Field: "password", Code: code})
	}
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		userID, err := q.ClaimPasswordReset(ctx, db.ClaimPasswordResetParams{TokenHash: hashToken(token), Now: s.now()})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Invalid(apperr.Field{Field: "token", Code: "invalid_or_expired"})
		}
		if err != nil {
			return err
		}
		u, err := q.LockUser(ctx, userID)
		if err != nil {
			return err
		}
		if u.DeactivatedAt != nil {
			return apperr.Invalid(apperr.Field{Field: "token", Code: "invalid_or_expired"})
		}
		return s.setPassword(ctx, tx, u, password, uuid.Nil, true)
	})
}

// --- Housekeeping -----------------------------------------------------------

// Cleanup deletes expired sessions, reset tokens and throttle windows. The
// worker runs it periodically.
func (s *Service) Cleanup(ctx context.Context) error {
	q := db.New(s.pool)
	now := s.now()
	sessions, err := q.DeleteExpiredSessions(ctx, now)
	if err != nil {
		return err
	}
	resets, err := q.DeleteStalePasswordResets(ctx, now)
	if err != nil {
		return err
	}
	throttles, err := q.DeleteStaleThrottles(ctx, now.Add(-ThrottleWindow))
	if err != nil {
		return err
	}
	s.log.Debug().Int64("sessions", sessions).Int64("password_resets", resets).Int64("throttles", throttles).Msg("identity cleanup")
	return nil
}

// --- Helpers ----------------------------------------------------------------

func (s *Service) event(actor uuid.NullUUID, u db.User, typ string, payload map[string]any) eventlog.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return eventlog.Event{
		AggregateType:    aggregateType,
		AggregateID:      u.ID,
		AggregateVersion: int64(u.Version),
		Type:             typ,
		ActorID:          actor,
		OccurredAt:       s.now(),
		Payload:          eventlog.Payload(payload),
	}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
