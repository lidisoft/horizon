package identity_test

import (
	"context"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestMain(m *testing.M) { os.Exit(testenv.Main(m, testenv.Postgres)) }

const password = "correct horse battery"

// clock is a settable time source.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	pool  *pgxpool.Pool
	svc   *identity.Service
	clock *clock
	mail  *platformmail.Memory
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		pool:  testenv.DB(t),
		clock: &clock{now: time.Date(2026, 10, 31, 9, 0, 0, 0, time.UTC)},
		mail:  &platformmail.Memory{},
	}
	f.svc = identity.New(identity.Options{
		Pool:          f.pool,
		Log:           zerolog.Nop(),
		Mail:          f.mail,
		BaseURL:       &url.URL{Scheme: "https", Host: "horizon.example.com"},
		SessionSecret: "test-session-secret-at-least-32-chars",
		Now:           f.clock.Now,
	})
	return f
}

func (f fixture) user(t *testing.T, email string) identity.User {
	t.Helper()
	u, err := f.svc.CreateUser(context.Background(), uuid.NullUUID{}, identity.NewUser{Email: email, Name: "Ada Lovelace", Password: password})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

var browser = identity.Client{UserAgent: "test", IP: netip.MustParseAddr("192.0.2.10")}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	if !apperr.Is(err, code) {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func wantField(t *testing.T, err error, field, code string) {
	t.Helper()
	e, ok := apperr.As(err)
	if ok {
		for _, f := range e.Fields {
			if f.Field == field && f.Code == code {
				return
			}
		}
	}
	t.Fatalf("err = %v, want %s: %s", err, field, code)
}

func TestCreateUser(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user(t, "ada@example.com")
	if !u.HasPassword || u.Version != 1 || u.Locale != "en" {
		t.Errorf("user = %+v", u)
	}

	_, err := f.svc.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: "ADA@example.com", Name: "Twin"})
	wantField(t, err, "email", "taken")

	_, err = f.svc.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{Email: "Ada <ada@x.io>", Name: " ", Password: "short"})
	wantField(t, err, "email", "invalid")
	wantField(t, err, "name", "required")
	wantField(t, err, "password", "too_short")

	events, err := eventlog.History(ctx, f.pool, "user", u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].PublicType() != "user.created" || events[0].WorkspaceID.Valid {
		t.Errorf("events = %+v", events)
	}
	if strings.Contains(string(events[0].Payload), "ada@") {
		t.Errorf("payload carries the email: %s", events[0].Payload)
	}
}

func TestLoginAndAuthenticate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user(t, "ada@example.com")

	in, err := f.svc.Login(ctx, "  ADA@example.com ", password, browser)
	if err != nil {
		t.Fatal(err)
	}
	if in.User.ID != u.ID || in.Token == "" || in.CSRF == "" || in.Session.IP.String() != "192.0.2.10" {
		t.Errorf("signed in = %+v", in)
	}

	p, err := f.svc.Authenticate(ctx, in.Token)
	if err != nil || p.User.ID != u.ID || p.SessionID != in.Session.ID {
		t.Fatalf("authenticate = %+v, %v", p, err)
	}
	if !f.svc.ValidCSRF(p, in.CSRF) || f.svc.ValidCSRF(p, in.CSRF+"x") || f.svc.CSRFToken(p) != in.CSRF {
		t.Error("CSRF token does not round-trip")
	}

	_, err = f.svc.Authenticate(ctx, in.Token+"x")
	wantCode(t, err, "unauthenticated")

	for _, c := range []struct{ email, password string }{
		{"ada@example.com", "wrong password!"},
		{"nobody@example.com", password},
	} {
		_, err := f.svc.Login(ctx, c.email, c.password, browser)
		wantCode(t, err, "invalid_credentials")
	}
}

func TestSessionsSlideAndExpire(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.user(t, "ada@example.com")
	in, err := f.svc.Login(ctx, "ada@example.com", password, browser)
	if err != nil {
		t.Fatal(err)
	}

	// Used every 20 days, the session outlives the 30-day idle timeout.
	for range 3 {
		f.clock.Advance(20 * 24 * time.Hour)
		if _, err := f.svc.Authenticate(ctx, in.Token); err != nil {
			t.Fatalf("after 20 idle days: %v", err)
		}
	}
	f.clock.Advance(identity.SessionIdleTimeout + time.Second)
	_, err = f.svc.Authenticate(ctx, in.Token)
	wantCode(t, err, "unauthenticated")

	if err := f.svc.Cleanup(ctx); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM sessions").Scan(&n); err != nil || n != 0 {
		t.Errorf("sessions after cleanup = %d, %v", n, err)
	}
}

func TestLoginThrottle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.user(t, "ada@example.com")

	for range 10 {
		_, err := f.svc.Login(ctx, "ada@example.com", "wrong password!", browser)
		wantCode(t, err, "invalid_credentials")
		f.clock.Advance(time.Minute)
	}
	// The eleventh attempt is refused even with the right password.
	_, err := f.svc.Login(ctx, "Ada@Example.com", password, browser)
	wantCode(t, err, "too_many_attempts")
	e, _ := apperr.As(err)
	if e.RetryAfter != 5*time.Minute {
		t.Errorf("retry after %v, want 5m (window opened 10 minutes ago)", e.RetryAfter)
	}

	// Other accounts from another address are unaffected.
	f.user(t, "grace@example.com")
	other := identity.Client{IP: netip.MustParseAddr("198.51.100.7")}
	if _, err := f.svc.Login(ctx, "grace@example.com", password, other); err != nil {
		t.Fatal(err)
	}

	f.clock.Advance(5 * time.Minute)
	if _, err := f.svc.Login(ctx, "ada@example.com", password, browser); err != nil {
		t.Fatalf("after the window: %v", err)
	}
}

