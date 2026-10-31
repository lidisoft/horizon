package issue

import (
	"errors"
	"strings"
)

// Ranks order issues manually (docs/data-model.md#work-items): fractional
// keys in base 62, compared bytewise (the rank column uses COLLATE "C").
// Placing an issue between two others only writes that issue: Between
// returns a key strictly between its neighbours'. Keys never end in the
// zero digit, so there is always room between two of them.

const rankDigits = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

var errRankOrder = errors.New("rank: lower bound is not below upper bound")

// Between returns a key k with a < k < b. Empty a means the start, empty b
// the end.
func Between(a, b string) (string, error) {
	if b != "" && a >= b {
		return "", errRankOrder
	}
	if !validRank(a) || !validRank(b) {
		return "", errors.New("rank: invalid key")
	}
	// Adding at either end is the common case (new issues go last): step
	// the last digit rather than halving the gap, so keys grow by about
	// one character per thirty additions instead of one per six.
	switch {
	case a != "" && b == "":
		return increment(a), nil
	case a == "" && b != "":
		return decrement(b), nil
	}
	return midpoint(a, b), nil
}

// increment returns a key just above k.
func increment(k string) string {
	last := strings.IndexByte(rankDigits, k[len(k)-1])
	if last < len(rankDigits)-1 {
		return k[:len(k)-1] + string(rankDigits[last+1])
	}
	return k + string(rankDigits[len(rankDigits)/2])
}

// decrement returns a key just below k (and above the empty start).
func decrement(k string) string {
	last := strings.IndexByte(rankDigits, k[len(k)-1])
	if last > 1 {
		return k[:len(k)-1] + string(rankDigits[last-1])
	}
	// k ends in "1": "…0" would end in zero, so go one level deeper.
	return k[:len(k)-1] + "0" + string(rankDigits[len(rankDigits)/2])
}

func validRank(k string) bool {
	if strings.HasSuffix(k, "0") {
		return false
	}
	for i := 0; i < len(k); i++ {
		if strings.IndexByte(rankDigits, k[i]) < 0 {
			return false
		}
	}
	return true
}

// midpoint follows the fractional-indexing algorithm: the keys are read as
// base-62 fractions 0.a and 0.b (b empty is 1).
func midpoint(a, b string) string {
	if b != "" {
		// Skip the common prefix, padding a with zero digits.
		n := 0
		for n < len(b) && digitAt(a, n) == b[n] {
			n++
		}
		if n > 0 {
			return b[:n] + midpoint(tail(a, n), b[n:])
		}
	}
	da := 0
	if a != "" {
		da = strings.IndexByte(rankDigits, a[0])
	}
	db := len(rankDigits)
	if b != "" {
		db = strings.IndexByte(rankDigits, b[0])
	}
	if db-da > 1 {
		return string(rankDigits[(da+db)/2])
	}
	// Consecutive digits: b's first digit alone fits if b goes on.
	if len(b) > 1 {
		return b[:1]
	}
	// Otherwise keep a's digit and find room after it.
	return string(rankDigits[da]) + midpoint(tail(a, 1), "")
}

func digitAt(s string, i int) byte {
	if i < len(s) {
		return s[i]
	}
	return rankDigits[0]
}

func tail(s string, n int) string {
	if n >= len(s) {
		return ""
	}
	return s[n:]
}
