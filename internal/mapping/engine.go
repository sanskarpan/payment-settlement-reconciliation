package mapping

import (
	"fmt"
	"sort"
	"strings"

	"reconciliation/internal/domain"
	"reconciliation/internal/money"
	"reconciliation/internal/normalize"
)

type Mode string

const (
	Diagnostic Mode = "diagnostic-baseline"
	Strict     Mode = "strict"
)

type Engine struct {
	Rules []domain.ConfigRule
	Mode  Mode
	index map[string][]domain.ConfigRule
}

func (e Engine) Map(rows []domain.RawRow) ([]domain.MappedRow, domain.Summary, []string, error) {
	if e.Mode != Diagnostic && e.Mode != Strict {
		return nil, domain.Summary{}, nil, fmt.Errorf("unsupported mapping mode %q", e.Mode)
	}
	for _, rule := range e.Rules {
		if rule.Source != domain.SourcePayment && rule.Source != domain.SourceSettlement {
			return nil, domain.Summary{}, nil, fmt.Errorf("rule line %d has unsupported source %q", rule.OriginLine, rule.Source)
		}
		if rule.Source == domain.SourcePayment && strings.TrimSpace(rule.AmountField) == "" {
			return nil, domain.Summary{}, nil, fmt.Errorf("payment rule line %d has no amount field", rule.OriginLine)
		}
		if rule.Source == domain.SourceSettlement && strings.TrimSpace(rule.AmountType) == "" {
			return nil, domain.Summary{}, nil, fmt.Errorf("settlement rule line %d has no amount type", rule.OriginLine)
		}
		for _, target := range []string{rule.PositiveTarget, rule.NegativeTarget} {
			if target != "" && !KnownSummaryField(target) {
				return nil, domain.Summary{}, nil, fmt.Errorf("rule line %d has unknown summary field %q", rule.OriginLine, target)
			}
		}
	}
	if e.index == nil {
		e.index = make(map[string][]domain.ConfigRule)
		for _, rule := range e.Rules {
			e.index[normalize.Label(rule.TransactionType)] = append(e.index[normalize.Label(rule.TransactionType)], rule)
		}
	}
	var out []domain.MappedRow
	summary := domain.Summary{Buckets: map[string]map[domain.Source]int64{}}
	var issues []string
	issueSet := map[string]struct{}{}
	for _, row := range rows {
		mapped, err := e.mapRow(row)
		if err != nil {
			if e.Mode == Diagnostic {
				mapped.Issues = append(mapped.Issues, err.Error())
				issues = append(issues, fmt.Sprintf("%s line %d: %s", row.Source, row.LineStart, err))
				out = append(out, mapped)
				continue
			}
			return nil, summary, issues, err
		}
		for _, issue := range mapped.Issues {
			if _, ok := issueSet[issue]; !ok {
				issues = append(issues, issue)
				issueSet[issue] = struct{}{}
			}
		}
		if row.Scope == domain.ScopeIn {
			for _, c := range mapped.Contributions {
				if c.Amount == 0 || c.Target == "" {
					continue
				}
				if summary.Buckets[c.Target] == nil {
					summary.Buckets[c.Target] = map[domain.Source]int64{}
				}
				v, err := money.Add(summary.Buckets[c.Target][row.Source], c.Amount)
				if err != nil {
					return nil, summary, issues, err
				}
				summary.Buckets[c.Target][row.Source] = v
			}
		}
		out = append(out, mapped)
	}
	return out, summary, issues, nil
}

