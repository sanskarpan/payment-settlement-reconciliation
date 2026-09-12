-- Existing frozen configurations created before migration 007 used the legacy
-- delimiter-concatenated hash. Preserve that identity as audit history, then
-- move the active content hash to the collision-safe canonical algorithm.

CREATE TABLE IF NOT EXISTS config_hash_history (
    config_version_id BIGINT NOT NULL REFERENCES config_versions(id) ON DELETE RESTRICT,
    hash_algorithm TEXT NOT NULL,
    content_sha256 TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (config_version_id, hash_algorithm),
    UNIQUE (hash_algorithm, content_sha256)
);

INSERT INTO config_hash_history(config_version_id,hash_algorithm,content_sha256)
SELECT id,'legacy-delimited-v1',content_sha256
FROM config_versions
WHERE state='FROZEN'
  AND content_sha256 IS DISTINCT FROM compute_config_sha256(id)
ON CONFLICT (config_version_id,hash_algorithm) DO NOTHING;

-- The frozen guard must be bypassed only inside this checksummed migration.
-- The transaction restores the trigger automatically if any statement fails.
ALTER TABLE config_versions DISABLE TRIGGER config_versions_frozen_guard;

UPDATE config_versions
SET content_sha256=compute_config_sha256(id)
WHERE state='FROZEN'
  AND content_sha256 IS DISTINCT FROM compute_config_sha256(id);

ALTER TABLE config_versions ENABLE TRIGGER config_versions_frozen_guard;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM config_versions
        WHERE state='FROZEN'
          AND content_sha256 IS DISTINCT FROM compute_config_sha256(id)
    ) THEN
        RAISE EXCEPTION 'canonical configuration hash backfill is incomplete';
    END IF;
END;
$$;
