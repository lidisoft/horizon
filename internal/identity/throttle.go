package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/identity/db"
)

// Sign-in throttling counts failures per key in fixed windows of
// ThrottleWindow: per email (protects one account) and per IP (slows
// spraying across accounts). The counters live in PostgreSQL, so every serve
// replica shares them.

type throttleKey struct {
	key   string
	limit int32
}

// checkThrottle refuses the attempt while any key is over its limit.
func (s *Service) checkThrottle(ctx context.Context, q *db.Queries, keys []throttleKey) error {
	now := s.now()
	for _, k := range keys {
		row, err := q.ThrottleFailures(ctx, db.ThrottleFailuresParams{Key: k.key, Since: now.Add(-ThrottleWindow)})
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if row.Failures >= k.limit {
			return apperr.TooManyRequests("too_many_attempts", row.WindowStart.Add(ThrottleWindow).Sub(now))
		}
	}
	return nil
}

func (s *Service) recordFailure(ctx context.Context, q *db.Queries, keys []throttleKey) error {
	now := s.now()
	for _, k := range keys {
		err := q.RecordThrottleFailure(ctx, db.RecordThrottleFailureParams{Key: k.key, Now: now, Since: now.Add(-ThrottleWindow)})
		if err != nil {
			return err
		}
	}
	return nil
}
