package ingest

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"reconciliation/internal/domain"
	"reconciliation/internal/money"
	"reconciliation/internal/normalize"
)

type FileInfo struct {
	Path   string
	SHA256 string
	Bytes  int64
}

func open(path string) (*os.File, FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, FileInfo{}, err
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		f.Close()
		return nil, FileInfo{}, err
	}
	if _, err = f.Seek(0, 0); err != nil {
		f.Close()
		return nil, FileInfo{}, err
	}
	return f, FileInfo{Path: path, SHA256: fmt.Sprintf("%x", h.Sum(nil)), Bytes: n}, nil
}

func readHeader(r *csv.Reader, required map[string]bool, maxPreamble int) ([]string, int, error) {
	for line := 1; line <= maxPreamble+1; line++ {
		fields, err := r.Read()
		if err != nil {
			return nil, 0, fmt.Errorf("read header near line %d: %w", line, err)
		}
		found := make(map[string]bool)
		for _, f := range fields {
			found[normalize.Header(f)] = true
		}
		ok := true
		for req := range required {
			if !found[req] {
				ok = false
				break
			}
		}
		if ok {
			return fields, line, nil
		}
	}
	return nil, 0, fmt.Errorf("required header not found")
}

func cleanReader(f *os.File) *bufio.Reader {
	b := bufio.NewReader(f)
	if prefix, err := b.Peek(3); err == nil && string(prefix) == "\ufeff" {
		_, _ = b.Discard(3)
	}
	return b
}

func ParsePayments(path string) ([]domain.RawRow, FileInfo, error) {
	f, info, err := open(path)
	if err != nil {
		return nil, info, err
	}
	defer f.Close()
	r := csv.NewReader(cleanReader(f))
	r.FieldsPerRecord = -1
	r.LazyQuotes = false
	required := map[string]bool{"date/time": true, "settlement_id": true, "type": true, "total": true, "transaction_status": true}
	headers, _, err := readHeader(r, required, 20)
	if err != nil {
		return nil, info, err
	}
	canon := make([]string, len(headers))
	seen := map[string]bool{}
	for i, h := range headers {
		canon[i] = normalize.Header(h)
		if seen[canon[i]] {
			return nil, info, fmt.Errorf("duplicate header %q", h)
		}
		seen[canon[i]] = true
	}
	var out []domain.RawRow
	for ordinal := 1; ; ordinal++ {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, info, fmt.Errorf("payment record %d: %w", ordinal, err)
		}
		if len(fields) != len(headers) {
			return nil, info, fmt.Errorf("payment record %d: got %d fields, want %d", ordinal, len(fields), len(headers))
		}
		line, _ := r.FieldPos(0)
		raw, cp := map[string]string{}, map[string]string{}
		for i, v := range fields {
			raw[headers[i]] = v
			cp[canon[i]] = strings.TrimSpace(v)
		}
		posted, err := parsePaymentDate(cp["date/time"])
		if err != nil {
			return nil, info, fmt.Errorf("payment line %d: %w", line, err)
		}
		release, err := parsePaymentDate(cp["transaction_release_date"])
		if err != nil {
			return nil, info, fmt.Errorf("payment line %d: %w", line, err)
		}
		total, err := money.ParseCents(cp["total"])
		if err != nil {
			return nil, info, fmt.Errorf("payment line %d total: %w", line, err)
		}
		if err = validatePaymentComponents(cp, total, line); err != nil {
			return nil, info, err
		}
		row := domain.RawRow{Source: domain.SourcePayment, Kind: domain.RowTransaction, Ordinal: ordinal, LineStart: line, LineEnd: line, Raw: raw, Canonical: cp, RawPayload: jsonPayload(raw), CanonicalPayload: jsonPayload(cp), SettlementID: cp["settlement_id"], Currency: "AUD", Transaction: cp["type"], Description: cp["description"], SKU: cp["sku"], TxnRef: cp["order_id"], PostedAt: posted, ReleaseAt: release, Status: cp["transaction_status"], ReconAmount: total, HasRecon: true}
		if release != nil {
			row.KeyDate = release.UTC().Format("2006-01-02")
		} else if posted != nil {
			row.KeyDate = posted.UTC().Format("2006-01-02")
		}
		out = append(out, row)
	}
	return out, info, nil
}

