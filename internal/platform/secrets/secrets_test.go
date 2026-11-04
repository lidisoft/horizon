package secrets_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/lidisoft/horizon/internal/platform/secrets"
)

func TestSealOpenRotate(t *testing.T) {
	oldKey, newKey := bytes.Repeat([]byte{1}, 32), bytes.Repeat([]byte{2}, 32)
	before, err := secrets.New(oldKey)
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := before.Seal("whsec_abc", "webhooks:1")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := before.Open(sealed, "webhooks:1"); err != nil || got != "whsec_abc" {
		t.Fatalf("open = %q, %v", got, err)
	}
	if _, err := before.Open(sealed, "webhooks:2"); err == nil {
		t.Error("opened under another row")
	}

	// Rotated: the old key still opens; new values use the new key.
	after, err := secrets.New(newKey, oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := after.Open(sealed, "webhooks:1"); err != nil || got != "whsec_abc" {
		t.Fatalf("open after rotation = %q, %v", got, err)
	}
	if after.Current(sealed) {
		t.Error("old value counted as current")
	}
	again, _ := after.Seal("whsec_abc", "webhooks:1")
	if !after.Current(again) || again == sealed {
		t.Errorf("resealed = %q", again)
	}
	// The old key alone can't open it.
	if _, err := before.Open(again, "webhooks:1"); !errors.Is(err, secrets.ErrUnknownKey) {
		t.Errorf("err = %v", err)
	}
}
