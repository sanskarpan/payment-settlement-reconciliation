package store

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ExplainRow struct {
	Source       string
	PhysicalLine int
	Ordinal      int
	Scope        string
	SettlementID string
	Currency     string
	RecordRef    string
	AmountField  string
	Amount       string
	Target       string
	Decision     string
	RuleFile     string
	RuleLine     int
}

func Explain(ctx context.Context, p *pgxpool.Pool, runID int64, recordRef, field string, physicalLine int) ([]ExplainRow, error) {
	query := `SELECT sr.source,sr.line_start,sr.ordinal,sr.scope_reason,COALESCE(sr.settlement_id,''),COALESCE(sr.currency,''),rm.record_ref,rm.amount_field,rm.amount,COALESCE(rm.target,''),rm.decision,mr.origin_file,mr.origin_line
		FROM row_mappings rm JOIN source_rows sr ON sr.id=rm.source_row_id JOIN mapping_rules mr ON mr.id=rm.rule_id
		WHERE rm.run_id=$1`
	args := []any{runID}
	if recordRef != "" {
		query += " AND rm.record_ref=$" + itoa(len(args)+1)
		args = append(args, recordRef)
	}
	if field != "" {
		query += " AND rm.target=$" + itoa(len(args)+1)
		args = append(args, field)
	}
	if physicalLine > 0 {
		query += " AND sr.line_start=$" + itoa(len(args)+1)
		args = append(args, physicalLine)
	}
	query += " ORDER BY sr.source,sr.line_start,rm.id"
	rows, err := p.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExplainRow
	for rows.Next() {
		var x ExplainRow
		if err = rows.Scan(&x.Source, &x.PhysicalLine, &x.Ordinal, &x.Scope, &x.SettlementID, &x.Currency, &x.RecordRef, &x.AmountField, &x.Amount, &x.Target, &x.Decision, &x.RuleFile, &x.RuleLine); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