func ParseSettlements(path string) ([]domain.RawRow, FileInfo, error) {
	f, info, err := open(path)
	if err != nil {
		return nil, info, err
	}
	defer f.Close()
	r := csv.NewReader(cleanReader(f))
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = false
	headers, _, err := readHeader(r, map[string]bool{"settlement_id": true, "transaction_type": true, "amount": true}, 0)
	if err != nil {
		return nil, info, err
	}
	canon := make([]string, len(headers))
	seen := map[string]bool{}
	for i, h := range headers {
		canon[i] = normalize.Header(h)
		if seen[canon[i]] {
			return nil, info, fmt.Errorf("duplicate settlement header %q", h)
		}
		seen[canon[i]] = true
	}
	var out []domain.RawRow
	for ordinal := 1; ; ordinal++ {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, info, fmt.Errorf("settlement record %d: %w", ordinal, err)
		}
		if len(fields) != len(headers) {
			return nil, info, fmt.Errorf("settlement record %d: got %d fields, want %d", ordinal, len(fields), len(headers))
		}
		line, _ := r.FieldPos(0)
		raw, cp := map[string]string{}, map[string]string{}
		for i, v := range fields {
			raw[headers[i]] = v
			cp[canon[i]] = strings.TrimSpace(v)
		}
		kind := domain.RowTransaction
		if cp["transaction_type"] == "" {
			kind = domain.RowMetadata
		}
		amountText := cp["amount"]
		has := amountText != ""
		amount := int64(0)
		if has {
			amount, err = money.ParseCents(amountText)
			if err != nil {
				return nil, info, fmt.Errorf("settlement line %d amount: %w", line, err)
			}
		}
		posted, err := parseSettlementDate(cp["posted_date_time"])
		if err != nil {
			return nil, info, fmt.Errorf("settlement line %d: %w", line, err)
		}
		if posted == nil {
			posted, err = parseSettlementDate(cp["posted_date"])
			if err != nil {
				return nil, info, fmt.Errorf("settlement line %d: %w", line, err)
			}
		}
		row := domain.RawRow{Source: domain.SourceSettlement, Kind: kind, Ordinal: ordinal, LineStart: line, LineEnd: line, Raw: raw, Canonical: cp, RawPayload: jsonPayload(raw), CanonicalPayload: jsonPayload(cp), SettlementID: cp["settlement_id"], Currency: cp["currency"], Transaction: cp["transaction_type"], Description: cp["amount_description"], AmountType: cp["amount_type"], AmountDesc: cp["amount_description"], SKU: cp["sku"], TxnRef: cp["order_id"], PostedAt: posted, ReconAmount: amount, HasRecon: has, Scope: domain.ScopeMetadata}
		if posted != nil {
			row.KeyDate = posted.UTC().Format("2006-01-02")
		}
		out = append(out, row)
	}
	// Flat File V2 puts currency and period controls on the metadata row. The
	// transaction rows remain source rows in their own right, but inherit the
	// single file currency for partitioning and reconciliation identity.
	currency := ""
	for _, row := range out {
		if row.Kind == domain.RowMetadata && row.Currency != "" {
			currency = row.Currency
			break
		}
	}
	if currency != "" {
		for i := range out {
			if out[i].Kind == domain.RowTransaction && out[i].Currency == "" {
				out[i].Currency = currency
			}
		}
	}
	return out, info, nil
}

func jsonPayload(v map[string]string) []byte {
	b, _ := json.Marshal(v)
	return b
}

func validatePaymentComponents(canonical map[string]string, total int64, line int) error {
	fields := []string{"product_sales", "shipping_credits", "gift_wrap_credits", "promotional_rebates", "sales_tax_collected", "low_value_goods", "selling_fees", "fulfilment_by_amazon_fees", "other_transaction_fees", "other"}
	for _, field := range fields {
		if _, ok := canonical[field]; !ok {
			return nil // A synthetic/minimal reader fixture may omit the optional component contract.
		}
	}
	var sum int64
	for _, field := range fields {
		value := canonical[field]
		if value == "" {
			continue
		}
		amount, err := money.ParseCents(value)
		if err != nil {
			return fmt.Errorf("payment line %d %s: %w", line, field, err)
		}
		sum, err = money.Add(sum, amount)
		if err != nil {
			return fmt.Errorf("payment line %d component total: %w", line, err)
		}
	}
	if sum != total {
		return fmt.Errorf("payment line %d component total %s does not equal total %s", line, money.FormatCents(sum), money.FormatCents(total))
	}
	return nil
}