func (e Engine) mapRow(row domain.RawRow) (domain.MappedRow, error) {
	m := domain.MappedRow{Row: row}
	if row.Kind == domain.RowMetadata {
		return m, nil
	}
	var selected []domain.ConfigRule
	candidates := append([]domain.ConfigRule{}, e.index[normalize.Label(row.Transaction)]...)
	candidates = append(candidates, e.index[""]...)
	for _, rule := range candidates {
		if rule.Source != row.Source {
			continue
		}
		tx := normalize.Label(row.Transaction)
		rtx := normalize.Label(rule.TransactionType)
		if rtx != "" && rtx != tx {
			continue
		}
		if row.Source == domain.SourcePayment {
			if rule.AmountField == "" {
				continue
			}
			if !descriptionMatch(row, rule.Description) {
				continue
			}
			selected = append(selected, rule)
		} else {
			if normalize.Label(rule.AmountType) != normalize.Label(row.AmountType) || !descriptionMatch(row, rule.AmountDescription) {
				continue
			}
			selected = append(selected, rule)
		}
	}
	// Transaction-specific rules suppress catch-all rules for the same row.
	hasSpecific := false
	for _, r := range selected {
		if strings.TrimSpace(r.TransactionType) != "" {
			hasSpecific = true
			break
		}
	}
	if hasSpecific {
		var x []domain.ConfigRule
		for _, r := range selected {
			if strings.TrimSpace(r.TransactionType) != "" {
				x = append(x, r)
			}
		}
		selected = x
	}
	// For a payment component, prefer exact description to wildcard. Settlement has
	// one amount component, so the same rule applies directly.
	byField := map[string][]domain.ConfigRule{}
	for _, r := range selected {
		field := r.AmountField
		if row.Source == domain.SourceSettlement {
			field = "amount"
		}
		byField[field] = append(byField[field], r)
	}
	fields := make([]string, 0, len(byField))
	for f := range byField {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	for _, field := range fields {
		rules := byField[field]
		exact := false
		for _, r := range rules {
			if row.Source == domain.SourcePayment && normalize.Label(r.Description) != "ANY" {
				exact = true
			}
			if row.Source == domain.SourceSettlement && normalize.Label(r.AmountDescription) != "ANY" {
				exact = true
			}
		}
		if exact {
			var x []domain.ConfigRule
			for _, r := range rules {
				if row.Source == domain.SourcePayment && normalize.Label(r.Description) != "ANY" {
					x = append(x, r)
				}
				if row.Source == domain.SourceSettlement && normalize.Label(r.AmountDescription) != "ANY" {
					x = append(x, r)
				}
			}
			rules = x
		}
		if len(rules) > 1 {
			if e.Mode == Strict {
				return m, fmt.Errorf("ambiguous rules for %s line %d field %s: %v", row.Source, row.LineStart, field, ruleLines(rules))
			}
			m.Issues = append(m.Issues, fmt.Sprintf("DUPLICATE_SELECTOR field=%s rules=%v", field, ruleLines(rules)))
		}
		for _, r := range rules {
			amount, err := amountFor(row, field)
			if err != nil {
				return m, err
			}
			parts, err := expand(r.RecordRef, row)
			if err != nil {
				return m, err
			}
			key := strings.Join(parts, "\x1f")
			target := r.PositiveTarget
			if amount < 0 {
				target = r.NegativeTarget
			}
			decision := "ROUTED"
			if field == "marketplace_withheld_tax" && row.Canonical[field] == "" {
				if r.PositiveTarget != "" || r.NegativeTarget != "" {
					return m, fmt.Errorf("optional absent field %q cannot route to a summary target", field)
				}
				decision = "OPTIONAL_ABSENT"
			} else if amount == 0 {
				decision = "ZERO"
			}
			if target == "" && decision != "OPTIONAL_ABSENT" {
				decision = "NOT_SUMMARIZED"
			}
			m.Contributions = append(m.Contributions, domain.Contribution{RowOrdinal: row.Ordinal, Rule: r, Field: field, Amount: amount, Target: target, Decision: decision, KeyParts: parts, Key: key})
			if m.Key == "" && key != "" {
				m.Key = key
				m.KeyParts = parts
			} else if key != "" && m.Key != key {
				return m, fmt.Errorf("row line %d expands to conflicting keys %q and %q", row.LineStart, m.Key, key)
			}
		}
	}
	if row.Source == domain.SourcePayment {
		totalRouted, componentRouted := false, false
		for _, c := range m.Contributions {
			if c.Target == "" || c.Amount == 0 {
				continue
			}
			if c.Field == "total" {
				totalRouted = true
			} else {
				componentRouted = true
			}
		}
		if totalRouted && componentRouted {
			return m, fmt.Errorf("payment line %d routes both total and component amounts", row.LineStart)
		}
	}
	if m.Key == "" {
		return m, fmt.Errorf("row line %d has no record_ref mapping", row.LineStart)
	}
	return m, nil
}

func descriptionMatch(row domain.RawRow, selector string) bool {
	s := normalize.Label(selector)
	if s == "ANY" {
		return true
	}
	actual := normalize.Label(row.Description)
	if row.Source == domain.SourcePayment && normalize.Label(row.Transaction) == "TRANSFER" {
		actual = normalize.TransferDescription(row.Description)
	}
	return s == actual
}

func amountFor(row domain.RawRow, field string) (int64, error) {
	if row.Source == domain.SourceSettlement {
		if !row.HasRecon {
			return 0, fmt.Errorf("settlement line %d has no reconciliation amount", row.LineStart)
		}
		return row.ReconAmount, nil
	}
	v := row.Canonical[field]
	if field == "fba_fees" {
		v = row.Canonical["fulfilment_by_amazon_fees"]
	}
	if v == "" {
		if field == "marketplace_withheld_tax" {
			return 0, nil
		}
		return 0, fmt.Errorf("payment line %d is missing mapped amount field %q", row.LineStart, field)
	}
	return money.ParseCents(v)
}

func expand(template string, row domain.RawRow) ([]string, error) {
	if strings.TrimSpace(template) == "" {
		return nil, fmt.Errorf("empty record_ref at source line %d", row.LineStart)
	}
	fields := map[string]string{"txn_ref": row.TxnRef, "sku": row.SKU, "settlement_id": row.SettlementID, "date": row.KeyDate, "shipment_id": row.Canonical["shipment_id"], "merchant_order_id": row.Canonical["merchant_order_id"], "description": normalize.Label(row.Description)}
	tokens := strings.Split(template, "+")
	out := make([]string, len(tokens))
	for i, t := range tokens {
		t = strings.TrimSpace(t)
		if v, ok := fields[t]; ok {
			if v == "" {
				return nil, fmt.Errorf("record_ref token %s empty at source line %d", t, row.LineStart)
			}
			out[i] = v
		} else if t == "record_type" {
			out[i] = normalize.Label(row.Transaction)
			if out[i] == "" {
				return nil, fmt.Errorf("record_ref token %s empty at source line %d", t, row.LineStart)
			}
		} else if t == "" || isFieldToken(t) {
			return nil, fmt.Errorf("unknown record_ref token %q at source line %d", t, row.LineStart)
		} else {
			out[i] = t
		}
		if strings.Contains(out[i], "\x1f") {
			return nil, fmt.Errorf("record_ref value contains reserved delimiter at source line %d", row.LineStart)
		}
	}
	return out, nil
}

func isFieldToken(s string) bool {
	if s == "" || strings.ToLower(s) != s {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

func ruleLines(r []domain.ConfigRule) []int {
	out := make([]int, len(r))
	for i, x := range r {
		out[i] = x.OriginLine
	}
	return out
}
