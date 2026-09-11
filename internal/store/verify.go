package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RunVerification struct {
	RunID             int64
	Mode              string
	Stage             string
	Status            string
	SourceRows        int64
	PaymentRows       int64
	SettlementRows    int64
	MetadataRows      int64
	TransactionRows   int64
	MappingRows       int64
	ContributionRows  int64
	GroupRows         int64
	MemberRows        int64
	IssueRows         int64
	UnmatchedRows     int64
	AmountMismatches  int64
	BucketMismatches  int64
	SummaryMismatches int64
	HeaderMismatches  int64
}

func VerifyRun(ctx context.Context, p *pgxpool.Pool, runID int64, expectedRows int64) (RunVerification, error) {
	var out RunVerification
	out.RunID = runID
	if err := p.QueryRow(ctx, `SELECT mode,stage,verification_status FROM runs WHERE id=$1`, runID).Scan(&out.Mode, &out.Stage, &out.Status); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE source='payment'),count(*) FILTER (WHERE source='settlement'),count(*) FILTER (WHERE row_kind='settlement_metadata'),count(*) FILTER (WHERE row_kind='transaction') FROM source_rows WHERE run_id=$1`, runID).Scan(&out.SourceRows, &out.PaymentRows, &out.SettlementRows, &out.MetadataRows, &out.TransactionRows); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM row_mappings WHERE run_id=$1`, runID).Scan(&out.MappingRows); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM summary_contributions WHERE run_id=$1`, runID).Scan(&out.ContributionRows); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM recon_groups WHERE run_id=$1`, runID).Scan(&out.GroupRows); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM recon_members WHERE run_id=$1`, runID).Scan(&out.MemberRows); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM run_issues WHERE run_id=$1`, runID).Scan(&out.IssueRows); err != nil {
		return out, err
	}
	if out.TransactionRows >= out.MemberRows {
		out.UnmatchedRows = out.TransactionRows - out.MemberRows
	} else {
		out.UnmatchedRows = 0
	}
	var duplicateMembers int64
	if err := p.QueryRow(ctx, `SELECT count(*) FROM (SELECT source_row_id FROM recon_members WHERE run_id=$1 GROUP BY source_row_id HAVING count(*)<>1) duplicate_members`, runID).Scan(&duplicateMembers); err != nil {
		return out, err
	}
	if duplicateMembers != 0 {
		return out, fmt.Errorf("run %d has %d duplicate recon memberships", runID, duplicateMembers)
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM recon_groups WHERE run_id=$1 AND status='reconciled' AND payment_amount<>settlement_amount`, runID).Scan(&out.AmountMismatches); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `WITH payment AS (
		SELECT rm.record_ref,rm.target,SUM(rm.amount) AS amount
		FROM row_mappings rm JOIN source_rows sr ON sr.id=rm.source_row_id
		WHERE rm.run_id=$1 AND sr.source='payment' AND sr.scope_reason='IN_SCOPE' AND rm.target IS NOT NULL AND rm.target<>''
		GROUP BY rm.record_ref,rm.target
	), settlement AS (
		SELECT rm.record_ref,rm.target,SUM(rm.amount) AS amount
		FROM row_mappings rm JOIN source_rows sr ON sr.id=rm.source_row_id
		WHERE rm.run_id=$1 AND sr.source='settlement' AND sr.scope_reason='IN_SCOPE' AND rm.target IS NOT NULL AND rm.target<>''
		GROUP BY rm.record_ref,rm.target
	)
	SELECT count(*) FROM (
		SELECT COALESCE(p.record_ref,s.record_ref) AS record_ref,COALESCE(p.target,s.target) AS target
		FROM payment p FULL OUTER JOIN settlement s ON s.record_ref=p.record_ref AND s.target=p.target
		WHERE COALESCE(p.amount,0)<>COALESCE(s.amount,0)
	) differences`, runID).Scan(&out.BucketMismatches); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `SELECT count(*) FROM (
		SELECT COALESCE(st.source,sc.source),COALESCE(st.field,sc.summary_field)
		FROM (SELECT * FROM summary_totals WHERE run_id=$1) st
		FULL OUTER JOIN (SELECT run_id,source,summary_field,SUM(amount) AS amount FROM summary_contributions WHERE run_id=$1 GROUP BY run_id,source,summary_field) sc
		  ON sc.run_id=st.run_id AND sc.source=st.source AND sc.summary_field=st.field
		WHERE COALESCE(st.amount,0)<>COALESCE(sc.amount,0)
	) mismatches`, runID).Scan(&out.SummaryMismatches); err != nil {
		return out, err
	}
	if err := p.QueryRow(ctx, `WITH checks AS (SELECT c.run_id,c.settlement_id,c.header_total,a.activity
		FROM settlement_controls c JOIN runs r ON r.id=c.run_id
		LEFT JOIN (SELECT run_id,settlement_id,SUM(recon_amount) AS activity FROM source_rows WHERE run_id=$1 AND source='settlement' AND row_kind='transaction' AND scope_reason='IN_SCOPE' GROUP BY run_id,settlement_id) a
		ON a.run_id=c.run_id AND a.settlement_id=c.settlement_id
		WHERE c.run_id=$1 AND c.settlement_id=r.selected_settlement_id)
		SELECT count(*) FILTER (WHERE header_total<>COALESCE(activity,0)) + CASE WHEN count(*)=1 THEN 0 ELSE 1 END FROM checks`, runID).Scan(&out.HeaderMismatches); err != nil {
		return out, err
	}
	if expectedRows > 0 && out.SourceRows != expectedRows {
		return out, fmt.Errorf("run %d retained %d source rows, want %d", runID, out.SourceRows, expectedRows)
	}
	if out.Stage != "REPORTED" {
		return out, fmt.Errorf("run %d stage=%s, want REPORTED", runID, out.Stage)
	}
	if out.Mode == "strict" && out.Status != "PASS" {
		return out, fmt.Errorf("strict run %d verification_status=%s, want PASS", runID, out.Status)
	}
	if out.Mode == "strict" && out.IssueRows != 0 {
		return out, fmt.Errorf("strict run %d has %d persisted issues", runID, out.IssueRows)
	}
	if out.AmountMismatches != 0 || out.SummaryMismatches != 0 {
		return out, fmt.Errorf("run %d has amount mismatches=%d summary mismatches=%d", runID, out.AmountMismatches, out.SummaryMismatches)
	}
	if out.Mode == "strict" && out.BucketMismatches != 0 {
		return out, fmt.Errorf("strict run %d has %d per-key bucket mismatches", runID, out.BucketMismatches)
	}
	if out.HeaderMismatches != 0 {
		return out, fmt.Errorf("run %d has %d settlement header mismatches", runID, out.HeaderMismatches)
	}
	if out.MemberRows != out.TransactionRows {
		return out, fmt.Errorf("run %d member count=%d, want transaction rows=%d", runID, out.MemberRows, out.TransactionRows)
	}
	return out, nil
}
