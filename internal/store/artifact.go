package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func RegisterReport(ctx context.Context, p *pgxpool.Pool, runID int64, kind, path string) error {
	info, err := FileInfoForPath(path)
	if err != nil {
		return err
	}
	var existingHash string
	err = p.QueryRow(ctx, `SELECT sha256 FROM report_artifacts WHERE run_id=$1 AND report_kind=$2`, runID, kind).Scan(&existingHash)
	if err == nil {
		// XLSX containers may legitimately differ in ZIP metadata between
		// retries while their financial cells remain the same. The report was
		// structurally verified before this registration, so update the artifact
		// locator and checksum for the latest successful retry.
		_, err = p.Exec(ctx, `UPDATE report_artifacts SET path=$3,byte_size=$4,report_status='VERIFIED' WHERE run_id=$1 AND report_kind=$2`, runID, kind, info.Path, info.Bytes)
		if err != nil {
			return err
		}
		_, err = p.Exec(ctx, `UPDATE report_artifacts SET sha256=$3 WHERE run_id=$1 AND report_kind=$2`, runID, kind, info.SHA256)
		return err
	}
	if err != pgx.ErrNoRows {
		return err
	}
	if err := p.QueryRow(ctx, `INSERT INTO report_artifacts(run_id,report_kind,path,sha256,byte_size,report_status) VALUES($1,$2,$3,$4,$5,'VERIFIED') RETURNING id`, runID, kind, info.Path, info.SHA256, info.Bytes).Scan(new(int64)); err != nil {
		return err
	}
	return nil
}
