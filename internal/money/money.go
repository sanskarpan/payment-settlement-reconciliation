package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var amountPattern = regexp.MustCompile(`^-?(?:0|[1-9]\d{0,2}(?:,\d{3})*|\d+)(?:\.\d{1,2})?$`)

func ParseCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
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
	whole, err := strconv.ParseInt(wholeText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	frac := int64(0)
	if len(parts) == 2 {
		f := parts[1]
		if len(f) == 1 {
			f += "0"
		}
		frac, err = strconv.ParseInt(f, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("amount %q: %w", s, err)
		}
	}
	if whole > (int64(^uint64(0)>>1)-frac)/100 {
		return 0, fmt.Errorf("amount %q overflows cents", s)
	}
	v := whole*100 + frac
	if negative {
		v = -v
	}
	return v, nil
}

func Add(a, b int64) (int64, error) {
	if b > 0 && a > int64(^uint64(0)>>1)-b {
		return 0, fmt.Errorf("money addition overflow")
	}
	if b < 0 && a < -int64(^uint64(0)>>1)-1-b {
		return 0, fmt.Errorf("money addition underflow")
	}
	return a + b, nil
}

func FormatCents(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole, frac := v/100, v%100
	s := fmt.Sprintf("%d.%02d", whole, frac)
	if neg {
		return "-" + s
	}
	return s
}
