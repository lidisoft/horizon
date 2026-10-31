// Package config loads the process configuration from HORIZON_* environment
// variables (docs/operations.md#configuration).
//
// It also hosts the workspace → project configuration resolver in later
// phases; this file is only about the process.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Prefix is prepended to every environment variable name.
const Prefix = "HORIZON_"

// Defaults for optional settings.
const (
	DefaultHTTPAddr        = ":8080"
	DefaultMetricsAddr     = ":9090"
	DefaultEventsRetention = 168 * time.Hour
	DefaultLogLevel        = "info"
	DefaultLogFormat       = "json"
	DefaultS3Region        = "us-east-1"
)

// secretKeyLen is the AES-256 key length SECRET_KEY must decode to.
const secretKeyLen = 32

// minSessionSecretLen is the shortest accepted SESSION_SECRET.
const minSessionSecretLen = 32

// Need selects the groups of settings a command requires. Settings outside the
// selected groups are still validated when present, but not required.
type Need uint8

// Setting groups.
const (
	NeedDatabase Need = 1 << iota
	NeedNATS
	NeedS3
	NeedSMTP
	NeedSecrets
	NeedHTTP
)

// Process presets.
const (
	// NeedServe is what `horizon serve` requires.
	NeedServe = NeedDatabase | NeedNATS | NeedS3 | NeedSMTP | NeedSecrets | NeedHTTP
	// NeedWorker is what `horizon worker` requires.
	NeedWorker = NeedDatabase | NeedNATS | NeedS3 | NeedSMTP | NeedSecrets
	// NeedMigrate is what `horizon migrate` requires.
	NeedMigrate = NeedDatabase
)

// S3 holds object storage settings.
type S3 struct {
	Endpoint  *url.URL
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
}

// Config is the validated process configuration.
type Config struct {
	DatabaseURL string
	NatsURL     string
	S3          S3
	SMTPURL     *url.URL
	SMTPFrom    *mail.Address
	BaseURL     *url.URL

	// SecretKey encrypts secrets at rest; SecretKeysPrevious only decrypt.
	SecretKey          []byte
	SecretKeysPrevious [][]byte
	SessionSecret      string

	EventsRetention time.Duration
	LogLevel        zerolog.Level
	LogFormat       string
	HTTPAddr        string
	MetricsAddr     string
}

// LookupFunc reads one environment variable; os.LookupEnv in production.
type LookupFunc func(name string) (string, bool)

// Error lists every problem found, so operators fix them in one pass.
type Error struct {
	Problems []string
}

func (e *Error) Error() string {
	return "invalid configuration:\n  - " + strings.Join(e.Problems, "\n  - ")
}

// FromEnv loads the configuration from the process environment.
func FromEnv(need Need) (Config, error) {
	return Load(os.LookupEnv, need)
}

// Load reads and validates the configuration through lookup.
func Load(lookup LookupFunc, need Need) (Config, error) {
	l := loader{lookup: lookup}
	var c Config

	c.LogLevel = l.level("LOG_LEVEL", DefaultLogLevel)
	c.LogFormat = l.oneOf("LOG_FORMAT", DefaultLogFormat, "json", "console")
	c.HTTPAddr = l.optional("HTTP_ADDR", DefaultHTTPAddr)
	c.MetricsAddr = l.optional("METRICS_ADDR", DefaultMetricsAddr)
	c.EventsRetention = l.duration("EVENTS_RETENTION", DefaultEventsRetention)

	c.DatabaseURL = l.urlString("DATABASE_URL", need&NeedDatabase != 0, "postgres", "postgresql")
	c.NatsURL = l.urlString("NATS_URL", need&NeedNATS != 0, "nats", "tls")

	s3 := need&NeedS3 != 0
	c.S3 = S3{
		Endpoint:  l.url("S3_ENDPOINT", s3, "http", "https"),
		Bucket:    l.str("S3_BUCKET", s3),
		Region:    l.optional("S3_REGION", DefaultS3Region),
		AccessKey: l.str("S3_ACCESS_KEY", s3),
		SecretKey: l.str("S3_SECRET_KEY", s3),
	}

	smtp := need&NeedSMTP != 0
	c.SMTPURL = l.url("SMTP_URL", smtp, "smtp", "smtps")
	c.SMTPFrom = l.address("SMTP_FROM", smtp)

	c.BaseURL = l.url("BASE_URL", need&NeedHTTP != 0 || smtp, "http", "https")

	secrets := need&NeedSecrets != 0
	c.SecretKey = l.key("SECRET_KEY", secrets)
	c.SecretKeysPrevious = l.keys("SECRET_KEY_PREVIOUS")
	c.SessionSecret = l.sessionSecret("SESSION_SECRET", secrets)

	if len(l.problems) > 0 {
		sort.Strings(l.problems)
		return Config{}, &Error{Problems: l.problems}
	}
	return c, nil
}

