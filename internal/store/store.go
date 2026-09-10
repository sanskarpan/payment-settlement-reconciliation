package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reconciliation/internal/domain"
	"reconciliation/internal/money"
)

type FileInfo struct {
	Path, SHA256 string
	Bytes        int64
}

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	files, err := filepath.Glob(filepath.Join("migrations", "*.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no migration files found")
	}
	sort.Strings(files)
	if _, err = p.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	for _, path := range files {
		version := filepath.Base(path)
		var applied bool
		if err = p.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tx, beginErr := p.Begin(ctx)
		if beginErr != nil {
			return beginErr
		}
		if _, execErr := tx.Exec(ctx, string(b)); execErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", version, execErr)
		}
		if _, execErr := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); execErr != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record %s: %w", version, execErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return fmt.Errorf("commit %s: %w", version, commitErr)
		}
	}
	return nil
}

func Persist(ctx context.Context, p *pgxpool.Pool, name, selected, mode string, configVersionID int64, retry bool, pi, si FileInfo, payments, settlements []domain.MappedRow, groups []*domain.Group, summary domain.Summary, rules []domain.ConfigRule, issues []string) (runID int64, retErr error) {
	if configVersionID <= 0 {
		return 0, fmt.Errorf("config version id is required for persistence")
	}
	if mode == "strict" {
		for field, sides := range summary.Buckets {
			if sides[domain.SourcePayment] != sides[domain.SourceSettlement] {
				return 0, fmt.Errorf("strict run summary mismatch for %s: payment=%s settlement=%s", field, money.FormatCents(sides[domain.SourcePayment]), money.FormatCents(sides[domain.SourceSettlement]))
			}
		}
		for _, group := range groups {
			if len(group.PaymentRows) > 0 && len(group.SettlementRows) > 0 && group.PaymentAmount != group.SettlementAmt {
				return 0, fmt.Errorf("strict run amount mismatch for record_ref %q", group.Key)
			}
		}
	}
	fingerprint := runFingerprint(pi, si, selected, mode, configVersionID, rules)
	runID, existing, err := claimRun(ctx, p, name, fingerprint, selected, mode, configVersionID, retry)
	if err != nil {
		return 0, err
	}
	if existing {
		return runID, nil
	}
	claimedID := runID
	defer func() {
		if retErr != nil {
			markRunFailed(context.Background(), p, claimedID, retErr)
		}
	}()
	tx, err := p.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	fileIDs := map[domain.Source]int64{}
	for _, x := range []struct {
		s domain.Source
		i FileInfo
	}{{domain.SourcePayment, pi}, {domain.SourceSettlement, si}} {
		var id int64
		err = tx.QueryRow(ctx, `INSERT INTO source_files(source_kind,path,sha256,byte_size) VALUES($1,$2,$3,$4) ON CONFLICT(source_kind,sha256) DO UPDATE SET path=EXCLUDED.path RETURNING id`, x.s, x.i.Path, x.i.SHA256, x.i.Bytes).Scan(&id)
		if err != nil {
			return 0, err
		}
		fileIDs[x.s] = id
	}
	if _, err = tx.Exec(ctx, `INSERT INTO run_files(run_id,role,source_file_id) VALUES($1,'payment',$2),($1,'settlement',$3)`, runID, fileIDs[domain.SourcePayment], fileIDs[domain.SourceSettlement]); err != nil {
		return 0, err
	}
	for role, kind := range map[string]string{"payment_config": "payment_config", "settlement_config": "settlement_config"} {
		if _, err = tx.Exec(ctx, `INSERT INTO run_files(run_id,role,source_file_id) SELECT $1,$2,source_file_id FROM config_version_files WHERE config_version_id=$3 AND source_kind=$4`, runID, role, configVersionID, kind); err != nil {
			return 0, err
		}
	}
	ruleIDs := map[string]int64{}
	for i := range rules {
		r := rules[i]
		var id int64
		err = tx.QueryRow(ctx, `SELECT id FROM mapping_rules WHERE config_version_id=$1 AND source=$2 AND origin_file=$3 AND origin_line=$4`, configVersionID, r.Source, r.OriginFile, r.OriginLine).Scan(&id)
		if err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx, `INSERT INTO mapping_rules(config_version_id,source,origin_file,origin_line,transaction_type,description,amount_field,amount_type,amount_description,record_ref,positive_target,negative_target) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id`, configVersionID, r.Source, r.OriginFile, r.OriginLine, r.TransactionType, nullable(r.Description), nullable(r.AmountField), nullable(r.AmountType), nullable(r.AmountDescription), r.RecordRef, nullable(r.PositiveTarget), nullable(r.NegativeTarget)).Scan(&id)
		}
		if err != nil {
			return 0, err
		}
		ruleIDs[ruleKey(r)] = id
	}
	rowIDs := map[string]int64{}
	all := append(append([]domain.MappedRow{}, payments...), settlements...)
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE source_rows_stage (
		run_id BIGINT, source_file_id BIGINT, source TEXT, row_kind TEXT, ordinal INTEGER,
		line_start INTEGER, line_end INTEGER, settlement_id TEXT, currency TEXT,
		transaction_type TEXT, description TEXT, amount_type TEXT, amount_description TEXT,
		sku TEXT, txn_ref TEXT, key_date DATE, status TEXT, recon_amount NUMERIC,
		scope_reason TEXT, raw_payload JSONB, canonical_payload JSONB
	) ON COMMIT DROP`); err != nil {
		return 0, err
	}
	stageRows := make([][]any, 0, len(all))
	for _, m := range all {
		r := m.Row
		raw := r.RawPayload
		if len(raw) == 0 {
			raw, _ = json.Marshal(r.Raw)
		}
		canonical := r.CanonicalPayload
		if len(canonical) == 0 {
			canonical, _ = json.Marshal(r.Canonical)
		}
		var keyDate any
		if r.KeyDate != "" {
			keyDate = r.KeyDate
		}
		var recon any
		if r.HasRecon {
			recon = money.FormatCents(r.ReconAmount)
		}
		stageRows = append(stageRows, []any{runID, fileIDs[r.Source], r.Source, r.Kind, r.Ordinal, r.LineStart, r.LineEnd, nullable(r.SettlementID), nullable(r.Currency), nullable(r.Transaction), nullable(r.Description), nullable(r.AmountType), nullable(r.AmountDesc), nullable(r.SKU), nullable(r.TxnRef), keyDate, nullable(r.Status), recon, r.Scope, raw, canonical})
	}
	if _, err = tx.CopyFrom(ctx, pgx.Identifier{"source_rows_stage"}, []string{"run_id", "source_file_id", "source", "row_kind", "ordinal", "line_start", "line_end", "settlement_id", "currency", "transaction_type", "description", "amount_type", "amount_description", "sku", "txn_ref", "key_date", "status", "recon_amount", "scope_reason", "raw_payload", "canonical_payload"}, pgx.CopyFromRows(stageRows)); err != nil {
		return 0, err
	}
	insertedRows, err := tx.Query(ctx, `INSERT INTO source_rows(run_id,source_file_id,source,row_kind,ordinal,line_start,line_end,settlement_id,currency,transaction_type,description,amount_type,amount_description,sku,txn_ref,key_date,status,recon_amount,scope_reason,raw_payload,canonical_payload)
		SELECT run_id,source_file_id,source,row_kind,ordinal,line_start,line_end,settlement_id,currency,transaction_type,description,amount_type,amount_description,sku,txn_ref,key_date,status,recon_amount,scope_reason,raw_payload,canonical_payload
		FROM source_rows_stage ORDER BY source, ordinal RETURNING id, source, ordinal`)
	if err != nil {
		return 0, err
	}
	for insertedRows.Next() {
		var id int64
		var source domain.Source
		var ordinal int
		if err = insertedRows.Scan(&id, &source, &ordinal); err != nil {
			insertedRows.Close()
			return 0, err
		}
		rowIDs[fmt.Sprintf("%s|%d", source, ordinal)] = id
	}
	if err = insertedRows.Err(); err != nil {
		insertedRows.Close()
		return 0, err
	}
	insertedRows.Close()
	mappingRows := make([][]any, 0)
	for _, m := range all {
		id := rowIDs[rowKey(m.Row)]
		for _, c := range m.Contributions {
			rid := ruleIDs[ruleKey(c.Rule)]
			if rid == 0 {
				return 0, fmt.Errorf("missing rule id for %s line %d", c.Rule.Source, c.Rule.OriginLine)
			}
			mappingRows = append(mappingRows, []any{runID, id, rid, c.Field, money.FormatCents(c.Amount), nullable(c.Target), c.Decision, c.Key})
		}
	}
	if len(mappingRows) > 0 {
		if _, err = tx.CopyFrom(ctx, pgx.Identifier{"row_mappings"}, []string{"run_id", "source_row_id", "rule_id", "amount_field", "amount", "target", "decision", "record_ref"}, pgx.CopyFromRows(mappingRows)); err != nil {
			return 0, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO summary_contributions(run_id,source_row_id,row_mapping_id,source,scope_reason,settlement_id,currency,summary_field,amount)
		SELECT rm.run_id,rm.source_row_id,rm.id,sr.source,sr.scope_reason,sr.settlement_id,sr.currency,rm.target,rm.amount
		FROM row_mappings rm JOIN source_rows sr ON sr.id=rm.source_row_id
		WHERE rm.run_id=$1 AND sr.scope_reason='IN_SCOPE' AND rm.target IS NOT NULL AND rm.target<>'' AND rm.amount<>0
		ON CONFLICT (run_id,row_mapping_id) DO NOTHING`, runID); err != nil {
		return 0, err
	}
	for _, m := range all {
		if m.Row.Source != domain.SourceSettlement || m.Row.Kind != domain.RowMetadata || m.Row.SettlementID == "" {
			continue
		}
		header, parseErr := money.ParseCents(m.Row.Raw["total-amount"])
		if parseErr != nil {
			return 0, fmt.Errorf("settlement metadata line %d total-amount: %w", m.Row.LineStart, parseErr)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO settlement_controls(run_id,settlement_id,metadata_row_id,currency,header_total) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, runID, m.Row.SettlementID, rowIDs[rowKey(m.Row)], m.Row.Currency, money.FormatCents(header)); err != nil {
			return 0, err
		}
	}
	for field, bySource := range summary.Buckets {
		for source, amount := range bySource {
			if amount == 0 {
				continue
			}
			var count int64
			err = tx.QueryRow(ctx, `SELECT count(*) FROM row_mappings rm JOIN source_rows sr ON sr.id=rm.source_row_id WHERE rm.run_id=$1 AND sr.source=$2 AND rm.target=$3 AND rm.amount<>0`, runID, source, field).Scan(&count)
			if err != nil {
				return 0, err
			}
			_, err = tx.Exec(ctx, `INSERT INTO summary_totals(run_id,source,field,amount,contribution_count) VALUES($1,$2,$3,$4,$5)`, runID, source, field, money.FormatCents(amount), count)
			if err != nil {
				return 0, err
			}
		}
	}
	groupBatch := &pgx.Batch{}
	for _, g := range groups {
		status := "reconciled"
		if len(g.PaymentRows) == 0 {
			status = "unreconciled_settlement"
		} else if len(g.SettlementRows) == 0 {
			status = "unreconciled_payment"
		}
		groupBatch.Queue(`INSERT INTO recon_groups(run_id,record_ref,settlement_id,currency,scope_reason,payment_rows,payment_amount,settlement_rows,settlement_amount,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`, runID, g.Key, g.SettlementID, g.Currency, g.Scope, len(g.PaymentRows), money.FormatCents(g.PaymentAmount), len(g.SettlementRows), money.FormatCents(g.SettlementAmt), status)
	}
	groupIDs := make([]int64, len(groups))
	groupResults := tx.SendBatch(ctx, groupBatch)
	for i := range groups {
		if err = groupResults.QueryRow().Scan(&groupIDs[i]); err != nil {
			_ = groupResults.Close()
			return 0, err
		}
	}
	if err = groupResults.Close(); err != nil {
		return 0, err
	}
	members := make([][]any, 0)
	for i, g := range groups {
		for _, row := range g.PaymentRows {
			members = append(members, []any{runID, groupIDs[i], rowIDs[rowKey(row)]})
		}
		for _, row := range g.SettlementRows {
			members = append(members, []any{runID, groupIDs[i], rowIDs[rowKey(row)]})
		}
	}
	if len(members) > 0 {
		if _, err = tx.CopyFrom(ctx, pgx.Identifier{"recon_members"}, []string{"run_id", "recon_group_id", "source_row_id"}, pgx.CopyFromRows(members)); err != nil {
			return 0, err
		}
	}
	for _, issue := range issues {
		if _, err = tx.Exec(ctx, `INSERT INTO run_issues(run_id,severity,code,detail) VALUES($1,'WARN','MAPPING_OR_RECONCILIATION',jsonb_build_object('message',$2::text))`, runID, issue); err != nil {
			return 0, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE runs SET stage='REPORTED',verification_status=$2,completed_at=$3 WHERE id=$1`, runID, func() string {
		if mode == "strict" {
			return "PASS"
		}
		return "DIAGNOSTIC"
	}(), time.Now().UTC()); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return runID, nil
}

func runFingerprint(pi, si FileInfo, selected, mode string, configVersionID int64, rules []domain.ConfigRule) string {
	h := sha256.New()
	h.Write([]byte(pi.SHA256))
	h.Write([]byte(si.SHA256))
	h.Write([]byte(selected))
	h.Write([]byte(mode))
	h.Write([]byte(fmt.Sprintf("|config-version:%d|engine-version:reconciliation-v2", configVersionID)))
	for _, r := range rules {
		h.Write([]byte(fmt.Sprintf("%s|%s|%d|%s|%s|%s|%s|%s|%s", r.Source, r.OriginFile, r.OriginLine, r.TransactionType, r.Description+r.AmountType+r.AmountDescription, r.AmountField, r.RecordRef, r.PositiveTarget, r.NegativeTarget)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func claimRun(ctx context.Context, p *pgxpool.Pool, name, fingerprint, selected, mode string, configVersionID int64, retry bool) (int64, bool, error) {
	tx, err := p.Begin(ctx)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback(ctx)
	var runID int64
	err = tx.QueryRow(ctx, `INSERT INTO runs(name,fingerprint,selected_settlement_id,mode,stage,verification_status,config_version_id,normalization_version,layout_version,engine_version) VALUES($1,$2,$3,$4,'CLAIMED','NOT_CHECKED',$5,'normalization-v1','layout-v1','reconciliation-v2') ON CONFLICT(fingerprint) DO NOTHING RETURNING id`, name, fingerprint, selected, mode, configVersionID).Scan(&runID)
	if err == nil {
		if err = tx.Commit(ctx); err != nil {
			return 0, false, err
		}
		return runID, false, nil
	}
	if err != pgx.ErrNoRows {
		return 0, false, err
	}
	var stage, status string
	var attempt int
	if err = tx.QueryRow(ctx, `SELECT id,stage,verification_status,attempt FROM runs WHERE fingerprint=$1 FOR UPDATE`, fingerprint).Scan(&runID, &stage, &status, &attempt); err != nil {
		return 0, false, err
	}
	if stage == "REPORTED" {
		if err = tx.Commit(ctx); err != nil {
			return 0, false, err
		}
		return runID, true, nil
	}
	if stage == "FAILED" && retry {
		if _, err = tx.Exec(ctx, `UPDATE runs SET stage='CLAIMED',verification_status='NOT_CHECKED',attempt=$2,error_code=NULL,error_detail=NULL,completed_at=NULL WHERE id=$1`, runID, attempt+1); err != nil {
			return 0, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return 0, false, err
		}
		return runID, false, nil
	}
	return 0, false, fmt.Errorf("run fingerprint %s is already %s (use --retry only for FAILED runs)", fingerprint, stage)
}

func markRunFailed(ctx context.Context, p *pgxpool.Pool, runID int64, cause error) {
	detail := cause.Error()
	if len(detail) > 4000 {
		detail = detail[:4000]
	}
	_, _ = p.Exec(ctx, `UPDATE runs SET stage='FAILED',verification_status='FAIL',error_code='PERSIST_FAILED',error_detail=$2,completed_at=now() WHERE id=$1 AND stage='CLAIMED'`, runID, detail)
}

func ruleKey(r domain.ConfigRule) string {
	return fmt.Sprintf("%s|%s|%d", r.Source, r.OriginFile, r.OriginLine)
}
func rowKey(r domain.RawRow) string { return fmt.Sprintf("%s|%d", r.Source, r.Ordinal) }
func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