func TestDeactivation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user(t, "ada@example.com")
	in, err := f.svc.Login(ctx, "ada@example.com", password, browser)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.SetDeactivated(ctx, uuid.NullUUID{}, u.ID, true); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Authenticate(ctx, in.Token)
	wantCode(t, err, "unauthenticated")
	_, err = f.svc.Login(ctx, "ada@example.com", password, browser)
	wantCode(t, err, "invalid_credentials")

	if _, err := f.svc.SetDeactivated(ctx, uuid.NullUUID{}, u.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Login(ctx, "ada@example.com", password, browser); err != nil {
		t.Fatalf("after reactivation: %v", err)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.user(t, "ada@example.com")
	laptop, _ := f.svc.Login(ctx, "ada@example.com", password, browser)
	phone, _ := f.svc.Login(ctx, "ada@example.com", password, browser)
	p, err := f.svc.Authenticate(ctx, laptop.Token)
	if err != nil {
		t.Fatal(err)
	}

	wantField(t, f.svc.ChangePassword(ctx, p, "wrong password!", "a new long passphrase"), "current_password", "incorrect")
	wantField(t, f.svc.ChangePassword(ctx, p, password, "short"), "new_password", "too_short")
	if err := f.svc.ChangePassword(ctx, p, password, "a new long passphrase"); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.Authenticate(ctx, laptop.Token); err != nil {
		t.Errorf("current session revoked: %v", err)
	}
	_, err = f.svc.Authenticate(ctx, phone.Token)
	wantCode(t, err, "unauthenticated")
	if _, err := f.svc.Login(ctx, "ada@example.com", "a new long passphrase", browser); err != nil {
		t.Errorf("new password: %v", err)
	}
}

func TestSessionList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.user(t, "ada@example.com")
	a, _ := f.svc.Login(ctx, "ada@example.com", password, browser)
	f.clock.Advance(time.Minute)
	b, _ := f.svc.Login(ctx, "ada@example.com", password, identity.Client{UserAgent: "phone"})
	p, _ := f.svc.Authenticate(ctx, a.Token)

	list, err := f.svc.Sessions(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != b.Session.ID || list[0].Current || !list[1].Current || list[0].IP != nil {
		t.Fatalf("sessions = %+v", list)
	}
	wantCode(t, f.svc.RevokeSession(ctx, p, uuid.Must(uuid.NewV7())), "session_not_found")
	if err := f.svc.RevokeSession(ctx, p, b.Session.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := f.svc.RevokeOtherSessions(ctx, p); err != nil || n != 0 {
		t.Errorf("revoke others = %d, %v", n, err)
	}
	if err := f.svc.Logout(ctx, p); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.Authenticate(ctx, a.Token)
	wantCode(t, err, "unauthenticated")
}

func TestPasswordReset(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user(t, "ada@example.com")
	session, _ := f.svc.Login(ctx, "ada@example.com", password, browser)

	if err := f.svc.RequestPasswordReset(ctx, "nobody@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.RequestPasswordReset(ctx, "ADA@example.com"); err != nil {
		t.Fatal(err)
	}
	var sent []platformmail.Message
	for deadline := time.Now().Add(5 * time.Second); len(sent) == 0 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		sent = f.mail.Sent()
	}
	if len(sent) != 1 || sent[0].To.Address != "ada@example.com" {
		t.Fatalf("sent = %+v", sent)
	}
	_, token, ok := strings.Cut(sent[0].Body, "https://horizon.example.com/reset-password#token=")
	if !ok {
		t.Fatalf("no link in %q", sent[0].Body)
	}
	token = strings.Fields(token)[0]

	wantField(t, f.svc.ResetPassword(ctx, token, "short"), "password", "too_short")
	if err := f.svc.ResetPassword(ctx, token, "a new long passphrase"); err != nil {
		t.Fatal(err)
	}
	wantField(t, f.svc.ResetPassword(ctx, token, "another passphrase"), "token", "invalid_or_expired")
	_, err := f.svc.Authenticate(ctx, session.Token)
	wantCode(t, err, "unauthenticated")

	// Links expire after an hour.
	link, err := f.svc.IssuePasswordReset(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(identity.ResetTokenLifetime)
	_, stale, _ := strings.Cut(link, "#token=")
	wantField(t, f.svc.ResetPassword(ctx, stale, "another passphrase"), "token", "invalid_or_expired")

	events, _ := eventlog.History(ctx, f.pool, "user", u.ID)
	if got := events[len(events)-1]; got.PublicType() != "user.password_changed" || string(got.Payload) != `{"reset": true}` {
		t.Errorf("last event = %s %s", got.PublicType(), got.Payload)
	}
}

func TestUpdateProfile(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user(t, "ada@example.com")
	name, tz, bad := "Ada King", "Europe/Paris", "Mars/Olympus"

	got, err := f.svc.UpdateProfile(ctx, u.ID, u.Version, identity.ProfileUpdate{Name: &name, Timezone: &tz})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name || *got.Timezone != tz || got.Version != 2 {
		t.Errorf("user = %+v", got)
	}
	_, err = f.svc.UpdateProfile(ctx, u.ID, 1, identity.ProfileUpdate{Name: &name})
	wantCode(t, err, "version_conflict")
	_, err = f.svc.UpdateProfile(ctx, u.ID, 0, identity.ProfileUpdate{Timezone: &bad})
	wantField(t, err, "timezone", "unknown")

	// No change, no new version.
	same, err := f.svc.UpdateProfile(ctx, u.ID, 0, identity.ProfileUpdate{Name: &name})
	if err != nil || same.Version != 2 {
		t.Errorf("no-op update = %+v, %v", same, err)
	}
}
