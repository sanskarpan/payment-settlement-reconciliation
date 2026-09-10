package money

import "testing"

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
	for _, input := range []string{"1,2", "1.001", "NaN", "AUD 1", "1e2"} {
		if _, err := ParseCents(input); err == nil {
			t.Fatalf("expected %q to fail", input)
		}
	}
}