type loader struct {
	lookup   LookupFunc
	problems []string
}

func (l *loader) get(name string) (string, bool) {
	v, ok := l.lookup(Prefix + name)
	v = strings.TrimSpace(v)
	return v, ok && v != ""
}

func (l *loader) fail(name, format string, args ...any) {
	l.problems = append(l.problems, Prefix+name+": "+fmt.Sprintf(format, args...))
}

func (l *loader) str(name string, required bool) string {
	v, ok := l.get(name)
	if !ok && required {
		l.fail(name, "required")
	}
	return v
}

func (l *loader) optional(name, def string) string {
	if v, ok := l.get(name); ok {
		return v
	}
	return def
}

func (l *loader) oneOf(name, def string, allowed ...string) string {
	v := l.optional(name, def)
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	l.fail(name, "must be one of %s, got %q", strings.Join(allowed, ", "), v)
	return def
}

func (l *loader) level(name, def string) zerolog.Level {
	v := l.optional(name, def)
	lvl, err := zerolog.ParseLevel(v)
	if err != nil || lvl == zerolog.NoLevel {
		l.fail(name, "unknown level %q (trace, debug, info, warn, error)", v)
		return zerolog.InfoLevel
	}
	return lvl
}

func (l *loader) duration(name string, def time.Duration) time.Duration {
	v, ok := l.get(name)
	if !ok {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		l.fail(name, "must be a positive Go duration such as 168h, got %q", v)
		return def
	}
	return d
}

func (l *loader) url(name string, required bool, schemes ...string) *url.URL {
	v, ok := l.get(name)
	if !ok {
		if required {
			l.fail(name, "required")
		}
		return nil
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		l.fail(name, "must be an absolute URL")
		return nil
	}
	for _, s := range schemes {
		if u.Scheme == s {
			return u
		}
	}
	l.fail(name, "scheme must be one of %s, got %q", strings.Join(schemes, ", "), u.Scheme)
	return nil
}

// urlString validates like url but keeps the original string, for drivers
// that parse it themselves (pgx, nats).
func (l *loader) urlString(name string, required bool, schemes ...string) string {
	if u := l.url(name, required, schemes...); u != nil {
		v, _ := l.get(name)
		return v
	}
	return ""
}

func (l *loader) address(name string, required bool) *mail.Address {
	v, ok := l.get(name)
	if !ok {
		if required {
			l.fail(name, "required")
		}
		return nil
	}
	a, err := mail.ParseAddress(v)
	if err != nil {
		l.fail(name, "must be an email address such as \"Horizon <horizon@example.com>\"")
		return nil
	}
	return a
}

func decodeKey(v string) ([]byte, error) {
	k, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, errors.New("must be base64")
	}
	if len(k) != secretKeyLen {
		return nil, fmt.Errorf("must decode to %d bytes, got %d (generate one with: openssl rand -base64 32)", secretKeyLen, len(k))
	}
	return k, nil
}

func (l *loader) key(name string, required bool) []byte {
	v, ok := l.get(name)
	if !ok {
		if required {
			l.fail(name, "required (generate one with: openssl rand -base64 32)")
		}
		return nil
	}
	k, err := decodeKey(v)
	if err != nil {
		l.fail(name, "%s", err)
		return nil
	}
	return k
}

func (l *loader) keys(name string) [][]byte {
	v, ok := l.get(name)
	if !ok {
		return nil
	}
	var keys [][]byte
	for i, part := range strings.Split(v, ",") {
		k, err := decodeKey(strings.TrimSpace(part))
		if err != nil {
			l.fail(name, "key %d %s", i+1, err)
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

func (l *loader) sessionSecret(name string, required bool) string {
	v := l.str(name, required)
	if v != "" && len(v) < minSessionSecretLen {
		l.fail(name, "must be at least %d characters", minSessionSecretLen)
	}
	return v
}
