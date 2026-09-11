package store

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterReport(ctx context.Context, p *pgxpool.Pool, runID int64, kind, path string) error {
	info, err := FileInfoForPath(path)
	if err != nil {
		return err
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO report_artifacts(run_id,report_kind,path,sha256,byte_size,report_status) VALUES($1,$2,$3,$4,$5,'VERIFIED') ON CONFLICT(run_id,report_kind) DO UPDATE SET path=EXCLUDED.path,sha256=EXCLUDED.sha256,byte_size=EXCLUDED.byte_size,report_status='VERIFIED'`, runID, kind, info.Path, info.SHA256, info.Bytes); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE runs SET stage='REPORTED',verification_status=CASE mode WHEN 'strict' THEN 'PASS' ELSE 'DIAGNOSTIC' END,completed_at=now() WHERE id=$1 AND stage IN ('RECONCILED','REPORTED')`, runID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
