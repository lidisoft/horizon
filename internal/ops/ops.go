// Package ops holds the instance operations of the admin CLI that span
// modules (docs/operations.md#admin-cli): reconcile after a restore, and
// rekey after a SECRET_KEY rotation.
package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/platform/s3"
)

// orphanAge spares objects younger than this: an upload in flight has a
// row, but a clock skew shouldn't cost it.
const orphanAge = time.Hour

// Storage is the object storage reconcile reads and cleans.
type Storage interface {
	List(ctx context.Context, fn func(s3.Object) error) error
	Delete(ctx context.Context, keys []string) error
}

// ReconcileReport tells what reconcile found and did.
type ReconcileReport struct {
	// Orphans are objects no row references (deleted unless dry run).
	Orphans []string
	// Missing are attachment keys whose object is gone (reported only:
	// their downloads fail until the files are restored).
	Missing []string
	// Purged is how many stream messages were dropped, and Consumers the
	// consumers deleted.
	Purged    uint64
	Consumers []string
}

// Reconcile brings a restored database, its bucket and JetStream back in
// step: objects no attachment references are deleted, attachments whose
// object is gone are reported, and the event stream is emptied with its
// consumers, so workers resume from the restored outbox: events the
// database doesn't have aren't handled, and those it hasn't published are
// published again by the relay. dryRun reports without changing anything.
// Run it before starting serve and worker.
func Reconcile(ctx context.Context, pool *pgxpool.Pool, store Storage, js jetstream.JetStream, stream eventlog.Stream, dryRun bool, now time.Time) (ReconcileReport, error) {
	var r ReconcileReport
	rows, err := pool.Query(ctx, `SELECT object_key, state FROM attachments`)
	if err != nil {
		return r, err
	}
	referenced := map[string]bool{}
	ready := map[string]bool{}
	for rows.Next() {
		var key, state string
		if err := rows.Scan(&key, &state); err != nil {
			rows.Close()
			return r, err
		}
		referenced[key] = true
		if state == "ready" {
			ready[key] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}
	seen := map[string]bool{}
	err = store.List(ctx, func(o s3.Object) error {
		seen[o.Key] = true
		if !referenced[o.Key] && now.Sub(o.Modified) > orphanAge {
			r.Orphans = append(r.Orphans, o.Key)
		}
		return nil
	})
	if err != nil {
		return r, err
	}
	for key := range ready {
		if !seen[key] {
			r.Missing = append(r.Missing, key)
		}
	}
	if dryRun {
		return r, nil
	}
	if err := store.Delete(ctx, r.Orphans); err != nil {
		return r, err
	}
	if r.Purged, r.Consumers, err = stream.Reset(ctx, js); err != nil {
		return r, fmt.Errorf("reset the event stream: %w", err)
	}
	return r, nil
}

// Rekeyer reseals one module's secrets with the current key.
type Rekeyer struct {
	Name  string
	Rekey func(ctx context.Context) (int, error)
}

// Rekey reseals every module's secrets, returning the count per module.
func Rekey(ctx context.Context, modules []Rekeyer) (map[string]int, error) {
	out := map[string]int{}
	for _, m := range modules {
		n, err := m.Rekey(ctx)
		out[m.Name] = n
		if err != nil {
			return out, fmt.Errorf("rekey %s: %w", m.Name, err)
		}
	}
	return out, nil
}
