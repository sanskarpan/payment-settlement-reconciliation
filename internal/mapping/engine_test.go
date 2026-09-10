package mapping

import (
	"path/filepath"
	"testing"

	"reconciliation/internal/domain"
	"reconciliation/internal/ingest"
	"reconciliation/internal/reconcile"
)

func TestReferenceMappingBaselineHasKnownDiagnostics(t *testing.T) {
	root := filepath.Join("..", "..", "workingData")
	p, _, err := ingest.ParsePayments(filepath.Join(root, "amazon_payments_data.csv"))
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
