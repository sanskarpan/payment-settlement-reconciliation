package reconcile

import (
	"fmt"
	"sort"
	"strings"

	"reconciliation/internal/domain"
	"reconciliation/internal/money"
)

func Classify(rows []domain.RawRow, selected string) []domain.RawRow {
	for i := range rows {
		r := &rows[i]
		if r.Kind == domain.RowMetadata {
			r.Scope = domain.ScopeMetadata
			continue
		}
		if r.Source == domain.SourcePayment {
			if r.SettlementID != selected {
				r.Scope = domain.ScopeOther
			} else if strings.EqualFold(r.Status, "Deferred") {
				r.Scope = domain.ScopeDeferred
			} else if strings.EqualFold(r.Status, "Released") {
				r.Scope = domain.ScopeIn
			} else {
				r.Scope = domain.ScopeUnsupported
			}
		} else {
			if r.SettlementID == selected {
				r.Scope = domain.ScopeIn
			} else {
				r.Scope = domain.ScopeOther
			}
		}
	}
	return rows
}

func BuildGroups(payments, settlements []domain.MappedRow) ([]*domain.Group, error) {
	m := map[string]*domain.Group{}
	add := func(row domain.MappedRow, payment bool) error {
		if row.Row.Kind == domain.RowMetadata {
			return nil
		}
		key := identityKey(row.Key, row.Row.SettlementID, row.Row.Currency, string(row.Row.Scope))
		g := m[key]
		if g == nil {
			g = &domain.Group{Key: row.Key, KeyParts: row.KeyParts, SettlementID: row.Row.SettlementID, Currency: row.Row.Currency, Scope: row.Row.Scope, PaymentBuckets: map[string]int64{}, SettleBuckets: map[string]int64{}}
			m[key] = g
		}
		if payment {
			g.PaymentRows = append(g.PaymentRows, row.Row)
			var err error
			g.PaymentAmount, err = money.Add(g.PaymentAmount, row.Row.ReconAmount)
			if err != nil {
				return fmt.Errorf("payment group %q: %w", row.Key, err)
			}
			for _, c := range row.Contributions {
				if c.Amount != 0 && c.Target != "" {
					g.PaymentBuckets[c.Target], err = money.Add(g.PaymentBuckets[c.Target], c.Amount)
					if err != nil {
						return fmt.Errorf("payment group %q bucket %s: %w", row.Key, c.Target, err)
					}
				}
			}
		} else {
			g.SettlementRows = append(g.SettlementRows, row.Row)
			var err error
			g.SettlementAmt, err = money.Add(g.SettlementAmt, row.Row.ReconAmount)
			if err != nil {
				return fmt.Errorf("settlement group %q: %w", row.Key, err)
			}
			for _, c := range row.Contributions {
				if c.Amount != 0 && c.Target != "" {
					g.SettleBuckets[c.Target], err = money.Add(g.SettleBuckets[c.Target], c.Amount)
					if err != nil {
						return fmt.Errorf("settlement group %q bucket %s: %w", row.Key, c.Target, err)
					}
				}
			}
		}
		return nil
	}
	for _, r := range payments {
		if err := add(r, true); err != nil {
			return nil, err
		}
	}
	for _, r := range settlements {
		if err := add(r, false); err != nil {
			return nil, err
		}
	}
	out := make([]*domain.Group, 0, len(m))
	for _, g := range m {
		sort.Slice(g.PaymentRows, func(i, j int) bool { return g.PaymentRows[i].LineStart < g.PaymentRows[j].LineStart })
		sort.Slice(g.SettlementRows, func(i, j int) bool { return g.SettlementRows[i].LineStart < g.SettlementRows[j].LineStart })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if groupRank(left) != groupRank(right) {
			return groupRank(left) < groupRank(right)
		}
		if left.SettlementID != right.SettlementID {
			return left.SettlementID < right.SettlementID
		}
		if left.Currency != right.Currency {
			return left.Currency < right.Currency
		}
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		return left.Key < right.Key
	})
	return out, nil
}

func identityKey(parts ...string) string {
	var b strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&b, "%d:%s", len(part), part)
	}
	return b.String()
}

func groupRank(g *domain.Group) int {
	if len(g.PaymentRows) > 0 && len(g.SettlementRows) > 0 {
		return 0
	}
	if len(g.PaymentRows) > 0 {
		return 1
	}
	return 2
}
