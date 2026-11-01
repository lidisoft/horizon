// Package worker runs the JetStream consumers (docs/architecture.md#workers).
//
// The outbox relay comes first: it feeds every other consumer.
package worker

import (
	"context"
	"time"

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
	// Jobs run periodically next to the consumers.
	Jobs []Job
}

// Job is periodic housekeeping, such as deleting expired sessions. A failed
// run is logged and retried at the next tick; it never stops the worker.
type Job struct {
	Name  string
	Every time.Duration
	Run   func(ctx context.Context) error
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

	purger := &eventlog.Purger{
		Pool:      deps.Pool,
		JetStream: deps.JetStream,
		Stream:    deps.Stream,
		Log:       logging.WithSystem(deps.Log, "purge"),
	}
	jobs := append([]Job{{Name: "purge", Every: 15 * time.Second, Run: purger.Run}}, deps.Jobs...)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return relay.Run(ctx) })
	for _, job := range jobs {
		g.Go(func() error {
			runJob(ctx, job, logging.WithSystem(deps.Log, "job").With().Str("job", job.Name).Logger())
			return nil
		})
	}
	return g.Wait()
}

func runJob(ctx context.Context, job Job, log zerolog.Logger) {
	tick := time.NewTicker(job.Every)
	defer tick.Stop()
	for {
		if err := job.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error().Err(err).Msg("job failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
