package config

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

const testKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" // 32 bytes

func env(vars map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		v, ok := vars[strings.TrimPrefix(name, Prefix)]
		return v, ok
	}
}

func complete() map[string]string {
	return map[string]string{
		"DATABASE_URL":   "postgres://horizon:horizon@localhost:5432/horizon",
		"NATS_URL":       "nats://localhost:4222",
		"S3_ENDPOINT":    "http://localhost:9000",
		"S3_BUCKET":      "horizon",
		"S3_ACCESS_KEY":  "access",
		"S3_SECRET_KEY":  "secret",
		"SMTP_URL":       "smtp://localhost:1025",
		"SMTP_FROM":      "Horizon <horizon@example.com>",
		"BASE_URL":       "http://localhost:8090",
		"SECRET_KEY":     testKey,
		"SESSION_SECRET": strings.Repeat("s", 32),
	}
}

func TestLoadServeDefaults(t *testing.T) {
	c, err := Load(env(complete()), NeedServe)
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != DefaultHTTPAddr || c.MetricsAddr != DefaultMetricsAddr {
		t.Errorf("addresses = %q, %q", c.HTTPAddr, c.MetricsAddr)
	}
	if c.EventsRetention != 168*time.Hour {
		t.Errorf("retention = %s", c.EventsRetention)
	}
	if c.LogLevel != zerolog.InfoLevel || c.LogFormat != "json" {
		t.Errorf("logging = %s, %s", c.LogLevel, c.LogFormat)
	}
	if c.S3.Region != DefaultS3Region || c.S3.Endpoint.Host != "localhost:9000" {
		t.Errorf("s3 = %+v", c.S3)
	}
	if len(c.SecretKey) != 32 || c.SMTPFrom.Address != "horizon@example.com" {
		t.Errorf("secrets = %d bytes, from %v", len(c.SecretKey), c.SMTPFrom)
	}
}

func TestLoadListsEveryProblem(t *testing.T) {
	vars := complete()
	delete(vars, "DATABASE_URL")
	delete(vars, "S3_BUCKET")
	vars["SECRET_KEY"] = "c2hvcnQ="
	vars["EVENTS_RETENTION"] = "a week"
	vars["LOG_FORMAT"] = "xml"

	_, err := Load(env(vars), NeedServe)
	var cerr *Error
	if !errors.As(err, &cerr) {
		t.Fatalf("err = %v, want *Error", err)
	}
	want := []string{
		"HORIZON_DATABASE_URL: required",
		"HORIZON_EVENTS_RETENTION: must be a positive Go duration",
		"HORIZON_LOG_FORMAT: must be one of json, console",
		"HORIZON_S3_BUCKET: required",
		"HORIZON_SECRET_KEY: must decode to 32 bytes, got 5",
	}
	if len(cerr.Problems) != len(want) {
		t.Fatalf("problems = %q", cerr.Problems)
	}
	for i, w := range want {
		if !strings.HasPrefix(cerr.Problems[i], w) {
			t.Errorf("problem %d = %q, want prefix %q", i, cerr.Problems[i], w)
		}
	}
}

func TestLoadMigrateNeedsOnlyDatabase(t *testing.T) {
	c, err := Load(env(map[string]string{"DATABASE_URL": "postgres://localhost/horizon"}), NeedMigrate)
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "postgres://localhost/horizon" {
		t.Errorf("database url = %q", c.DatabaseURL)
	}
}

func TestLoadValidatesOptionalValuesWhenPresent(t *testing.T) {
	vars := map[string]string{
		"DATABASE_URL": "postgres://localhost/horizon",
		"S3_ENDPOINT":  "ftp://files",
	}
	_, err := Load(env(vars), NeedMigrate)
	if err == nil || !strings.Contains(err.Error(), "HORIZON_S3_ENDPOINT: scheme must be one of http, https") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadPreviousKeys(t *testing.T) {
	vars := complete()
	vars["SECRET_KEY_PREVIOUS"] = testKey + ", " + testKey
	c, err := Load(env(vars), NeedServe)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.SecretKeysPrevious) != 2 {
		t.Errorf("previous keys = %d", len(c.SecretKeysPrevious))
	}
}
