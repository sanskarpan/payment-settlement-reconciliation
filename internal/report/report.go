package report

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
	"reconciliation/internal/domain"
	"reconciliation/internal/money"
)

// setValue keeps untrusted source strings as literal Excel strings. In
// particular, descriptions or identifiers beginning with =, +, -, or @ must
// never become formulas when opened by a finance user.
func setValue(f *excelize.File, sheet, cell string, value interface{}) {
	if s, ok := value.(string); ok {
		_ = f.SetCellStr(sheet, cell, s)
		return
	}
	_ = f.SetCellValue(sheet, cell, value)
}

// reportAmount keeps ordinary fixture amounts numeric for Excel sorting while
// avoiding a silent float64 rounding error if a future input exceeds the exact
// integer range of IEEE-754 cents. Such an extreme value is emitted as an
// auditable decimal string instead of being rounded.
func reportAmount(cents int64) any {
	const maxExactInteger = int64(1 << 53)
	if cents > maxExactInteger || cents < -maxExactInteger {
		return money.FormatCents(cents)
	}
	return float64(cents) / 100
}

func streamString(value string) any {
	return []excelize.RichTextRun{{Text: value}}
}

var summaryRows = []struct {
	row          int
	label, field string
}{{4, "Sales", ""}, {5, "Product Charges", "sales_product_charges"}, {6, "Tax", "sales_tax"}, {7, "Shipping", "sales_shipping"}, {8, "Amazon fees", "sales_amazon_fees"}, {9, "Inventory Reimbursements", "sales_inventory_reimbursements"}, {10, "Cross-account Debt Adjustment", ""}, {11, "Other", "sales_other"}, {12, "FBA Fees", ""}, {13, "Micro Deposit (Failed)", ""}, {15, "Refunds", ""}, {16, "Refund expenses", "refunded_expenses"}, {17, "Refunded sales", "refunded_sales"}, {19, "Expenses", ""}, {20, "Promo rebates", "expenses_promotional_rebates"}, {21, "FBA fees", "expenses_fba_fees"}, {22, "Cost of Advertising", "expenses_cost_of_advertising"}, {23, "Shipping Charges", ""}, {24, "Amazon fees", "expenses_amazon_fees"}, {25, "Reversed Reimbursements", "expenses_reversed_reimbursements"}, {26, "Cross-account Debt Adjustment", ""}, {27, "Other", "expenses_other"}, {28, "Micro Deposit", ""}, {32, "Paid To Amazon", "paid_to_amazon"}}

