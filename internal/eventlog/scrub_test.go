package eventlog_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestScrubAndPurgeKeepTheTombstone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testenv.DB(t)
	bus := testenv.JetStream(t)
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}
	r := relay(pool, bus)
	deleted, a := issueEvents(3)
	scrubbed, b := issueEvents(2)
	kept, c := issueEvents(1)
	appendCommitted(t, pool, append(append(deleted, scrubbed...), kept...))
	if _, err := r.Drain(ctx); err != nil {
		t.Fatal(err)
	}

	tomb := eventlog.Event{WorkspaceID: workspace, AggregateType: "issue", AggregateID: a, AggregateVersion: 4, Type: "IssueDeleted"}
	err := postgres.InTx(ctx, pool, func(tx pgx.Tx) error {
		if err := eventlog.Tombstone(ctx, tx, tomb); err != nil {
			return err
		}
		return eventlog.Scrub(ctx, tx, workspace, "issue", []uuid.UUID{b})
	})
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := eventlog.History(ctx, pool, "issue", a); len(h) != 1 || h[0].Type != "IssueDeleted" {
		t.Errorf("history of a = %+v", h)
	}
	if h, _ := eventlog.History(ctx, pool, "issue", b); len(h) != 0 {
		t.Errorf("history of b = %+v", h)
	}

	p := &eventlog.Purger{Pool: pool, JetStream: bus.JetStream, Stream: bus.Stream, Log: zerolog.Nop()}
	// The tombstone isn't published yet: only b goes.
	if err := p.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if n := streamCount(t, bus); n != 4 {
		t.Errorf("after the first purge: %d messages, want 4", n)
	}
	if _, err := r.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if err := p.Run(ctx); err != nil {
		t.Fatal(err)
	}
	msgs := fetchAll(t, bus, 2)
	if len(msgs) != 2 || streamCount(t, bus) != 2 {
		t.Fatalf("left %d messages, want 2", len(msgs))
	}
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.Subject()] = true
	}
	tomb.AggregateID = a
	keep := eventlog.Event{WorkspaceID: workspace, AggregateType: "issue", AggregateID: c, Type: kept[0].Type}
	if !got[bus.Stream.Subject(tomb)] || !got[bus.Stream.Subject(keep)] {
		t.Errorf("left %v", got)
	}
	var queued int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM purges`).Scan(&queued)
	if queued != 0 {
		t.Errorf("%d purges left", queued)
	}
}
