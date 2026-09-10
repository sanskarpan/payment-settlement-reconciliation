package mapping

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"reconciliation/internal/domain"
	"reconciliation/internal/normalize"
)

func LoadConfig(path string, source domain.Source) ([]domain.ConfigRule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return nil, err
	}
	for i := range head {
		head[i] = normalize.Header(head[i])
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[h] = i
	}
	required := []string{"transaction_type", "record_ref", "to_summary_field_when_positive_amount", "to_summary_field_when_negative_amount"}
	for _, h := range required {
		if _, ok := idx[h]; !ok {
			return nil, fmt.Errorf("config %s missing %s", path, h)
		}
	}
	get := func(row []string, k string) string {
		if i, ok := idx[k]; ok && i < len(row) {
			return strings.TrimSpace(row[i])
		}
		return ""
	}
	var out []domain.ConfigRule
	line := 1
	for {
		row, e := r.Read()
		line++
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("config line %d: %w", line, e)
		}
		if len(row) != len(head) {
			return nil, fmt.Errorf("config line %d width %d want %d", line, len(row), len(head))
		}
		c := domain.ConfigRule{Source: source, OriginFile: path, OriginLine: line, TransactionType: get(row, "transaction_type"), RecordRef: get(row, "record_ref"), PositiveTarget: get(row, "to_summary_field_when_positive_amount"), NegativeTarget: get(row, "to_summary_field_when_negative_amount")}
		if source == domain.SourcePayment {
			c.Description = get(row, "description")
			c.AmountField = get(row, "amount_field")
		} else {
			c.AmountType = get(row, "amount_type")
			c.AmountDescription = get(row, "amount_description")
		}
		out = append(out, c)
	}
	return out, nil
}
