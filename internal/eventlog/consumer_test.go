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
