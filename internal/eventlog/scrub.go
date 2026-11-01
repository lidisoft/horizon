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
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog/db"
)

// Scrub removes the stored events of hard-deleted aggregates in the
// caller's transaction and queues their JetStream subjects for the purge
// job (ADR-0012). objects are object storage keys to delete with them.
func Scrub(ctx context.Context, tx db.DBTX, workspaceID uuid.NullUUID, aggregateType string, ids []uuid.UUID, objects ...string) error {
	if len(ids) == 0 {
		return nil
	}
	q := db.New(tx)
	if err := q.DeleteAggregateEvents(ctx, db.DeleteAggregateEventsParams{AggregateType: aggregateType, Ids: ids}); err != nil {
		return fmt.Errorf("eventlog: scrub: %w", err)
	}
	for i, id := range ids {
		var keys []string
		if i == 0 {
			keys = objects
		}
		if err := queuePurge(ctx, q, workspaceID, aggregateType, id, uuid.NullUUID{}, keys); err != nil {
			return err
		}
	}
	return nil
}

// Tombstone scrubs e's aggregate like Scrub, then appends e: the one event
// that remains of it, in PostgreSQL and in the stream.
func Tombstone(ctx context.Context, tx db.DBTX, e Event, objects ...string) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.Must(uuid.NewV7())
	}
	q := db.New(tx)
	if err := q.DeleteAggregateEvents(ctx, db.DeleteAggregateEventsParams{AggregateType: e.AggregateType, Ids: []uuid.UUID{e.AggregateID}}); err != nil {
		return fmt.Errorf("eventlog: scrub: %w", err)
	}
	if err := Append(ctx, tx, e); err != nil {
		return err
	}
	return queuePurge(ctx, q, e.WorkspaceID, e.AggregateType, e.AggregateID, uuid.NullUUID{UUID: e.ID, Valid: true}, objects)
}

func queuePurge(ctx context.Context, q *db.Queries, workspaceID uuid.NullUUID, aggregateType string, id uuid.UUID, tombstone uuid.NullUUID, objects []string) error {
	if objects == nil {
		objects = []string{}
	}
	err := q.QueuePurge(ctx, db.QueuePurgeParams{
		ID: uuid.Must(uuid.NewV7()), WorkspaceID: workspaceID, AggregateType: aggregateType, AggregateID: id,
		TombstoneID: tombstone, Objects: objects, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("eventlog: queue purge: %w", err)
	}
	return nil
}

// Purger is the purge job: it removes scrubbed aggregates' messages from
// the stream, keeping each tombstone, and deletes their objects.
type Purger struct {
	Pool      *pgxpool.Pool
	JetStream jetstream.JetStream
	Stream    Stream
	Log       zerolog.Logger
	// DeleteObjects removes object storage keys; nil when nothing stores
	// objects yet.
	DeleteObjects func(ctx context.Context, keys []string) error
	BatchSize     int
}

// maxPurgeAttempts: a purge failing this often is left for an operator
// (it stays in the table, out of the way of the others).
const maxPurgeAttempts = 20

// Run purges what is ready until the queue is drained.
func (p *Purger) Run(ctx context.Context) error {
	for {
		n, err := p.batch(ctx)
		if err != nil || n == 0 {
			return err
		}
	}
}

func (p *Purger) batch(ctx context.Context) (int, error) {
	size := p.BatchSize
	if size <= 0 {
		size = 100
	}
	tx, err := p.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	q := db.New(tx)
	rows, err := q.ClaimPurges(ctx, int32(size))
	if err != nil {
		return 0, fmt.Errorf("purge: claim: %w", err)
	}
	st, err := p.JetStream.Stream(ctx, p.Stream.Name)
	if err != nil {
		return 0, fmt.Errorf("purge: stream: %w", err)
	}
	done := 0
	for _, r := range rows {
		if r.Attempts >= maxPurgeAttempts {
			continue
		}
		if err := p.purge(ctx, st, r); err != nil {
			p.Log.Warn().Err(err).Str("aggregate", r.AggregateType).Stringer("id", r.AggregateID).Msg("purge failed")
			if err := q.FailedPurge(ctx, r.ID); err != nil {
				return 0, err
			}
			continue
		}
		if err := q.DonePurge(ctx, r.ID); err != nil {
			return 0, err
		}
		done++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return done, nil
}

func (p *Purger) purge(ctx context.Context, st jetstream.Stream, r db.ClaimPurgesRow) error {
	opts := []jetstream.StreamPurgeOpt{jetstream.WithPurgeSubject(p.Stream.AggregateSubjects(r.WorkspaceID, r.AggregateType, r.AggregateID))}
	if r.TombstoneID.Valid && r.TombstoneType != "" {
		subject := p.Stream.Subject(Event{WorkspaceID: r.WorkspaceID, AggregateType: r.AggregateType, AggregateID: r.AggregateID, Type: r.TombstoneType})
		msg, err := st.GetLastMsgForSubject(ctx, subject)
		switch {
		case errors.Is(err, jetstream.ErrMsgNotFound):
			// Expired already: nothing to keep.
		case err != nil:
			return err
		default:
			// Everything before the tombstone.
			opts = append(opts, jetstream.WithPurgeSequence(msg.Sequence))
		}
	}
	if err := st.Purge(ctx, opts...); err != nil {
		return err
	}
	if len(r.Objects) > 0 && p.DeleteObjects != nil {
		return p.DeleteObjects(ctx, r.Objects)
	}
	return nil
}
