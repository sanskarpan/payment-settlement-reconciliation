package ingest

import (
	"path/filepath"
	"testing"
)

func TestPaymentComponentConservation(t *testing.T) {
	fields := []string{"product_sales", "shipping_credits", "gift_wrap_credits", "promotional_rebates", "sales_tax_collected", "low_value_goods", "selling_fees", "fulfilment_by_amazon_fees", "other_transaction_fees", "other"}
	row := make(map[string]string, len(fields))
	for _, field := range fields {
		row[field] = "1.00"
	}
	if err := validatePaymentComponents(row, 10_00, 7); err != nil {
		t.Fatalf("valid components rejected: %v", err)
	}
	row["other"] = "0.99"
	if err := validatePaymentComponents(row, 10_00, 7); err == nil {
		t.Fatal("component mismatch accepted")
	}
}

func TestReferenceParsers(t *testing.T) {
	root := filepath.Join("..", "..", "workingData")
	p, _, err := ParsePayments(filepath.Join(root, "amazon_payments_data.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 23026 {
		t.Fatalf("payment rows=%d", len(p))
	}
	if p[0].KeyDate != "2026-07-17" {
		t.Fatalf("first payment key date=%s", p[0].KeyDate)
	}
	s, _, err := ParseSettlements(filepath.Join(root, "amazon_settlements_data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 54980 || s[0].Kind != "settlement_metadata" {
		t.Fatalf("settlement rows=%d first kind=%s", len(s), s[0].Kind)
	}
	if s[1].Currency != "AUD" {
		t.Fatalf("inherited settlement currency=%q", s[1].Currency)
	}
}
