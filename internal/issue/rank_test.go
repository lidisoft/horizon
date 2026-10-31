package issue

import (
	"math/rand/v2"
	"sort"
	"testing"
)

func TestBetween(t *testing.T) {
	cases := [][3]string{
		{"", "", "V"},
		{"V", "", "W"},
		{"", "V", "U"},
		{"z", "", "zV"},
		{"", "1", "0V"},
		{"a", "b", "aV"},
		{"a", "a1", "a0V"},
		{"zz", "", "zzV"},
		{"F", "k", "U"},
	}
	for _, c := range cases {
		got, err := Between(c[0], c[1])
		if err != nil || got != c[2] {
			t.Errorf("Between(%q, %q) = %q, %v; want %q", c[0], c[1], got, err, c[2])
		}
	}
	if _, err := Between("b", "a"); err == nil {
		t.Error("reversed bounds accepted")
	}
	if _, err := Between("a0", ""); err == nil {
		t.Error("trailing zero accepted")
	}
}

// Random inserts keep keys strictly ordered, valid and reasonably short.
func TestBetweenProperty(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	keys := []string{}
	for range 2000 {
		i := r.IntN(len(keys) + 1)
		lo, hi := "", ""
		if i > 0 {
			lo = keys[i-1]
		}
		if i < len(keys) {
			hi = keys[i]
		}
		k, err := Between(lo, hi)
		if err != nil {
			t.Fatal(err)
		}
		if k <= lo || (hi != "" && k >= hi) || !validRank(k) {
			t.Fatalf("Between(%q, %q) = %q", lo, hi, k)
		}
		keys = append(keys[:i], append([]string{k}, keys[i:]...)...)
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatal("keys out of order")
	}
	// Appending at the end, the common case, grows keys slowly.
	k := ""
	for range 1000 {
		k, _ = Between(k, "")
	}
	if len(k) > 40 {
		t.Errorf("1000 appends give a %d-character key", len(k))
	}
}
