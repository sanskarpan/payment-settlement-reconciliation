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
	"unicode/utf8"

	"reconciliation/internal/domain"
	"reconciliation/internal/money"
	"reconciliation/internal/normalize"
)

type FileInfo struct {
	Path   string
	SHA256 string
	Bytes  int64
}

const (
	maxInputBytes  = int64(100 << 20)
	maxRecordBytes = int64(1 << 20)
	maxInputRows   = 5_000_000
)

func open(path string) (*os.File, FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, FileInfo{}, err
	}
	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, FileInfo{}, err
	}
	if stat.Size() > maxInputBytes {
		f.Close()
		return nil, FileInfo{}, fmt.Errorf("input %s is %d bytes, exceeds %d-byte limit", path, stat.Size(), maxInputBytes)
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
		before := r.InputOffset()
		fields, err := r.Read()
		if err != nil {
			return nil, 0, fmt.Errorf("read header near line %d: %w", line, err)
		}
		if err := validateRecord(fields, r.InputOffset()-before); err != nil {
			return nil, 0, fmt.Errorf("header candidate %d: %w", line, err)
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
			headerLine, _ := r.FieldPos(0)
			return fields, headerLine, nil
		}
	}
	return nil, 0, fmt.Errorf("required header not found")
}

func validateRecord(fields []string, bytes int64) error {
	if bytes > maxRecordBytes {
		return fmt.Errorf("logical record is %d bytes, exceeds %d-byte limit", bytes, maxRecordBytes)
	}
	for i, value := range fields {
		if !utf8.ValidString(value) {
			return fmt.Errorf("field %d contains invalid UTF-8", i+1)
		}
	}
	return nil
}

func lineRange(r *csv.Reader, fields []string) (int, int) {
	start, _ := r.FieldPos(0)
	end, _ := r.FieldPos(len(fields) - 1)
	end += strings.Count(fields[len(fields)-1], "\n")
	return start, end
}

func cleanReader(f *os.File) (*bufio.Reader, int64, error) {
	prefix := make([]byte, 3)
	n, err := io.ReadFull(f, prefix)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, 0, err
	}
	if n == 3 && string(prefix) == "\ufeff" {
		return bufio.NewReader(f), 3, nil
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, 0, err
	}
	return bufio.NewReader(f), 0, nil
}

