package domain

import (
	"encoding/json"
	"time"
)

type Source string

const (
	SourcePayment    Source = "payment"
	SourceSettlement Source = "settlement"
)

type RowKind string

const (
	RowTransaction RowKind = "transaction"
	RowMetadata    RowKind = "settlement_metadata"
)

type ScopeReason string

const (
	ScopeIn          ScopeReason = "IN_SCOPE"
	ScopeDeferred    ScopeReason = "DEFERRED"
	ScopeOther       ScopeReason = "OTHER_SETTLEMENT"
	ScopeMetadata    ScopeReason = "METADATA"
	ScopeUnsupported ScopeReason = "UNSUPPORTED"
)

type ReconStatus string

const (
	StatusReconciled          ReconStatus = "reconciled"
	StatusUnreconciledPayment ReconStatus = "unreconciled_payment"
	StatusUnreconciledSettle  ReconStatus = "unreconciled_settlement"
)

type RawRow struct {
	Source    Source
	Kind      RowKind
	Ordinal   int
	LineStart int
	LineEnd   int
	ByteStart int64
	ByteEnd   int64
	Raw       map[string]string
	Canonical map[string]string
	// Serialized payloads allow the orchestration layer to release the parser's
	// maps after mapping without losing database lineage.
	RawPayload       []byte
	CanonicalPayload []byte
	SettlementID     string
	Currency         string
	Transaction      string
	Description      string
	AmountType       string
	AmountDesc       string
	SKU              string
	TxnRef           string
	PostedAt         *time.Time
	ReleaseAt        *time.Time
	KeyDate          string
	Status           string
	ReconAmount      int64
	HasRecon         bool
	Scope            ScopeReason
	EventClass       string
}

type ConfigRule struct {
	ID                int64
	Source            Source
	OriginFile        string
	OriginLine        int
	TransactionType   string
	Description       string
	AmountField       string
	AmountType        string
	AmountDescription string
	RecordRef         string
	PositiveTarget    string
	NegativeTarget    string
}

type Contribution struct {
	RowOrdinal int
	Rule       ConfigRule
	Field      string
	Amount     int64
	Target     string
	Decision   string
	KeyParts   []string
	Key        string
}

type MappedRow struct {
	Row           RawRow
	Contributions []Contribution
	KeyParts      []string
	Key           string
	Issues        []string
}

type Group struct {
	KeyParts       []string
	Key            string
	SettlementID   string
	Currency       string
	Scope          ScopeReason
	PaymentRows    []RawRow
	SettlementRows []RawRow
	PaymentAmount  int64
	SettlementAmt  int64
	PaymentBuckets map[string]int64
	SettleBuckets  map[string]int64
}

type Summary struct {
	Buckets map[string]map[Source]int64
}

type RunResult struct {
	PaymentRows        []MappedRow
	SettlementRows     []MappedRow
	Groups             []*Group
	Summary            Summary
	SettlementHeader   int64
	SettlementCurrency string
	Issues             []string
}

func (r RawRow) RawJSON() []byte { b, _ := json.Marshal(r.Raw); return b }
