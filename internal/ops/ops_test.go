package ops_test

import (
	"bytes"
	"context"
	"errors"
	"net/url"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/ops"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/s3"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/testenv"
	"github.com/lidisoft/horizon/internal/workspace"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Main(m, testenv.Postgres|testenv.NATS|testenv.S3Storage))
}

const password = "correct horse battery"

// TestReconcile: after a restore, objects no row references go, missing
// ones are reported, and the event stream starts over without its
// consumers.
func TestReconcile(t *testing.T) {
	ctx := context.Background()
	pool := testenv.DB(t)
	store, err := s3.New(testenv.S3(t))
	if err != nil {
		t.Fatal(err)
	}
	bus := testenv.JetStream(t)

	// An issue with two attachments: one whose file exists, one whose file
	// the restored bucket lost.
	base := &url.URL{Scheme: "https", Host: "horizon.example.com"}
	ids := identity.New(identity.Options{Pool: pool, Log: zerolog.Nop(), BaseURL: base, SessionSecret: "test-session-secret-at-least-32-chars"})
	acc := access.New(pool, nil)
	cfg := configdef.New(pool, acc, nil, nil)
	wss := workspace.New(workspace.Options{Pool: pool, Log: zerolog.Nop(), Identity: ids, Access: acc, Mail: &platformmail.Memory{}, BaseURL: base,
		OnCreate: []workspace.CreateHook{cfg.SeedDefaults}})
	projects := project.New(pool, acc, nil)
	issues := issue.New(pool, acc, cfg, projects, nil)
	if _, err := ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: "owner@example.com", Name: "Owner", Password: password}); err != nil {
		t.Fatal(err)
	}
	in, _ := ids.Login(ctx, "owner@example.com", password, identity.Client{})
	owner, _ := ids.Authenticate(ctx, in.Token)
	acme, err := wss.Create(ctx, uuid.NullUUID{}, workspace.NewWorkspace{Slug: "acme", Name: "Acme", OwnerID: owner.User.ID})
	if err != nil {
		t.Fatal(err)
	}
	wc, _ := wss.Enter(ctx, owner, "acme")
	key, name, vis := "HZN", "Horizon", project.VisibilityWorkspace
	if _, err := projects.Create(ctx, wc, project.Input{Key: &key, Name: &name, Visibility: &vis}); err != nil {
		t.Fatal(err)
	}
	pc, _ := projects.Enter(ctx, wc, "HZN")
	r, _ := cfg.Resolve(ctx, pool, acme.ID, uuid.NullUUID{UUID: pc.Project.ID, Valid: true})
	i, err := issues.Create(ctx, pc, issue.NewIssue{TypeID: r.IssueTypes[0].ID, Title: "Outbox relay"})
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"kept", "lost"} {
		if _, err := pool.Exec(ctx, `INSERT INTO attachments (workspace_id, id, issue_id, filename, content_type, size, object_key, state, created_at, completed_at)
			VALUES ($1, $2, $3, $4, 'text/plain', 5, $4, 'ready', now(), now())`, acme.ID, uuid.Must(uuid.NewV7()), i.ID, k); err != nil {
			t.Fatal(err)
		}
	}
	for _, k := range []string{"kept", "orphan"} {
		if _, err := store.PutObject(ctx, store.Bucket, k, bytes.NewReader([]byte("hello")), 5, minio.PutObjectOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	// Events relayed, and a consumer that read them.
	if _, err := bus.Stream.Ensure(ctx, bus.JetStream); err != nil {
		t.Fatal(err)
	}
	relay := &eventlog.Relay{Pool: pool, JetStream: bus.JetStream, Stream: bus.Stream, Log: zerolog.Nop(), BatchSize: 100}
	if _, err := relay.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := bus.JetStream.CreateOrUpdateConsumer(ctx, bus.Stream.Name, jetstream.ConsumerConfig{Durable: "notify", AckPolicy: jetstream.AckExplicitPolicy}); err != nil {
		t.Fatal(err)
	}

	// Objects just written are spared: an upload may be in flight.
	rep, err := ops.Reconcile(ctx, pool, store, bus.JetStream, bus.Stream, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Orphans) != 0 || !slices.Equal(rep.Missing, []string{"lost"}) {
		t.Errorf("fresh report = %+v", rep)
	}

	// A dry run an hour later changes nothing.
	later := time.Now().Add(2 * time.Hour)
	rep, err = ops.Reconcile(ctx, pool, store, bus.JetStream, bus.Stream, true, later)
	if err != nil || !slices.Equal(rep.Orphans, []string{"orphan"}) {
		t.Fatalf("dry run = %+v, %v", rep, err)
	}
	if _, err := store.Stat(ctx, "orphan"); err != nil {
		t.Errorf("the dry run deleted: %v", err)
	}

	rep, err = ops.Reconcile(ctx, pool, store, bus.JetStream, bus.Stream, false, later)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Purged == 0 || !slices.Equal(rep.Consumers, []string{"notify"}) {
		t.Errorf("report = %+v", rep)
	}
	if _, err := store.Stat(ctx, "orphan"); !errors.Is(err, s3.ErrNotFound) {
		t.Errorf("orphan: %v", err)
	}
	if _, err := store.Stat(ctx, "kept"); err != nil {
		t.Errorf("kept: %v", err)
	}
	st, err := bus.JetStream.Stream(ctx, bus.Stream.Name)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := st.Info(ctx)
	if info.State.Msgs != 0 || info.State.Consumers != 0 {
		t.Errorf("stream = %+v", info.State)
	}
}