func Write(path string, result domain.RunResult, metadata map[string]string) error {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Summary"); err != nil {
		return err
	}
	for _, s := range []string{"Consolidated Data", "Source Rows", "Contributions", "Mapping Issues", "Run Info"} {
		if _, err := f.NewSheet(s); err != nil {
			return err
		}
	}
	styles, err := makeStyles(f)
	if err != nil {
		return err
	}
	if err = writeSummary(f, result, styles); err != nil {
		return err
	}
	if err = writeGroups(f, result.Groups, styles); err != nil {
		return err
	}
	if err = writeRows(f, result, styles); err != nil {
		return err
	}
	if err = writeContributions(f, result, styles); err != nil {
		return err
	}
	if err = writeIssues(f, result, styles); err != nil {
		return err
	}
	if err = writeInfo(f, metadata, styles); err != nil {
		return err
	}
	f.SetActiveSheet(0)
	destination := filepath.Clean(path)
	dir := filepath.Dir(destination)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".reconciliation-*.xlsx")
	if err != nil {
		return fmt.Errorf("create temporary workbook: %w", err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary workbook: %w", err)
	}
	defer os.Remove(tmpPath)
	if err = f.SaveAs(tmpPath); err != nil {
		return fmt.Errorf("save workbook: %w", err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("close workbook: %w", err)
	}
	if err = verifyStructure(tmpPath); err != nil {
		return fmt.Errorf("verify temporary workbook: %w", err)
	}
	if err = os.Rename(tmpPath, destination); err != nil {
		return fmt.Errorf("publish workbook: %w", err)
	}
	return nil
}

// Verify reopens a generated workbook and checks the structural and financial
// invariants that can be verified without relying on Excel recalculation.
func Verify(path string) error {
	if err := verifyStructure(path); err != nil {
		return err
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	want := []string{"Summary", "Consolidated Data", "Source Rows", "Contributions", "Mapping Issues", "Run Info"}
	got := f.GetSheetList()
	if len(got) != len(want) {
		return fmt.Errorf("sheet count %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			return fmt.Errorf("sheet %d=%q, want %q", i, got[i], name)
		}
	}
	for _, cell := range []string{"E5", "E7", "E11", "E16", "E20", "E21", "E24", "E25", "D35"} {
		v, err := f.GetCellValue("Summary", cell)
		if err != nil {
			return err
		}
		if v != "0" && v != "0.00" && v != "0.0" {
			return fmt.Errorf("Summary!%s=%q, want zero", cell, v)
		}
	}
	for _, name := range []string{"Consolidated Data", "Source Rows", "Contributions"} {
		rows, err := f.GetRows(name)
		if err != nil {
			return err
		}
		if len(rows) < 2 {
			return fmt.Errorf("sheet %q contains no data rows", name)
		}
	}
	return nil
}

func verifyStructure(path string) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("open workbook: %w", err)
	}
	defer f.Close()
	want := []string{"Summary", "Consolidated Data", "Source Rows", "Contributions", "Mapping Issues", "Run Info"}
	got := f.GetSheetList()
	if len(got) != len(want) {
		return fmt.Errorf("sheet count %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			return fmt.Errorf("sheet %d=%q, want %q", i, got[i], name)
		}
	}
	for _, name := range []string{"Consolidated Data", "Source Rows", "Contributions"} {
		rows, err := f.GetRows(name)
		if err != nil {
			return err
		}
		if len(rows) < 2 {
			return fmt.Errorf("sheet %q contains no data rows", name)
		}
	}
	return nil
}

func makeStyles(f *excelize.File) (map[string]int, error) {
	h, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"1F4E78"}, Pattern: 1}, Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"}})
	if err != nil {
		return nil, err
	}
	moneyStyle, err := f.NewStyle(&excelize.Style{NumFmt: 4})
	if err != nil {
		return nil, err
	}
	sub, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"D9EAF7"}, Pattern: 1}})
	if err != nil {
		return nil, err
	}
	return map[string]int{"header": h, "money": moneyStyle, "sub": sub}, nil
}

func writeSummary(f *excelize.File, r domain.RunResult, st map[string]int) error {
	sh := "Summary"
	setValue(f, sh, "B1", "Summary")
	setValue(f, sh, "C1", "Payments")
	setValue(f, sh, "D1", "Settlements")
	setValue(f, sh, "E1", "Payments - Settlements")
	f.SetCellStyle(sh, "B1", "E1", st["header"])
	vals := map[int]string{}
	for _, x := range summaryRows {
		vals[x.row] = x.field
		setValue(f, sh, fmt.Sprintf("B%d", x.row), x.label)
	}
	for _, x := range summaryRows {
		var p, s int64
		if x.field != "" {
			p = r.Summary.Buckets[x.field][domain.SourcePayment]
			s = r.Summary.Buckets[x.field][domain.SourceSettlement]
		} else {
			switch x.row {
			case 4:
				for _, f := range []string{"sales_product_charges", "sales_tax", "sales_shipping", "sales_amazon_fees", "sales_inventory_reimbursements", "sales_other"} {
					p += r.Summary.Buckets[f][domain.SourcePayment]
					s += r.Summary.Buckets[f][domain.SourceSettlement]
				}
			case 15:
				for _, f := range []string{"refunded_expenses", "refunded_sales"} {
					p += r.Summary.Buckets[f][domain.SourcePayment]
					s += r.Summary.Buckets[f][domain.SourceSettlement]
				}
			case 19:
				for _, f := range []string{"expenses_promotional_rebates", "expenses_fba_fees", "expenses_cost_of_advertising", "expenses_reversed_reimbursements", "expenses_amazon_fees", "expenses_other"} {
					p += r.Summary.Buckets[f][domain.SourcePayment]
					s += r.Summary.Buckets[f][domain.SourceSettlement]
				}
			}
		}
		setValue(f, sh, fmt.Sprintf("C%d", x.row), reportAmount(p))
		setValue(f, sh, fmt.Sprintf("D%d", x.row), reportAmount(s))
		setValue(f, sh, fmt.Sprintf("E%d", x.row), reportAmount(p-s))
		f.SetCellStyle(sh, fmt.Sprintf("C%d", x.row), fmt.Sprintf("E%d", x.row), st["money"])
		if x.field == "" {
			f.SetCellStyle(sh, fmt.Sprintf("B%d", x.row), fmt.Sprintf("E%d", x.row), st["sub"])
		}
	}
	setValue(f, sh, "B34", "Settlement header control")
	setValue(f, sh, "D34", reportAmount(r.SettlementHeader))
	f.SetCellStyle(sh, "D34", "D34", st["money"])
	setValue(f, sh, "B35", "Activity minus header control")
	setValue(f, sh, "D35", reportAmount(sumSummary(r.Summary, domain.SourceSettlement)-r.SettlementHeader))
	f.SetCellStyle(sh, "D35", "D35", st["money"])
	f.SetColWidth(sh, "B", "B", 32)
	f.SetColWidth(sh, "C", "E", 18)
	if err := f.SetSheetDimension(sh, "B1:E35"); err != nil {
		return err
	}
	return nil
}

