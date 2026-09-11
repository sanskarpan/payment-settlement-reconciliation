package reconcile

import (
	"math"
	"testing"

	"reconciliation/internal/domain"
)

func TestGroupPresenceIsIndependentOfAmount(t *testing.T) {
	p := domain.MappedRow{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: 100, LineStart: 1}, Key: "same", KeyParts: []string{"same"}}
	s := domain.MappedRow{Row: domain.RawRow{Source: domain.SourceSettlement, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: 90, LineStart: 2}, Key: "same", KeyParts: []string{"same"}}
	groups, err := BuildGroups([]domain.MappedRow{p}, []domain.MappedRow{s})
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].PaymentRows) != 1 || len(groups[0].SettlementRows) != 1 || groups[0].PaymentAmount == groups[0].SettlementAmt {
		t.Fatalf("unexpected group: %+v", groups)
	}
}

func TestGroupAmountOverflowFails(t *testing.T) {
	rows := []domain.MappedRow{
		{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: math.MaxInt64}, Key: "k"},
		{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "s", Currency: "AUD", ReconAmount: 1}, Key: "k"},
	}
	if _, err := BuildGroups(rows, nil); err == nil {
		t.Fatal("group overflow accepted")
	}
}

func TestGroupIdentityCannotCollide(t *testing.T) {
	a := domain.MappedRow{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "a", Currency: "bc"}, Key: "k"}
	b := domain.MappedRow{Row: domain.RawRow{Source: domain.SourcePayment, Scope: domain.ScopeIn, SettlementID: "ab", Currency: "c"}, Key: "k"}
	groups, err := BuildGroups([]domain.MappedRow{a, b}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("partition identities collided: %d groups", len(groups))
	}
}

func TestClassifyRejectsUnknownSelectedPaymentStatus(t *testing.T) {
	rows := Classify([]domain.RawRow{{Source: domain.SourcePayment, Kind: domain.RowTransaction, SettlementID: "s", Status: "Pending"}}, "s")
	if rows[0].Scope != domain.ScopeUnsupported {
		t.Fatalf("scope=%s, want %s", rows[0].Scope, domain.ScopeUnsupported)
	}
}
