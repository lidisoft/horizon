package migrations_test

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/lidisoft/horizon/internal/migrations"
	"github.com/lidisoft/horizon/internal/testenv"
)

func TestCheckUpgrade(t *testing.T) {
	for _, c := range []struct{ last, binary, min, want string }{
		{"", "v1.4.0", "v1.3.0", ""},       // a database no release migrated
		{"v1.3.2", "dev", "v1.3.0", ""},    // a development build
		{"v1.3.2", "v1.4.0", "v1.3.0", ""}, // a regular upgrade
		{"v1.4.0", "v1.4.0", "", ""},       // again
		{"v1.2.9", "v1.4.0", "v1.3.0", "upgrade to v1.3.0 first"},
		{"v1.5.0", "v1.4.1", "", "downgrades aren't supported"},
		{"v1.4.0-rc.1", "v1.4.0", "", ""},
	} {
		err := migrations.CheckUpgrade(c.last, c.binary, c.min)
		if (c.want == "") != (err == nil) || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("CheckUpgrade(%q, %q, %q) = %v, want %q", c.last, c.binary, c.min, err, c.want)
		}
	}
}

func TestCheckRunnable(t *testing.T) {
	for _, c := range []struct {
		last, binary string
		ok           bool
	}{
		{"", "v1.0.0", true},
		{"v1.4.0", "dev", true},
		{"v1.4.0", "v1.4.2", true},
		{"v1.4.0", "v1.3.7", true}, // the old processes of a rolling upgrade
		{"v1.5.0", "v1.3.7", false},
		{"v2.0.0", "v1.9.0", false},
	} {
		if err := migrations.CheckRunnable(c.last, c.binary); (err == nil) != c.ok {
			t.Errorf("CheckRunnable(%q, %q) = %v", c.last, c.binary, err)
		}
	}
}

func TestRecordRelease(t *testing.T) {
	ctx := t.Context()
	db := stdlib.OpenDBFromPool(testenv.DB(t))
	defer db.Close()
	if got, err := migrations.LastRelease(ctx, db); err != nil || got != "" {
		t.Fatalf("fresh = %q, %v", got, err)
	}
	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		if err := migrations.RecordRelease(ctx, db, v, "abc123"); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := migrations.LastRelease(ctx, db); err != nil || got != "v1.1.0" {
		t.Errorf("last = %q, %v", got, err)
	}
}