func sumSummary(s domain.Summary, src domain.Source) int64 {
	var v int64
	for _, m := range s.Buckets {
		v += m[src]
	}
	return v
}

func writeGroups(f *excelize.File, groups []*domain.Group, st map[string]int) error {
	sh := "Consolidated Data"
	sw, err := f.NewStreamWriter(sh)
	if err != nil {
		return err
	}
	if err = sw.SetColWidth(1, 12, 18); err != nil {
		return err
	}
	headers := []string{"Status", "Scope", "Settlement ID", "Currency", "Record Ref", "Payment Rows", "Payment Amount", "Settlement Rows", "Settlement Amount", "Difference", "Payment Fields", "Settlement Fields"}
	headerCells := make([]interface{}, len(headers))
	for i, h := range headers {
		headerCells[i] = excelize.Cell{StyleID: st["header"], Value: streamString(h)}
	}
	if err = sw.SetRow("A1", headerCells); err != nil {
		return err
	}
	for i, g := range groups {
		row := i + 2
		status := "reconciled"
		if len(g.PaymentRows) == 0 {
			status = "unreconciled_settlement"
		} else if len(g.SettlementRows) == 0 {
			status = "unreconciled_payment"
		}
		values := []interface{}{
			excelize.Cell{Value: streamString(status)}, excelize.Cell{Value: streamString(string(g.Scope))},
			excelize.Cell{Value: streamString(g.SettlementID)}, excelize.Cell{Value: streamString(g.Currency)},
			excelize.Cell{Value: streamString(displayKey(g.KeyParts))}, excelize.Cell{Value: len(g.PaymentRows)},
			excelize.Cell{StyleID: st["money"], Value: reportAmount(g.PaymentAmount)},
			excelize.Cell{Value: len(g.SettlementRows)}, excelize.Cell{StyleID: st["money"], Value: reportAmount(g.SettlementAmt)},
			excelize.Cell{StyleID: st["money"], Value: reportAmount(g.PaymentAmount - g.SettlementAmt)},
			excelize.Cell{Value: streamString(bucketNames(g.PaymentBuckets))}, excelize.Cell{Value: streamString(bucketNames(g.SettleBuckets))},
		}
		if err = sw.SetRow(fmt.Sprintf("A%d", row), values); err != nil {
			return err
		}
	}
	if err = sw.Flush(); err != nil {
		return err
	}
	return nil
}

