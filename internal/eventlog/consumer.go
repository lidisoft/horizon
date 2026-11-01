package eventlog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"
)

// Consumer is a durable JetStream consumer of domain events
// (docs/architecture.md#workers): each event it matches is handled at least
// once, in stream order, by one worker at a time. Handlers must be
// idempotent: an event is redelivered when its handler fails or the worker
// stops before acknowledging it.
type Consumer struct {
	// Name is the durable name, shared by every worker replica.
	Name string
	// Filter matches events by aggregate type and event type, as in
	// "iteration.IterationCompleted"; "*" matches any.
	Filter []string
	// Handle processes one event. An error retries it later, with backoff,
	// up to MaxDeliver times; then it is logged and left behind.
	Handle func(ctx context.Context, e Event) error
}

// Consumer delivery limits.
const (
	MaxDeliver  = 10
	ackWait     = time.Minute
	fetchWait   = 5 * time.Second
	maxInFlight = 1 // in order: one event at a time
)

// backoff is the redelivery delay after each failed attempt (the last one
// repeats).
var backoff = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute}

// subjects turns the filters into stream subjects.
func (s Stream) subjects(filters []string) []string {
	out := make([]string, len(filters))
	for i, f := range filters {
		aggregate, typ := "*", "*"
		if before, after, ok := strings.Cut(f, "."); ok {
			aggregate, typ = before, after
		}
		out[i] = fmt.Sprintf("%s.*.%s.*.%s", s.Prefix, aggregate, typ)
	}
	return out
}

// Consume runs c until ctx ends: it creates or updates the durable
// consumer (starting at the beginning of the stream the first time, which
// idempotent handlers allow), then fetches and handles events one by one.
func (s Stream) Consume(ctx context.Context, js jetstream.JetStream, c Consumer, log zerolog.Logger) error {
	if _, err := s.Ensure(ctx, js); err != nil {
		return err
	}
	cons, err := js.CreateOrUpdateConsumer(ctx, s.Name, jetstream.ConsumerConfig{
		Durable:        c.Name,
		Description:    "Horizon " + c.Name + " worker",
		FilterSubjects: s.subjects(c.Filter),
		DeliverPolicy:  jetstream.DeliverAllPolicy,
		AckPolicy:      jetstream.AckExplicitPolicy,
		AckWait:        ackWait,
		MaxDeliver:     MaxDeliver,
		MaxAckPending:  maxInFlight,
	})
	if err != nil {
		return fmt.Errorf("eventlog: consumer %s: %w", c.Name, err)
	}
	log = log.With().Str("consumer", c.Name).Logger()
	for ctx.Err() == nil {
		batch, err := cons.Fetch(1, jetstream.FetchMaxWait(fetchWait))
		if err != nil {
			if ctx.Err() == nil {
				log.Warn().Err(err).Msg("fetch failed")
				sleep(ctx, time.Second)
			}
			continue
		}
		for msg := range batch.Messages() {
			s.handle(ctx, c, msg, log)
		}
		if err := batch.Error(); err != nil && !errors.Is(err, jetstream.ErrNoMessages) && ctx.Err() == nil {
			log.Warn().Err(err).Msg("fetch failed")
		}
	}
	return nil
}

func (s Stream) handle(ctx context.Context, c Consumer, msg jetstream.Msg, log zerolog.Logger) {
	e, err := Decode(msg.Data())
	if err != nil {
		log.Error().Err(err).Str("subject", msg.Subject()).Msg("undecodable event, dropped")
		_ = msg.Term()
		return
	}
	if err := c.Handle(ctx, e); err != nil {
		if ctx.Err() != nil {
			return // redelivered after the restart
		}
		attempt := uint64(1)
		if md, mdErr := msg.Metadata(); mdErr == nil {
			attempt = md.NumDelivered
		}
		ev := log.Warn()
		if attempt >= MaxDeliver {
			ev = log.Error()
		}
		ev.Err(err).Stringer("event", e.ID).Str("type", e.Type).Uint64("attempt", attempt).Msg("handler failed")
		_ = msg.NakWithDelay(backoff[min(int(attempt), len(backoff))-1])
		return
	}
	if err := msg.Ack(); err != nil && ctx.Err() == nil {
		log.Warn().Err(err).Stringer("event", e.ID).Msg("ack failed")
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