func ParsePayments(path string) ([]domain.RawRow, FileInfo, error) {
	f, info, err := open(path)
	if err != nil {
		return nil, info, err
	}
	defer f.Close()
	reader, byteBase, err := cleanReader(f)
	if err != nil {
		return nil, info, err
	}
	r := csv.NewReader(reader)
	r.FieldsPerRecord = -1
	r.LazyQuotes = false
	required := map[string]bool{
		"date/time": true, "transaction_release_date": true, "settlement_id": true,
		"type": true, "description": true, "order_id": true, "sku": true,
		"total": true, "transaction_status": true,
		"product_sales": true, "shipping_credits": true, "gift_wrap_credits": true,
		"promotional_rebates": true, "sales_tax_collected": true, "low_value_goods": true,
		"selling_fees": true, "fulfilment_by_amazon_fees": true,
		"other_transaction_fees": true, "other": true,
	}
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
		if ordinal > maxInputRows {
			return nil, info, fmt.Errorf("payment input exceeds %d-row limit", maxInputRows)
		}
		rawByteStart := r.InputOffset()
		byteStart := byteBase + rawByteStart
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
		if err = validateRecord(fields, r.InputOffset()-rawByteStart); err != nil {
			return nil, info, fmt.Errorf("payment record %d: %w", ordinal, err)
		}
		line, lineEnd := lineRange(r, fields)
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
		if posted == nil {
			return nil, info, fmt.Errorf("payment line %d has no posted timestamp", line)
		}
		row := domain.RawRow{Source: domain.SourcePayment, Kind: domain.RowTransaction, Ordinal: ordinal, LineStart: line, LineEnd: lineEnd, ByteStart: byteStart, ByteEnd: byteBase + r.InputOffset(), Raw: raw, Canonical: cp, RawPayload: jsonPayload(raw), CanonicalPayload: jsonPayload(cp), SettlementID: cp["settlement_id"], Currency: "AUD", Transaction: cp["type"], Description: cp["description"], SKU: cp["sku"], TxnRef: cp["order_id"], PostedAt: posted, ReleaseAt: release, Status: cp["transaction_status"], ReconAmount: total, HasRecon: true, EventClass: "OPERATING"}
		switch {
		case strings.EqualFold(row.Status, "Released"):
			if release == nil {
				return nil, info, fmt.Errorf("payment line %d Released row has no transaction release date", line)
			}
			row.KeyDate = release.UTC().Format("2006-01-02")
		case strings.EqualFold(row.Status, "Deferred"):
			row.KeyDate = posted.UTC().Format("2006-01-02")
		default:
			return nil, info, fmt.Errorf("payment line %d has unsupported transaction status %q", line, row.Status)
		}
		if normalize.Label(row.Transaction) == "TRANSFER" {
			row.EventClass = "BANK_TRANSFER"
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
	reader, byteBase, err := cleanReader(f)
	if err != nil {
		return nil, info, err
	}
	r := csv.NewReader(reader)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = false
	headers, _, err := readHeader(r, map[string]bool{
		"settlement_id": true, "transaction_type": true, "amount": true,
		"amount_type": true, "amount_description": true, "posted_date": true,
		"posted_date_time": true, "currency": true, "total_amount": true,
		"order_id": true, "adjustment_id": true, "sku": true,
		"shipment_id": true, "merchant_order_id": true,
	}, 0)
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
		if ordinal > maxInputRows {
			return nil, info, fmt.Errorf("settlement input exceeds %d-row limit", maxInputRows)
		}
		rawByteStart := r.InputOffset()
		byteStart := byteBase + rawByteStart
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
		if err = validateRecord(fields, r.InputOffset()-rawByteStart); err != nil {
			return nil, info, fmt.Errorf("settlement record %d: %w", ordinal, err)
		}
		line, lineEnd := lineRange(r, fields)
		raw, cp := map[string]string{}, map[string]string{}
		for i, v := range fields {
			raw[headers[i]] = v
			cp[canon[i]] = strings.TrimSpace(v)
		}
		kind := domain.RowTransaction
		if cp["transaction_type"] == "" && cp["total_amount"] != "" {
			kind = domain.RowMetadata
		} else if cp["transaction_type"] == "" {
			return nil, info, fmt.Errorf("settlement line %d has neither transaction type nor metadata total", line)
		}
		amountText := cp["amount"]
		has := amountText != ""
		if kind == domain.RowTransaction && !has {
			return nil, info, fmt.Errorf("settlement line %d transaction has no amount", line)
		}
		if kind == domain.RowMetadata && has {
			return nil, info, fmt.Errorf("settlement line %d metadata unexpectedly has transaction amount", line)
		}
		amount := int64(0)
		if has {
			amount, err = money.ParseCents(amountText)
			if err != nil {
				return nil, info, fmt.Errorf("settlement line %d amount: %w", line, err)
			}
		}
		postedTime, err := parseSettlementDate(cp["posted_date_time"])
		if err != nil {
			return nil, info, fmt.Errorf("settlement line %d: %w", line, err)
		}
		postedDate, err := parseSettlementDate(cp["posted_date"])
		if err != nil {
			return nil, info, fmt.Errorf("settlement line %d: %w", line, err)
		}
		if postedTime != nil && postedDate != nil && postedTime.UTC().Format("2006-01-02") != postedDate.UTC().Format("2006-01-02") {
			return nil, info, fmt.Errorf("settlement line %d posted date %s disagrees with timestamp UTC day %s", line, postedDate.Format("2006-01-02"), postedTime.UTC().Format("2006-01-02"))
		}
		posted := postedTime
		if posted == nil {
			posted = postedDate
		}
		if kind == domain.RowTransaction && postedDate == nil {
			return nil, info, fmt.Errorf("settlement line %d transaction has no posted date", line)
		}
		txnRef := cp["order_id"]
		if txnRef == "" {
			txnRef = cp["adjustment_id"]
		}
		row := domain.RawRow{Source: domain.SourceSettlement, Kind: kind, Ordinal: ordinal, LineStart: line, LineEnd: lineEnd, ByteStart: byteStart, ByteEnd: byteBase + r.InputOffset(), Raw: raw, Canonical: cp, RawPayload: jsonPayload(raw), CanonicalPayload: jsonPayload(cp), SettlementID: cp["settlement_id"], Currency: cp["currency"], Transaction: cp["transaction_type"], Description: cp["amount_description"], AmountType: cp["amount_type"], AmountDesc: cp["amount_description"], SKU: cp["sku"], TxnRef: txnRef, PostedAt: posted, ReconAmount: amount, HasRecon: has, Scope: domain.ScopeMetadata, EventClass: "OPERATING"}
		if kind == domain.RowMetadata {
			row.EventClass = "CONTROL"
			if row.SettlementID == "" || row.Currency == "" {
				return nil, info, fmt.Errorf("settlement line %d metadata requires settlement ID and currency", line)
			}
			headerTotal, parseErr := money.ParseCents(cp["total_amount"])
			if parseErr != nil {
				return nil, info, fmt.Errorf("settlement line %d total amount: %w", line, parseErr)
			}
			row.ReconAmount = headerTotal
			row.HasRecon = true
		} else {
			if row.SettlementID == "" {
				return nil, info, fmt.Errorf("settlement line %d transaction has no settlement ID", line)
			}
			row.KeyDate = postedDate.UTC().Format("2006-01-02")
		}
		out = append(out, row)
	}
	metadata := make(map[string]domain.RawRow)
	for _, row := range out {
		if row.Kind != domain.RowMetadata {
			continue
		}
		if _, exists := metadata[row.SettlementID]; exists {
			return nil, info, fmt.Errorf("duplicate metadata rows for settlement %s", row.SettlementID)
		}
		metadata[row.SettlementID] = row
	}
	for i := range out {
		if out[i].Kind != domain.RowTransaction {
			continue
		}
		control, ok := metadata[out[i].SettlementID]
		if !ok {
			return nil, info, fmt.Errorf("settlement line %d has no metadata for settlement %s", out[i].LineStart, out[i].SettlementID)
		}
		if out[i].Currency != "" && out[i].Currency != control.Currency {
			return nil, info, fmt.Errorf("settlement line %d currency %s conflicts with metadata currency %s", out[i].LineStart, out[i].Currency, control.Currency)
		}
		out[i].Currency = control.Currency
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
