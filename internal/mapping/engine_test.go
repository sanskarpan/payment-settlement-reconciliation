package mapping

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"reconciliation/internal/domain"
	"reconciliation/internal/ingest"
	"reconciliation/internal/reconcile"
)

func TestReferenceMappingBaselineHasKnownDiagnostics(t *testing.T) {
	root := filepath.Join("..", "..", "workingData")
	paymentPath := filepath.Join(root, "amazon_payments_data.csv")
	if _, err := os.Stat(paymentPath); errors.Is(err, os.ErrNotExist) {
		t.Skip("reference data package is not installed")
	} else if err != nil {
		t.Fatal(err)
	}
	p, _, err := ingest.ParsePayments(paymentPath)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := ingest.ParseSettlements(filepath.Join(root, "amazon_settlements_data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	// The full reference-data run is exercised by make before/after. Keep this
	// unit test bounded so race testing remains practical while still using the
	// actual source shape and the duplicate selectors.
	p = reconcile.Classify(p[:1], "12395580393")
	s = reconcile.Classify(s[1:2], "12395580393")
	pr, err := LoadConfig(filepath.Join(root, "amazon_payment_configs_au_old.csv"), domain.SourcePayment)
	if err != nil {
		t.Fatal(err)
	}
	sr, err := LoadConfig(filepath.Join(root, "amazon_settlement_configs_au.csv"), domain.SourceSettlement)
	if err != nil {
		t.Fatal(err)
	}
	_, _, issues, err := (Engine{Rules: pr, Mode: Diagnostic}).Map(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("diagnostic issues=%v", issues)
	}
	_, _, issues, err = (Engine{Rules: sr, Mode: Diagnostic}).Map(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("settlement issues=%v", issues)
	}
}

func TestEngineRejectsInvalidModeAndMissingAmounts(t *testing.T) {
	row := domain.RawRow{Source: domain.SourcePayment, Kind: domain.RowTransaction, LineStart: 2, Transaction: "ORDER", Description: "any", Canonical: map[string]string{}}
	rule := domain.ConfigRule{Source: domain.SourcePayment, TransactionType: "ORDER", Description: "any", AmountField: "product_sales", RecordRef: "LITERAL"}
	if _, _, _, err := (Engine{Rules: []domain.ConfigRule{rule}, Mode: "typo"}).Map([]domain.RawRow{row}); err == nil {
		t.Fatal("invalid mode accepted")
	}
	if _, _, _, err := (Engine{Rules: []domain.ConfigRule{rule}, Mode: Strict}).Map([]domain.RawRow{row}); err == nil {
		t.Fatal("missing mapped amount accepted")
	}
	row.Canonical["product_sales"] = "92233720368547758.07"
	if got, err := amountFor(row, "product_sales"); err != nil || got != math.MaxInt64 {
		t.Fatalf("max cents got=%d err=%v", got, err)
	}
}

func TestRecordRefExpansionRejectsUnknownTokenAndDelimiter(t *testing.T) {
	row := domain.RawRow{LineStart: 3, TxnRef: "id", Canonical: map[string]string{}}
	if _, err := expand("unknown_field", row); err == nil {
		t.Fatal("unknown field-like token accepted")
	}
	row.TxnRef = "bad\x1fvalue"
	if _, err := expand("txn_ref", row); err == nil {
		t.Fatal("reserved delimiter accepted")
	}
}
