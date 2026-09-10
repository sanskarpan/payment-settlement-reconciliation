package reconcile

import (
	"testing"

	"reconciliation/internal/domain"
)

func TestGroupPresenceIsIndependentOfAmount(t *testing.T) {
	p := domain.MappedRow{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: 100, LineStart: 1}, Key: "same", KeyParts: []string{"same"}}
	s := domain.MappedRow{Row: domain.RawRow{Source: domain.SourceSettlement, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: 90, LineStart: 2}, Key: "same", KeyParts: []string{"same"}}
	groups := BuildGroups([]domain.MappedRow{p}, []domain.MappedRow{s})
	if len(groups) != 1 || len(groups[0].PaymentRows) != 1 || len(groups[0].SettlementRows) != 1 || groups[0].PaymentAmount == groups[0].SettlementAmt {
		t.Fatalf("unexpected group: %+v", groups)
	}
}
