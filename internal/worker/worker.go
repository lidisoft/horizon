// Package worker runs the JetStream consumers (docs/architecture.md#workers).
//
// Phase 1 starts with no consumers; the outbox relay is the first to land.
package worker

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// Deps groups what the consumers need.
type Deps struct {
	Log       zerolog.Logger
	JetStream jetstream.JetStream
}

// Run starts every consumer and blocks until ctx is cancelled.
func Run(ctx context.Context, deps Deps) error {
	deps.Log.Info().Msg("worker started")
	<-ctx.Done()
	deps.Log.Info().Msg("worker stopped")
	return nil
}
