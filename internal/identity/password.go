package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Password length limits (docs/security.md). The maximum only bounds hashing
// work; it is far above any real passphrase.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 256
)

// argon2id parameters: the OWASP baseline (19 MiB, 2 passes, 1 lane). Stored
// hashes carry their parameters, so raising them later only affects new hashes.
type argonParams struct {
	memory  uint32 // KiB
	time    uint32
	threads uint8
	keyLen  uint32
	saltLen int
}

var defaultArgon = argonParams{memory: 19 * 1024, time: 2, threads: 1, keyLen: 32, saltLen: 16}

// passwordProblem returns a field error code, or "" when password is
// acceptable.
func passwordProblem(password string) string {
	n := utf8.RuneCountInString(password)
	switch {
	case n < MinPasswordLength:
		return "too_short"
	case n > MaxPasswordLength:
		return "too_long"
	}
	return ""
}

// hashPassword returns an argon2id PHC string:
// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>.
func hashPassword(password string) (string, error) {
	return hashWith(defaultArgon, password)
}

func hashWith(p argonParams, password string) (string, error) {
	salt := make([]byte, p.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, p.keyLen)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// verifyPassword reports whether password matches the PHC string encoded.
func verifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("password hash: not argon2id")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, fmt.Errorf("password hash: unsupported version %q", parts[2])
	}
	var p argonParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return false, fmt.Errorf("password hash: parameters %q: %w", parts[3], err)
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("password hash: salt: %w", err)
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("password hash: key: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is verified against when the email is unknown, so a failed
// sign-in takes as long whether or not the account exists.
var dummyHash = func() string {
	h, err := hashPassword("horizon-dummy-password")
	if err != nil {
		panic(err)
	}
	return h
}()
