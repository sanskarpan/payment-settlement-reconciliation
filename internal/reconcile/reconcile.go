package reconcile

import (
	"sort"
	"strings"

	"reconciliation/internal/domain"
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
			} else {
				r.Scope = domain.ScopeIn
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

func BuildGroups(payments, settlements []domain.MappedRow) []*domain.Group {
	m := map[string]*domain.Group{}
	add := func(row domain.MappedRow, payment bool) {
		if row.Row.Kind == domain.RowMetadata {
			return
		}
		key := row.Key + "\x1f" + row.Row.SettlementID + "\x1f" + row.Row.Currency + "\x1f" + string(row.Row.Scope)
		g := m[key]
		if g == nil {
			g = &domain.Group{Key: row.Key, KeyParts: row.KeyParts, SettlementID: row.Row.SettlementID, Currency: row.Row.Currency, Scope: row.Row.Scope, PaymentBuckets: map[string]int64{}, SettleBuckets: map[string]int64{}}
			m[key] = g
		}
		if payment {
			g.PaymentRows = append(g.PaymentRows, row.Row)
			g.PaymentAmount += row.Row.ReconAmount
			for _, c := range row.Contributions {
				if c.Amount != 0 && c.Target != "" {
					g.PaymentBuckets[c.Target] += c.Amount
				}
			}
		} else {
			g.SettlementRows = append(g.SettlementRows, row.Row)
			g.SettlementAmt += row.Row.ReconAmount
			for _, c := range row.Contributions {
				if c.Amount != 0 && c.Target != "" {
					g.SettleBuckets[c.Target] += c.Amount
				}
			}
		}
	}
	for _, r := range payments {
		add(r, true)
	}
	for _, r := range settlements {
		add(r, false)
	}
	out := make([]*domain.Group, 0, len(m))
	for _, g := range m {
		sort.Slice(g.PaymentRows, func(i, j int) bool { return g.PaymentRows[i].LineStart < g.PaymentRows[j].LineStart })
		sort.Slice(g.SettlementRows, func(i, j int) bool { return g.SettlementRows[i].LineStart < g.SettlementRows[j].LineStart })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		return groupRank(out[i]) < groupRank(out[j]) || (groupRank(out[i]) == groupRank(out[j]) && out[i].Key < out[j].Key)
	})
	return out
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
