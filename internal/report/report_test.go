package report

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
	"reconciliation/internal/domain"
)

func TestWriteAndVerifyStrictReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.xlsx")
	row := domain.RawRow{Source: domain.SourcePayment, Kind: domain.RowTransaction, Ordinal: 1, LineStart: 2, Scope: domain.ScopeIn, HasRecon: true}
	mapped := domain.MappedRow{Row: row, KeyParts: []string{"key"}, Key: "key", Contributions: []domain.Contribution{{Field: "product_sales", KeyParts: []string{"key"}}}}
	settlement := mapped
	settlement.Row.Source = domain.SourceSettlement
	result := domain.RunResult{PaymentRows: []domain.MappedRow{mapped}, SettlementRows: []domain.MappedRow{settlement}, Groups: []*domain.Group{{KeyParts: []string{"key"}, Key: "key", Scope: domain.ScopeIn, PaymentRows: []domain.RawRow{row}, SettlementRows: []domain.RawRow{settlement.Row}, PaymentBuckets: map[string]int64{}, SettleBuckets: map[string]int64{}}}, Summary: domain.Summary{Buckets: map[string]map[domain.Source]int64{}}}
	if err := Write(path, result, map[string]string{"mode": "strict", "mapping_issues": "0"}); err != nil {
		t.Fatal(err)
	}
	if err := Verify(path); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.SetCellValue("Summary", "E6", 0.01); err != nil {
		t.Fatal(err)
	}
	if err = f.Save(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := Verify(path); err == nil {
		t.Fatal("corrupt summary delta accepted")
	}
}

func TestSummaryOverflowFails(t *testing.T) {
	result := domain.RunResult{Summary: domain.Summary{Buckets: map[string]map[domain.Source]int64{"sales_product_charges": {domain.SourcePayment: math.MaxInt64}, "sales_tax": {domain.SourcePayment: 1}}}}
	if err := writeSummary(excelize.NewFile(), result, map[string]int{}); err == nil {
		t.Fatal("overflowing subtotal accepted")
	}
}
