package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"reconciliation/internal/domain"
)

// FileInfoForPath computes the immutable identity used for config provenance.
// It reads bytes only; it never rewrites or normalizes the source file.
func FileInfoForPath(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, err
	}
	defer f.Close()
	h := sha256.New()
	var n int64
	buf := make([]byte, 128*1024)
	for {
		read, readErr := f.Read(buf)
		if read > 0 {
			if _, err = h.Write(buf[:read]); err != nil {
				return FileInfo{}, err
			}
			n += int64(read)
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return FileInfo{}, readErr
		}
	}
	return FileInfo{Path: path, SHA256: hex.EncodeToString(h.Sum(nil)), Bytes: n}, nil
}

// FrozenConfig is the database identity of an immutable pair of mapping files.
type FrozenConfig struct {
	ID            int64
	Name          string
	ContentSHA256 string
}

// EnsureFrozenConfig imports both mapping files and their parsed rules into a
// frozen config version. Existing versions are returned only when their
// content hash is identical; a frozen row is never updated in place.
func EnsureFrozenConfig(ctx context.Context, p *pgxpool.Pool, name, note string, paymentFile, settlementFile FileInfo, rules []domain.ConfigRule) (FrozenConfig, error) {
	if paymentFile.SHA256 == "" || settlementFile.SHA256 == "" {
		return FrozenConfig{}, fmt.Errorf("config files must have content hashes")
	}
	contentHash := configHash(paymentFile, settlementFile, rules)
	if name == "" {
		name = "config-" + contentHash[:16]
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return FrozenConfig{}, err
	}
	defer tx.Rollback(ctx)
	var existing FrozenConfig
	var state string
	err = tx.QueryRow(ctx, `SELECT id,name,content_sha256,state FROM config_versions WHERE name=$1`, name).Scan(&existing.ID, &existing.Name, &existing.ContentSHA256, &state)
	if err == nil {
		if state != "FROZEN" || existing.ContentSHA256 != contentHash {
			return FrozenConfig{}, fmt.Errorf("config version %q already exists with different or unfrozen content", name)
		}
		if err = tx.Commit(ctx); err != nil {
			return FrozenConfig{}, err
		}
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return FrozenConfig{}, err
	}
	err = tx.QueryRow(ctx, `SELECT id,name,content_sha256,state FROM config_versions WHERE content_sha256=$1`, contentHash).Scan(&existing.ID, &existing.Name, &existing.ContentSHA256, &state)
	if err == nil {
		if state != "FROZEN" {
			return FrozenConfig{}, fmt.Errorf("config content %s belongs to an unfrozen version", contentHash)
		}
		if err = tx.Commit(ctx); err != nil {
			return FrozenConfig{}, err
		}
		return existing, nil
	}
	if err != pgx.ErrNoRows {
		return FrozenConfig{}, err
	}
	if err = tx.QueryRow(ctx, `INSERT INTO config_versions(name,state,note) VALUES($1,'DRAFT',$2) RETURNING id`, name, note).Scan(&existing.ID); err != nil {
		return FrozenConfig{}, err
	}
	existing.Name, existing.ContentSHA256 = name, contentHash
	fileIDs := make(map[string]int64, 2)
	for _, item := range []struct {
		kind string
		file FileInfo
	}{{"payment_config", paymentFile}, {"settlement_config", settlementFile}} {
		var id int64
		if err = tx.QueryRow(ctx, `INSERT INTO source_files(source_kind,path,sha256,byte_size) VALUES($1,$2,$3,$4) ON CONFLICT(source_kind,sha256) DO UPDATE SET path=EXCLUDED.path RETURNING id`, item.kind, item.file.Path, item.file.SHA256, item.file.Bytes).Scan(&id); err != nil {
			return FrozenConfig{}, err
		}
		fileIDs[item.kind] = id
		if _, err = tx.Exec(ctx, `INSERT INTO config_version_files(config_version_id,source_kind,source_file_id) VALUES($1,$2,$3)`, existing.ID, item.kind, id); err != nil {
			return FrozenConfig{}, err
		}
	}
	ruleBatch := &pgx.Batch{}
	for _, rule := range rules {
		ruleBatch.Queue(`INSERT INTO mapping_rules(config_version_id,source,origin_file,origin_line,transaction_type,description,amount_field,amount_type,amount_description,record_ref,positive_target,negative_target) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, existing.ID, rule.Source, rule.OriginFile, rule.OriginLine, rule.TransactionType, nullable(rule.Description), nullable(rule.AmountField), nullable(rule.AmountType), nullable(rule.AmountDescription), rule.RecordRef, nullable(rule.PositiveTarget), nullable(rule.NegativeTarget))
	}
	results := tx.SendBatch(ctx, ruleBatch)
	for range rules {
		if _, err = results.Exec(); err != nil {
			_ = results.Close()
			return FrozenConfig{}, err
		}
	}
	if err = results.Close(); err != nil {
		return FrozenConfig{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE config_versions SET state='FROZEN',frozen_at=now(),content_sha256=$2 WHERE id=$1`, existing.ID, contentHash); err != nil {
		return FrozenConfig{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return FrozenConfig{}, err
	}
	return existing, nil
}

func configHash(payment, settlement FileInfo, rules []domain.ConfigRule) string {
	ordered := append([]domain.ConfigRule(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Source != ordered[j].Source {
			return ordered[i].Source < ordered[j].Source
		}
		if ordered[i].OriginFile != ordered[j].OriginFile {
			return ordered[i].OriginFile < ordered[j].OriginFile
		}
		return ordered[i].OriginLine < ordered[j].OriginLine
	})
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|", payment.SHA256, settlement.SHA256)
	for _, r := range ordered {
		fmt.Fprintf(h, "%s|%s|%d|%s|%s|%s|%s|%s|%s|%s|%s|", r.Source, r.OriginFile, r.OriginLine, r.TransactionType, r.Description, r.AmountField, r.AmountType, r.AmountDescription, r.RecordRef, r.PositiveTarget, r.NegativeTarget)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func LookupConfigVersion(ctx context.Context, p *pgxpool.Pool, name string) (FrozenConfig, error) {
	var out FrozenConfig
	var state string
	if err := p.QueryRow(ctx, `SELECT id,name,content_sha256,state FROM config_versions WHERE name=$1`, name).Scan(&out.ID, &out.Name, &out.ContentSHA256, &state); err != nil {
		return FrozenConfig{}, err
	}
	if state != "FROZEN" {
		return FrozenConfig{}, fmt.Errorf("config version %q is %s, want FROZEN", name, state)
	}
	return out, nil
}

func LoadRules(ctx context.Context, p *pgxpool.Pool, versionID int64) ([]domain.ConfigRule, error) {
	rows, err := p.Query(ctx, `SELECT source,origin_file,origin_line,transaction_type,COALESCE(description,''),COALESCE(amount_field,''),COALESCE(amount_type,''),COALESCE(amount_description,''),record_ref,COALESCE(positive_target,''),COALESCE(negative_target,'') FROM mapping_rules WHERE config_version_id=$1 ORDER BY source,origin_line`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ConfigRule
	for rows.Next() {
		var r domain.ConfigRule
		if err = rows.Scan(&r.Source, &r.OriginFile, &r.OriginLine, &r.TransactionType, &r.Description, &r.AmountField, &r.AmountType, &r.AmountDescription, &r.RecordRef, &r.PositiveTarget, &r.NegativeTarget); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
