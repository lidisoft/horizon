// Package worker runs the JetStream consumers (docs/architecture.md#workers).
//
// The outbox relay comes first: it feeds every other consumer.
package worker

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/platform/metrics"
)

// Deps groups what the consumers need.
type Deps struct {
	Log       zerolog.Logger
	Pool      *pgxpool.Pool
	JetStream jetstream.JetStream
	Stream    eventlog.Stream
	Metrics   *metrics.Registry
}

// Run starts every consumer and blocks until ctx is cancelled or one fails.
func Run(ctx context.Context, deps Deps) error {
	relay := &eventlog.Relay{
		Pool:      deps.Pool,
		JetStream: deps.JetStream,
		Stream:    deps.Stream,
		Log:       logging.WithSystem(deps.Log, "outbox"),
		Metrics:   eventlog.NewRelayMetrics(deps.Metrics),
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return relay.Run(ctx) })
	return g.Wait()
}
