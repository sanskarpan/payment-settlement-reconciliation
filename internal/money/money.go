package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var amountPattern = regexp.MustCompile(`^-?(?:0|[1-9]\d{0,2}(?:,\d{3})*|\d+)(?:\.\d{1,2})?$`)

const maxInt64 = int64(^uint64(0) >> 1)

func ParseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("monetary amount is empty")
	}
	if !amountPattern.MatchString(s) {
		return 0, fmt.Errorf("invalid monetary amount %q", s)
	}
	negative := strings.HasPrefix(s, "-")
	if negative {
		s = s[1:]
	}
	parts := strings.SplitN(s, ".", 2)
	wholeText := strings.ReplaceAll(parts[0], ",", "")
	whole, err := strconv.ParseUint(wholeText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	frac := uint64(0)
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		frac, err = strconv.ParseUint(f, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("amount %q: %w", s, err)
		}
	}
	limit := uint64(maxInt64)
	if negative {
		limit++
	}
	if frac > limit || whole > (limit-frac)/100 {
		return 0, fmt.Errorf("amount %q overflows cents", s)
	}
	magnitude := whole*100 + frac
	if negative {
		if magnitude == uint64(maxInt64)+1 {
			return -maxInt64 - 1, nil
		}
		return -int64(magnitude), nil
	}
	return int64(magnitude), nil
}

func Add(a, b int64) (int64, error) {
	if b > 0 && a > maxInt64-b {
		return 0, fmt.Errorf("money addition overflow")
	}
	if b < 0 && a < -maxInt64-1-b {
		return 0, fmt.Errorf("money addition underflow")
	}
	return a + b, nil
}

func Sub(a, b int64) (int64, error) {
	if b > 0 && a < -maxInt64-1+b {
		return 0, fmt.Errorf("money subtraction underflow")
	}
	if b < 0 && a > maxInt64+b {
		return 0, fmt.Errorf("money subtraction overflow")
	}
	return a - b, nil
}

func FormatCents(v int64) string {
	neg := v < 0
	var magnitude uint64
	if neg {
		// -(MinInt64) is not representable as int64. Convert without signed
		// overflow so diagnostics remain correct even for a defensive boundary.
		magnitude = uint64(-(v + 1)) + 1
	} else {
		magnitude = uint64(v)
	}
	whole, frac := magnitude/100, magnitude%100
	s := fmt.Sprintf("%d.%02d", whole, frac)
	if neg {
		return "-" + s
	}
	return s
}
