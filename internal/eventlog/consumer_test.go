package eventlog_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestConsumerHandlesMatchingEventsInOrderAndRetries(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)

	issues, _ := issueEvents(3)
	iteration := uuid.Must(uuid.NewV7())
	completed := eventlog.Event{WorkspaceID: workspace, AggregateType: "iteration", AggregateID: iteration, AggregateVersion: 1,
		Type: "IterationCompleted", Payload: eventlog.Payload(map[string]string{})}
	started := completed
	started.Type = "IterationStarted"
	appendCommitted(t, pool, append(issues, started, completed))
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}
	if _, err := relay(pool, bus).Drain(ctx); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var seen []string
	failed := false
	done := make(chan struct{})
	c := eventlog.Consumer{
		Name:   "test",
		Filter: []string{"iteration.IterationCompleted", "issue.*"},
		Handle: func(_ context.Context, e eventlog.Event) error {
			mu.Lock()
			defer mu.Unlock()
			// The second issue event fails once: it comes back, and nothing
			// after it is handled before it.
			if e.Type == "IssueRetitled" && e.AggregateVersion == 2 && !failed {
				failed = true
				return errors.New("boom")
			}
			seen = append(seen, e.Type)
			if e.Type == "IterationCompleted" {
				close(done)
			}
			return nil
		},
	}
	go func() { _ = bus.Stream.Consume(ctx, bus.JetStream, c, zerolog.Nop()) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("not all events handled")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"IssueRetitled", "IssueRetitled", "IssueRetitled", "IterationCompleted"}
	if len(seen) != len(want) || !failed {
		t.Fatalf("seen = %v (failed once: %v)", seen, failed)
	}
}

// TestConsumerFinishesInFlightOnStop: a worker stopping mid-event lets it
// finish and acknowledges it, so it isn't handled twice.
func TestConsumerFinishesInFlightOnStop(t *testing.T) {
	t.Parallel()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	issues, _ := issueEvents(2)
	appendCommitted(t, pool, issues)
	if _, err := bus.Stream.Ensure(context.Background(), bus.JetStream); err != nil {
		t.Fatal(err)
	}
	if _, err := relay(pool, bus).Drain(context.Background()); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var handled []int64
	started, release := make(chan struct{}), make(chan struct{})
	c := eventlog.Consumer{Name: "drain", Filter: []string{"issue.*"}, Handle: func(ctx context.Context, e eventlog.Event) error {
		if e.AggregateVersion == 1 {
			close(started)
			<-release
			if ctx.Err() != nil {
				return ctx.Err() // the stop cancelled it
			}
		}
		mu.Lock()
		handled = append(handled, e.AggregateVersion)
		mu.Unlock()
		return nil
	}}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = bus.Stream.Consume(ctx, bus.JetStream, c, zerolog.Nop())
		close(done)
	}()
	<-started
	stop() // the worker stops while the first event is in flight
	time.Sleep(100 * time.Millisecond)
	close(release)
	<-done

	// It was acknowledged: a new worker gets only the second.
	ctx2, stop2 := context.WithCancel(context.Background())
	defer stop2()
	c.Handle = func(_ context.Context, e eventlog.Event) error {
		mu.Lock()
		handled = append(handled, e.AggregateVersion)
		mu.Unlock()
		return nil
	}
	go func() { _ = bus.Stream.Consume(ctx2, bus.JetStream, c, zerolog.Nop()) }()
	deadline := time.Now().Add(15 * time.Second)
	for {
		mu.Lock()
		n := len(handled)
		mu.Unlock()
		if n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(handled) != 2 || handled[0] != 1 || handled[1] != 2 {
		t.Errorf("handled = %v", handled)
	}
}
