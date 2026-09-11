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
	if source != domain.SourcePayment && source != domain.SourceSettlement {
		return nil, fmt.Errorf("unsupported config source %q", source)
	}
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
		if h == "" {
			return nil, fmt.Errorf("config %s has blank header at column %d", path, i+1)
		}
		if _, exists := idx[h]; exists {
			return nil, fmt.Errorf("config %s has duplicate normalized header %q", path, h)
		}
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
	for {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("config record: %w", e)
		}
		line, _ := r.FieldPos(0)
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
		if strings.TrimSpace(c.RecordRef) == "" && (c.PositiveTarget != "" || c.NegativeTarget != "") {
			return nil, fmt.Errorf("config line %d routes an amount but has an empty record_ref", line)
		}
		for _, target := range []string{c.PositiveTarget, c.NegativeTarget} {
			if target != "" && !KnownSummaryField(target) {
				return nil, fmt.Errorf("config line %d has unknown summary field %q", line, target)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

var knownSummaryFields = map[string]struct{}{
	"sales_product_charges": {}, "sales_tax": {}, "sales_shipping": {}, "sales_amazon_fees": {},
	"sales_inventory_reimbursements": {}, "sales_other": {}, "refunded_expenses": {}, "refunded_sales": {},
	"expenses_promotional_rebates": {}, "expenses_fba_fees": {}, "expenses_cost_of_advertising": {},
	"expenses_amazon_fees": {}, "expenses_reversed_reimbursements": {}, "expenses_other": {}, "paid_to_amazon": {},
	"bank_account_transfer_round_off": {}, "amazon_carried_forward": {}, "beginning_balance": {},
	"current_reserve_amount": {}, "total_adjustment_other_buyer_recharge_amt": {},
	"total_refund_expense_or_sales_amt": {},
}

func KnownSummaryField(field string) bool { _, ok := knownSummaryFields[field]; return ok }
