package identity

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// tokenBytes is the entropy of session and reset tokens.
const tokenBytes = 32

// newToken returns a random URL-safe token and the SHA-256 hash stored in
// its place.
func newToken() (string, []byte) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// csrfToken derives the double-submit token of a session: an HMAC of the
// session's token hash under SESSION_SECRET. A cookie planted by a sibling
// origin can't match it without the session.
func csrfToken(secret string, sessionTokenHash []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte("horizon csrf\x00"))
	m.Write(sessionTokenHash)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func validCSRF(secret string, sessionTokenHash []byte, got string) bool {
	return got != "" && hmac.Equal([]byte(csrfToken(secret, sessionTokenHash)), []byte(got))
}
