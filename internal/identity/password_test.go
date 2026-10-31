package identity

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	h, err := hashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %s", h)
	}
	for pw, want := range map[string]bool{"correct horse battery": true, "correct horse batter": false, "": false} {
		got, err := verifyPassword(h, pw)
		if err != nil || got != want {
			t.Errorf("verify(%q) = %v, %v; want %v", pw, got, err, want)
		}
	}
	h2, _ := hashPassword("correct horse battery")
	if h == h2 {
		t.Error("two hashes of one password are equal: salt not random")
	}
}

func TestVerifyPasswordKeepsStoredParameters(t *testing.T) {
	h, err := hashWith(argonParams{memory: 8 * 1024, time: 1, threads: 2, keyLen: 16, saltLen: 8}, "a long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := verifyPassword(h, "a long passphrase"); !ok || err != nil {
		t.Errorf("verify with other parameters = %v, %v", ok, err)
	}
}

func TestVerifyPasswordRejectsMalformedHashes(t *testing.T) {
	for _, h := range []string{"", "plain", "$2a$10$bcrypt", "$argon2id$v=18$m=1,t=1,p=1$c2FsdA$a2V5", "$argon2id$v=19$m=x$c2FsdA$a2V5"} {
		if _, err := verifyPassword(h, "whatever"); err == nil {
			t.Errorf("verify(%q): no error", h)
		}
	}
}

func TestPasswordProblem(t *testing.T) {
	cases := map[string]string{
		"short":                  "too_short",
		"elevenchars":            "too_short",
		"twelve chars":           "",
		"ééééééééééé":            "too_short", // 11 runes, 22 bytes
		strings.Repeat("x", 256): "",
		strings.Repeat("x", 257): "too_long",
	}
	for pw, want := range cases {
		if got := passwordProblem(pw); got != want {
			t.Errorf("passwordProblem(%d runes) = %q, want %q", len([]rune(pw)), got, want)
		}
	}
}

func TestCSRFTokenIsBoundToTheSession(t *testing.T) {
	_, a := newToken()
	_, b := newToken()
	tok := csrfToken("secret-secret-secret-secret-secret", a)
	if !validCSRF("secret-secret-secret-secret-secret", a, tok) {
		t.Error("own token rejected")
	}
	if validCSRF("secret-secret-secret-secret-secret", b, tok) {
		t.Error("token valid for another session")
	}
	if validCSRF("another-secret-another-secret-xx", a, tok) {
		t.Error("token valid under another secret")
	}
	if validCSRF("secret-secret-secret-secret-secret", a, "") {
		t.Error("empty token accepted")
	}
}
