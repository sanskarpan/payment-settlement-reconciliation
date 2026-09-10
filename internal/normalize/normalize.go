package normalize

import (
	"regexp"
	"strings"
)

var spaces = regexp.MustCompile(`\s+`)

func Label(s string) string {
	return strings.ToUpper(spaces.ReplaceAllString(strings.TrimSpace(s), "_"))
}

func Header(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(s, "\ufeff"))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return strings.ToLower(s)
}

func TransferDescription(s string) string {
	n := Label(s)
	if strings.HasPrefix(n, "TO_ACCOUNT_ENDING_WITH:") {
		return "TO_ACCOUNT_ENDING"
	}
	return n
}
