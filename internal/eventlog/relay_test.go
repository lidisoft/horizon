package eventlog_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres|testenv.NATS)) }

var workspace = uuid.NullUUID{UUID: uuid.MustParse("01929b2a-0000-7000-8000-0000000000aa"), Valid: true}

// issueEvents returns n events on one issue, versions 1..n.
func issueEvents(n int) ([]eventlog.Event, uuid.UUID) {
	id := uuid.Must(uuid.NewV7())
	events := make([]eventlog.Event, n)
	for i := range events {
		events[i] = eventlog.Event{
			WorkspaceID:      workspace,
			AggregateType:    "issue",
			AggregateID:      id,
			AggregateVersion: int64(i + 1),
			Type:             "IssueRetitled",
			Payload:          eventlog.Payload(map[string]int{"n": i + 1}),
		}
	}
	return events, id
}

func appendCommitted(t *testing.T, pool *pgxpool.Pool, events []eventlog.Event) {
	t.Helper()
	err := postgres.InTx(context.Background(), pool, func(tx pgx.Tx) error {
		return eventlog.Append(context.Background(), tx, events...)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAppendCommitsAndRollsBackWithTheTransaction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testenv.DB(t)

	events, id := issueEvents(2)
	appendCommitted(t, pool, events)

	rolledBack, otherID := issueEvents(1)
	errBoom := errors.New("boom")
	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if err := eventlog.Append(ctx, tx, rolledBack...); err != nil {
			return err
		}
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}

	got, err := eventlog.History(ctx, pool, "issue", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].AggregateVersion != 1 || got[1].Type != "IssueRetitled" || got[0].ID == uuid.Nil {
		t.Fatalf("history = %+v", got)
	}
	if none, _ := eventlog.History(ctx, pool, "issue", otherID); len(none) != 0 {
		t.Fatalf("rolled back events stored: %+v", none)
	}
}

func TestAppendRejectsInvalidEvents(t *testing.T) {
	t.Parallel()
	pool := testenv.DB(t)
	events, _ := issueEvents(1)
	events[0].Type = "issue.retitled"
	if err := eventlog.Append(context.Background(), pool, events...); err == nil {
		t.Fatal("invalid event appended")
	}
}

func TestAppendNotifiesOnCommitOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testenv.DB(t)

	listener, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Release()
	if _, err := listener.Exec(ctx, "LISTEN events"); err != nil {
		t.Fatal(err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	events, _ := issueEvents(3)
	if err := eventlog.Append(ctx, tx, events...); err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	_, err = listener.Conn().WaitForNotification(short)
	cancel()
	if err == nil {
		t.Fatal("notified before commit")
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	wait, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	n, err := listener.Conn().WaitForNotification(wait)
	if err != nil || n.Channel != "events" {
		t.Fatalf("notification = %+v, %v", n, err)
	}
}

func relay(pool *pgxpool.Pool, bus testenv.Bus) *eventlog.Relay {
	return &eventlog.Relay{
		Pool:      pool,
		JetStream: bus.JetStream,
		Stream:    bus.Stream,
		Log:       zerolog.Nop(),
		BatchSize: 100,
	}
}

// fetchAll reads every message in the test's stream.
func fetchAll(t *testing.T, bus testenv.Bus, want int) []jetstream.Msg {
	t.Helper()
	ctx := context.Background()
	cons, err := bus.JetStream.CreateConsumer(ctx, bus.Stream.Name, jetstream.ConsumerConfig{AckPolicy: jetstream.AckNonePolicy})
	if err != nil {
		t.Fatal(err)
	}
	var msgs []jetstream.Msg
	deadline := time.Now().Add(5 * time.Second)
	for len(msgs) < want && time.Now().Before(deadline) {
		batch, err := cons.FetchNoWait(want - len(msgs))
		if err != nil {
			t.Fatal(err)
		}
		for m := range batch.Messages() {
			msgs = append(msgs, m)
		}
		if len(msgs) < want {
			time.Sleep(50 * time.Millisecond)
		}
	}
	return msgs
}

func streamCount(t *testing.T, bus testenv.Bus) uint64 {
	t.Helper()
	st, err := bus.JetStream.Stream(context.Background(), bus.Stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	info, err := st.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return info.State.Msgs
}

func TestRelayPublishesEveryEventOnceInOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	r := relay(pool, bus)
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}

	events, id := issueEvents(250) // three batches
	appendCommitted(t, pool, events)

	n, err := r.Drain(ctx)
	if err != nil || n != 250 {
		t.Fatalf("drain = %d, %v", n, err)
	}
	if n, err := r.Drain(ctx); err != nil || n != 0 {
		t.Fatalf("second drain = %d, %v", n, err)
	}

	msgs := fetchAll(t, bus, 250)
	if len(msgs) != 250 {
		t.Fatalf("stream holds %d messages", len(msgs))
	}
	for i, m := range msgs {
		e, err := eventlog.Decode(m.Data())
		if err != nil {
			t.Fatal(err)
		}
		if e.AggregateVersion != int64(i+1) || e.AggregateID != id {
			t.Fatalf("message %d is version %d of %s", i, e.AggregateVersion, e.AggregateID)
		}
		if m.Subject() != bus.Stream.Subject(e) || m.Headers().Get(jetstream.MsgIDHeader) != e.ID.String() {
			t.Fatalf("message %d: subject %s, id %s", i, m.Subject(), m.Headers().Get(jetstream.MsgIDHeader))
		}
	}

	var pending int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE published_at IS NULL").Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("unpublished = %d, %v", pending, err)
	}
}

func TestConcurrentRelaysPublishEachEventOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		events, _ := issueEvents(100)
		appendCommitted(t, pool, events)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	total := 0
	for range 4 {
		wg.Go(func() {
			r := relay(pool, bus)
			r.BatchSize = 25
			n, err := r.Drain(ctx)
			if err != nil {
				t.Error(err)
			}
			mu.Lock()
			total += n
			mu.Unlock()
		})
	}
	wg.Wait()
	if total != 500 {
		t.Fatalf("relays published %d events, want 500", total)
	}
	if got := streamCount(t, bus); got != 500 {
		t.Fatalf("stream holds %d messages, want 500", got)
	}
}

func TestRelayRunWakesOnCommit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	r := relay(pool, bus)
	r.PollInterval = time.Hour // only NOTIFY can wake it

	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()

	// Let Run create the stream, drain the empty outbox and start listening.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := bus.JetStream.Stream(ctx, bus.Stream.Name); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("stream not created")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)

	events, _ := issueEvents(3)
	appendCommitted(t, pool, events)
	deadline = time.Now().Add(3 * time.Second)
	for streamCount(t, bus) < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("relay did not wake: %d messages", streamCount(t, bus))
		}
		time.Sleep(20 * time.Millisecond)
	}
}
