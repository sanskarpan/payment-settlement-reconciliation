package ingest

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var gmtHour = regexp.MustCompile(`GMT([+-])(\d{1,2})$`)

func parsePaymentDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	s = gmtHour.ReplaceAllStringFunc(s, func(v string) string {
		m := gmtHour.FindStringSubmatch(v)
		return "GMT" + m[1] + fmt.Sprintf("%02s00", m[2])
	})
	for _, layout := range []string{"2 January 2006 3:04:05 pm MST-0700", "2 January 2006 3:04:05 PM MST-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	for _, layout := range []string{"2 Jan 2006 3:04:05 pm MST-0700", "2 Jan 2006 3:04:05 PM MST-0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid payment timestamp %q", s)
}

func parseSettlementDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{"02.01.2006 15:04:05 MST", "02.01.2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("invalid settlement timestamp/date %q", s)
}
