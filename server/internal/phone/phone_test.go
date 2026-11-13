package phone_test

import (
	"testing"

	"louvadeira/internal/phone"
)

func TestCanonical(t *testing.T) {
	cases := map[string]string{
		"557591110000":  "5575991110000",
		"5575991110000": "5575991110000",
		"551133334444":  "551133334444",
		"14155550100":   "14155550100",
		"":              "",
	}
	for in, want := range cases {
		if got := phone.Canonical(in); got != want {
			t.Errorf("Canonical(%q) = %q, want %q", in, got, want)
		}
	}
}
