package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity/db"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// Personal access tokens (docs/security.md#api-tokens): "hzn_" and 43
// random URL-safe characters, sent as Authorization: Bearer. The prefix lets
// secret scanners recognise leaked tokens.
const (
	TokenPrefix = "hzn_"
	// tokenHintLen is how much of the token is kept to tell tokens apart.
	tokenHintLen = len(TokenPrefix) + 4
	// MaxAPITokens bounds the tokens of one user.
	MaxAPITokens = 50
	// tokenTouchEvery limits how often last_used_at is written.
	tokenTouchEvery = 5 * time.Minute
)

// Scope limits what a token may do. Each scope includes the ones before it.
type Scope string

// Scopes.
const (
	// ScopeRead allows safe requests (GET, HEAD) only.
	ScopeRead Scope = "read"
	// ScopeWrite also allows changes, within the user's permissions.
	ScopeWrite Scope = "write"
	// ScopeAdmin also allows administration: the workspace.* permissions and
	// instance administration. Sessions have it.
	ScopeAdmin Scope = "admin"
)

var scopeRank = map[Scope]int{ScopeRead: 1, ScopeWrite: 2, ScopeAdmin: 3}

// Includes reports whether s grants need.
func (s Scope) Includes(need Scope) bool {
	return scopeRank[s] >= scopeRank[need] && scopeRank[need] > 0
}

// APIToken is the public view of a token; the secret is never stored.
type APIToken struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	Hint       string     `json:"hint" example:"hzn_k3Xa"`
	Scope      Scope      `json:"scope" enums:"read,write,admin"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
}

func tokenFrom(t db.ApiToken) APIToken {
	return APIToken{
		ID: t.ID, Name: t.Name, Hint: t.Hint, Scope: Scope(t.Scope),
		CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt, ExpiresAt: t.ExpiresAt,
	}
}

// NewAPIToken is the input of CreateAPIToken.
type NewAPIToken struct {
	Name      string
	Scope     Scope
	ExpiresAt *time.Time // nil: never
}

// CreatedAPIToken carries the secret, shown to the user once.
type CreatedAPIToken struct {
	APIToken
	Token string `json:"token" example:"hzn_k3Xa…"`
}

// CreateAPIToken issues a token for the caller.
func (s *Service) CreateAPIToken(ctx context.Context, p Principal, in NewAPIToken) (CreatedAPIToken, error) {
	now := s.now()
	var fields apperr.Fields
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		fields.Add("name", "required")
	case len([]rune(name)) > 100:
		fields.Add("name", "too_long")
	}
	if _, ok := scopeRank[in.Scope]; !ok {
		fields.Add("scope", "invalid")
	}
	if in.ExpiresAt != nil && !in.ExpiresAt.After(now) {
		fields.Add("expires_at", "past")
	}
	if err := fields.Err(); err != nil {
		return CreatedAPIToken{}, err
	}

	random, _ := newToken()
	secret := TokenPrefix + random
	hash := hashToken(secret)
	var created db.ApiToken
	err := postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		// Serialises token creation per user, so the limit holds.
		if _, err := q.LockUser(ctx, p.User.ID); err != nil {
			return err
		}
		n, err := q.CountUserAPITokens(ctx, p.User.ID)
		if err != nil {
			return err
		}
		if n >= MaxAPITokens {
			return apperr.Conflict("too_many_tokens")
		}
		var expires *time.Time
		if in.ExpiresAt != nil {
			e := in.ExpiresAt.UTC().Truncate(time.Microsecond)
			expires = &e
		}
		created, err = q.CreateAPIToken(ctx, db.CreateAPITokenParams{
			ID: uuid.Must(uuid.NewV7()), UserID: p.User.ID, Name: name, TokenHash: hash,
			Hint: secret[:tokenHintLen], Scope: string(in.Scope), CreatedAt: now, ExpiresAt: expires,
		})
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.tokenEvent(p.User.ID, created.ID, 1, "ApiTokenCreated",
			map[string]any{"user_id": p.User.ID, "scope": created.Scope, "expires_at": created.ExpiresAt}))
	})
	if err != nil {
		return CreatedAPIToken{}, err
	}
	return CreatedAPIToken{APIToken: tokenFrom(created), Token: secret}, nil
}

// APITokens lists the caller's tokens, newest first, expired ones included.
func (s *Service) APITokens(ctx context.Context, p Principal) ([]APIToken, error) {
	rows, err := db.New(s.pool).ListUserAPITokens(ctx, p.User.ID)
	if err != nil {
		return nil, err
	}
	out := make([]APIToken, len(rows))
	for i, r := range rows {
		out[i] = tokenFrom(r)
	}
	return out, nil
}

// RevokeAPIToken deletes one of the caller's tokens.
func (s *Service) RevokeAPIToken(ctx context.Context, p Principal, id uuid.UUID) error {
	return postgres.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := db.New(tx).DeleteAPIToken(ctx, db.DeleteAPITokenParams{ID: id, UserID: p.User.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.NotFound("token_not_found")
		}
		if err != nil {
			return err
		}
		return eventlog.Append(ctx, tx, s.tokenEvent(p.User.ID, t.ID, 2, "ApiTokenRevoked",
			map[string]any{"user_id": p.User.ID}))
	})
}

// AuthenticateToken resolves a bearer token. Unknown, expired and revoked
// tokens, and tokens of deactivated users, are unauthenticated.
func (s *Service) AuthenticateToken(ctx context.Context, token string) (Principal, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return Principal{}, apperr.Unauthorized("unauthenticated")
	}
	now := s.now()
	q := db.New(s.pool)
	row, err := q.APITokenByHash(ctx, db.APITokenByHashParams{TokenHash: hashToken(token), Now: &now})
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, apperr.Unauthorized("unauthenticated")
	}
	if err != nil {
		return Principal{}, err
	}
	if last := row.ApiToken.LastUsedAt; last == nil || now.Sub(*last) >= tokenTouchEvery {
		if err := q.TouchAPIToken(ctx, db.TouchAPITokenParams{ID: row.ApiToken.ID, LastUsedAt: &now}); err != nil {
			return Principal{}, err
		}
	}
	return Principal{User: userFrom(row.User), TokenID: row.ApiToken.ID, Scope: Scope(row.ApiToken.Scope)}, nil
}

func (s *Service) tokenEvent(actor, id uuid.UUID, version int64, typ string, payload map[string]any) eventlog.Event {
	return eventlog.Event{
		AggregateType:    "api_token",
		AggregateID:      id,
		AggregateVersion: version,
		Type:             typ,
		ActorID:          uuid.NullUUID{UUID: actor, Valid: true},
		OccurredAt:       s.now(),
		Payload:          eventlog.Payload(payload),
	}
}
