// Package secrets encrypts secrets at rest (webhook secrets, forge tokens,
// OIDC client secrets) with SECRET_KEY: AES-256-GCM, each value naming the
// key that sealed it, so keys can rotate (docs/operations.md#configuration).
//
// A sealed value reads "v1.<key id>.<base64url(nonce ‖ ciphertext)>"; the
// key id is the first 8 hex digits of the key's SHA-256. The caller's
// associated data (a table and row) binds a value to its place: a value
// copied to another row doesn't open.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrUnknownKey means no configured key sealed the value: SECRET_KEY
// changed and the old one isn't in SECRET_KEY_PREVIOUS.
var ErrUnknownKey = errors.New("secrets: sealed with an unknown key")

// Box seals with the current key and opens with any configured key.
type Box struct {
	current string // key id
	aeads   map[string]cipher.AEAD
}

// New returns a box for the current key and the previous ones (each 32
// bytes).
func New(current []byte, previous ...[]byte) (*Box, error) {
	b := &Box{aeads: map[string]cipher.AEAD{}}
	for i, k := range append([][]byte{current}, previous...) {
		block, err := aes.NewCipher(k)
		if err != nil {
			return nil, fmt.Errorf("secrets: key %d: %w", i, err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		id := KeyID(k)
		if i == 0 {
			b.current = id
		}
		if _, ok := b.aeads[id]; !ok {
			b.aeads[id] = aead
		}
	}
	return b, nil
}

// KeyID names a key without revealing it.
func KeyID(k []byte) string {
	sum := sha256.Sum256(k)
	return hex.EncodeToString(sum[:4])
}

// Seal encrypts plain, bound to ad.
func (b *Box) Seal(plain, ad string) (string, error) {
	aead := b.aeads[b.current]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := aead.Seal(nonce, nonce, []byte(plain), []byte(ad))
	return "v1." + b.current + "." + base64.RawURLEncoding.EncodeToString(out), nil
}

// Open decrypts a sealed value bound to ad.
func (b *Box) Open(sealed, ad string) (string, error) {
	version, rest, ok := strings.Cut(sealed, ".")
	id, data, ok2 := strings.Cut(rest, ".")
	if !ok || !ok2 || version != "v1" {
		return "", errors.New("secrets: not a sealed value")
	}
	aead, ok := b.aeads[id]
	if !ok {
		return "", ErrUnknownKey
	}
	raw, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil || len(raw) < aead.NonceSize() {
		return "", errors.New("secrets: malformed sealed value")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte(ad))
	if err != nil {
		return "", fmt.Errorf("secrets: %w", err)
	}
	return string(plain), nil
}

// Current reports whether the current key sealed the value (rekey skips
// those).
func (b *Box) Current(sealed string) bool {
	return strings.HasPrefix(sealed, "v1."+b.current+".")
}
