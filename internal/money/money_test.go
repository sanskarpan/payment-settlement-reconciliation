package money

import (
	"math"
	"testing"
)

func TestParseCents(t *testing.T) {
	cases := map[string]int64{"0": 0, "-0.41": -41, "1,234.50": 123450, "1.2": 120, "-2": -200}
	for input, want := range cases {
		got, err := ParseCents(input)
		if err != nil || got != want {
			t.Fatalf("%q: got %d/%v, want %d", input, got, err, want)
		}
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	for _, input := range []string{"", "  ", "1,2", "1.001", "NaN", "AUD 1", "1e2", "92233720368547758.08"} {
		if _, err := ParseCents(input); err == nil {
			t.Fatalf("expected %q to fail", input)
		}
	}
}

func TestCheckedArithmeticBoundaries(t *testing.T) {
	if got, err := ParseCents("-92233720368547758.08"); err != nil || got != math.MinInt64 {
		t.Fatalf("parse min got=%d err=%v", got, err)
	}
	if _, err := ParseCents("-92233720368547758.09"); err == nil {
		t.Fatal("accepted below minimum cents")
	}
	if _, err := Add(math.MaxInt64, 1); err == nil {
		t.Fatal("addition overflow accepted")
	}
	if _, err := Add(math.MinInt64, -1); err == nil {
		t.Fatal("addition underflow accepted")
	}
	if _, err := Sub(math.MinInt64, 1); err == nil {
		t.Fatal("subtraction underflow accepted")
	}
	if _, err := Sub(math.MaxInt64, -1); err == nil {
		t.Fatal("subtraction overflow accepted")
	}
	if got := FormatCents(math.MinInt64); got != "-92233720368547758.08" {
		t.Fatalf("FormatCents(MinInt64)=%q", got)
	}
}
