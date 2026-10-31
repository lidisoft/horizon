package eventlog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog/db"
)

// Relay defaults.
const (
	DefaultBatchSize    = 100
	DefaultPollInterval = time.Second
	maxBackoff          = 30 * time.Second
)

// notifyChannel is the channel the events trigger notifies on commit.
const notifyChannel = "events"

// Relay publishes unpublished events to JetStream, at least once, roughly in
// id order. Several relays may run at once: each claims its own batch with
// FOR UPDATE SKIP LOCKED.
type Relay struct {
	Pool         *pgxpool.Pool
	JetStream    jetstream.JetStream
	Stream       Stream
	Log          zerolog.Logger
	Metrics      *RelayMetrics // optional
	BatchSize    int
	PollInterval time.Duration
}

// RelayMetrics are the outbox lag metrics (docs/operations.md#health).
type RelayMetrics struct {
	published prometheus.Counter
	pending   prometheus.Gauge
	oldest    prometheus.Gauge
	failures  prometheus.Counter
}

// NewRelayMetrics registers the relay metrics on reg.
func NewRelayMetrics(reg prometheus.Registerer) *RelayMetrics {
	m := &RelayMetrics{
		published: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "horizon", Subsystem: "outbox", Name: "published_total",
			Help: "Events published to JetStream by this process.",
		}),
		pending: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "horizon", Subsystem: "outbox", Name: "pending_events",
			Help: "Events not yet published.",
		}),
		oldest: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "horizon", Subsystem: "outbox", Name: "lag_seconds",
			Help: "Age of the oldest unpublished event.",
		}),
		failures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "horizon", Subsystem: "outbox", Name: "failures_total",
			Help: "Relay batches that failed and will be retried.",
		}),
	}
	reg.MustRegister(m.published, m.pending, m.oldest, m.failures)
	return m
}

func (r *Relay) batchSize() int {
	if r.BatchSize > 0 {
		return r.BatchSize
	}
	return DefaultBatchSize
}

func (r *Relay) pollInterval() time.Duration {
	if r.PollInterval > 0 {
		return r.PollInterval
	}
	return DefaultPollInterval
}

// Run ensures the stream exists, then relays until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	if _, err := r.Stream.Ensure(ctx, r.JetStream); err != nil {
		return err
	}
	wake := make(chan struct{}, 1)
	go r.listen(ctx, wake)

	r.Log.Info().Str("stream", r.Stream.Name).Msg("outbox relay started")
	backoff := time.Duration(0)
	poll := time.NewTicker(r.pollInterval())
	defer poll.Stop()
	for {
		if _, err := r.Drain(ctx); err != nil {
			if ctx.Err() != nil {
				break
			}
			backoff = min(max(2*backoff, time.Second), maxBackoff)
			r.Log.Warn().Err(err).Dur("retry_in", backoff).Msg("outbox relay failed")
			if r.Metrics != nil {
				r.Metrics.failures.Inc()
			}
			select {
			case <-ctx.Done():
			case <-time.After(backoff):
			}
			continue
		}
		backoff = 0
		r.observe(ctx)

		select {
		case <-ctx.Done():
		case <-wake:
		case <-poll.C:
		}
		if ctx.Err() != nil {
			break
		}
	}
	r.Log.Info().Msg("outbox relay stopped")
	return nil
}

// Drain relays batches until none is left, returning how many were published.
func (r *Relay) Drain(ctx context.Context) (int, error) {
	total := 0
	for {
		n, err := r.relayBatch(ctx)
		total += n
		if err != nil || n < r.batchSize() {
			return total, err
		}
	}
}

func (r *Relay) relayBatch(ctx context.Context) (int, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	q := db.New(tx)
	rows, err := q.ClaimUnpublished(ctx, int32(r.batchSize()))
	if err != nil {
		return 0, fmt.Errorf("claim: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	futures := make([]jetstream.PubAckFuture, 0, len(rows))
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		msg, err := r.Stream.encode(Event{
			ID:               row.ID,
			WorkspaceID:      row.WorkspaceID,
			AggregateType:    row.AggregateType,
			AggregateID:      row.AggregateID,
			AggregateVersion: row.AggregateVersion,
			Type:             row.Type,
			ActorID:          row.ActorID,
			OccurredAt:       row.OccurredAt,
			Payload:          row.Payload,
		})
		if err != nil {
			return 0, fmt.Errorf("encode %s: %w", row.ID, err)
		}
		f, err := r.JetStream.PublishMsgAsync(msg)
		if err != nil {
			return 0, fmt.Errorf("publish %s: %w", row.ID, err)
		}
		futures = append(futures, f)
		ids = append(ids, row.ID)
	}

	select {
	case <-r.JetStream.PublishAsyncComplete():
	case <-ctx.Done():
		return 0, ctx.Err()
	}
	var errs []error
	for _, f := range futures {
		select {
		case <-f.Ok():
		case err := <-f.Err():
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		// Nothing is marked: the batch is retried, and JetStream drops the
		// copies it already stored (same Nats-Msg-Id).
		return 0, fmt.Errorf("publish: %w", err)
	}

	if err := q.MarkPublished(ctx, ids); err != nil {
		return 0, fmt.Errorf("mark published: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	if r.Metrics != nil {
		r.Metrics.published.Add(float64(len(ids)))
	}
	r.Log.Debug().Int("events", len(ids)).Msg("relayed")
	return len(ids), nil
}

// listen holds a dedicated connection on LISTEN events and signals wake on
// every notification. Failures only cost latency: Run also polls.
func (r *Relay) listen(ctx context.Context, wake chan<- struct{}) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := r.listenOnce(ctx, wake)
		if ctx.Err() != nil {
			return
		}
		r.Log.Warn().Err(err).Dur("retry_in", backoff).Msg("outbox listener lost")
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
}

func (r *Relay) listenOnce(ctx context.Context, wake chan<- struct{}) error {
	conn, err := r.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return err
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "UNLISTEN "+notifyChannel) }()
	for {
		if _, err := conn.Conn().WaitForNotification(ctx); err != nil {
			return err
		}
		select {
		case wake <- struct{}{}:
		default: // a wake-up is already pending
		}
	}
}

func (r *Relay) observe(ctx context.Context) {
	if r.Metrics == nil {
		return
	}
	b, err := db.New(r.Pool).OutboxBacklog(ctx)
	if err != nil {
		r.Log.Debug().Err(err).Msg("outbox backlog")
		return
	}
	r.Metrics.pending.Set(float64(b.Pending))
	r.Metrics.oldest.Set(b.OldestSeconds)
}