func bucketNames(m map[string]int64) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
func writeRows(f *excelize.File, r domain.RunResult, st map[string]int) error {
	sh := "Source Rows"
	sw, err := f.NewStreamWriter(sh)
	if err != nil {
		return err
	}
	if err = sw.SetColWidth(1, 14, 18); err != nil {
		return err
	}
	h := []string{"Source", "Kind", "Line", "Ordinal", "Settlement ID", "Scope", "Status", "Type", "Description", "SKU", "Txn Ref", "Key Date", "Amount", "Record Ref"}
	headerCells := make([]interface{}, len(h))
	for i, x := range h {
		headerCells[i] = excelize.Cell{StyleID: st["header"], Value: streamString(x)}
	}
	if err = sw.SetRow("A1", headerCells); err != nil {
		return err
	}
	row := 2
	for _, list := range [][]domain.MappedRow{r.PaymentRows, r.SettlementRows} {
		for _, m := range list {
			x := m.Row
			v := []interface{}{excelize.Cell{Value: streamString(string(x.Source))}, excelize.Cell{Value: streamString(string(x.Kind))}, excelize.Cell{Value: x.LineStart}, excelize.Cell{Value: x.Ordinal}, excelize.Cell{Value: streamString(x.SettlementID)}, excelize.Cell{Value: streamString(string(x.Scope))}, excelize.Cell{Value: streamString(x.Status)}, excelize.Cell{Value: streamString(x.Transaction)}, excelize.Cell{Value: streamString(x.Description)}, excelize.Cell{Value: streamString(x.SKU)}, excelize.Cell{Value: streamString(x.TxnRef)}, excelize.Cell{Value: streamString(x.KeyDate)}, excelize.Cell{StyleID: st["money"], Value: reportAmount(x.ReconAmount)}, excelize.Cell{Value: streamString(m.Key)}}
			if err = sw.SetRow(fmt.Sprintf("A%d", row), v); err != nil {
				return err
			}
			row++
		}
	}
	if err = sw.Flush(); err != nil {
		return err
	}
	return nil
}
func writeContributions(f *excelize.File, r domain.RunResult, st map[string]int) error {
	sh := "Contributions"
	sw, err := f.NewStreamWriter(sh)
	if err != nil {
		return err
	}
	if err = sw.SetColWidth(1, 8, 20); err != nil {
		return err
	}
	h := []string{"Source", "Line", "Rule Line", "Field", "Amount", "Target", "Decision", "Record Ref"}
	headerCells := make([]interface{}, len(h))
	for i, x := range h {
		headerCells[i] = excelize.Cell{StyleID: st["header"], Value: streamString(x)}
	}
	if err = sw.SetRow("A1", headerCells); err != nil {
		return err
	}
	row := 2
	for _, list := range [][]domain.MappedRow{r.PaymentRows, r.SettlementRows} {
		for _, m := range list {
			for _, x := range m.Contributions {
				v := []interface{}{excelize.Cell{Value: streamString(string(m.Row.Source))}, excelize.Cell{Value: m.Row.LineStart}, excelize.Cell{Value: x.Rule.OriginLine}, excelize.Cell{Value: streamString(x.Field)}, excelize.Cell{StyleID: st["money"], Value: reportAmount(x.Amount)}, excelize.Cell{Value: streamString(x.Target)}, excelize.Cell{Value: streamString(x.Decision)}, excelize.Cell{Value: streamString(displayKey(x.KeyParts))}}
				if err = sw.SetRow(fmt.Sprintf("A%d", row), v); err != nil {
					return err
				}
				row++
			}
		}
	}
	if err = sw.Flush(); err != nil {
		return err
	}
	return nil
}
func writeIssues(f *excelize.File, r domain.RunResult, st map[string]int) error {
	sh := "Mapping Issues"
	setValue(f, sh, "A1", "Issue")
	f.SetCellStyle(sh, "A1", "A1", st["header"])
	for i, x := range r.Issues {
		setValue(f, sh, fmt.Sprintf("A%d", i+2), x)
	}
	if len(r.Issues) == 0 {
		setValue(f, sh, "A2", "No mapping issues")
	}
	if err := f.SetSheetDimension(sh, fmt.Sprintf("A1:A%d", len(r.Issues)+2)); err != nil {
		return err
	}
	return nil
}
func writeInfo(f *excelize.File, m map[string]string, st map[string]int) error {
	sh := "Run Info"
	setValue(f, sh, "A1", "Property")
	setValue(f, sh, "B1", "Value")
	f.SetCellStyle(sh, "A1", "B1", st["header"])
	row := 2
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		setValue(f, sh, fmt.Sprintf("A%d", row), k)
		setValue(f, sh, fmt.Sprintf("B%d", row), v)
		row++
	}
	if err := f.SetSheetDimension(sh, fmt.Sprintf("A1:B%d", row-1)); err != nil {
		return err
	}
	return nil
}

func displayKey(parts []string) string { return strings.Join(parts, " + ") }
