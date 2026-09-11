-- Compute frozen configuration hashes with the same collision-safe canonical
-- encoding used by Go. Fixed children retain their two source config hashes.

CREATE OR REPLACE FUNCTION config_hash_part(v TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
    SELECT octet_length(COALESCE(v,''))::text || ':' || COALESCE(v,'')
$$;

CREATE OR REPLACE FUNCTION compute_config_sha256(p_version BIGINT) RETURNS TEXT
LANGUAGE sql STABLE AS $$
    WITH file_parts AS (
        SELECT string_agg(config_hash_part(sf.sha256), '' ORDER BY CASE cvf.source_kind WHEN 'payment_config' THEN 1 ELSE 2 END) AS value
        FROM config_version_files cvf JOIN source_files sf ON sf.id=cvf.source_file_id
        WHERE cvf.config_version_id=p_version
    ), rule_parts AS (
        SELECT string_agg(
            config_hash_part(mr.source) || config_hash_part(mr.origin_file) || config_hash_part(mr.origin_line::text) ||
            config_hash_part(mr.transaction_type) || config_hash_part(COALESCE(mr.description,'')) ||
            config_hash_part(COALESCE(mr.amount_field,'')) || config_hash_part(COALESCE(mr.amount_type,'')) ||
            config_hash_part(COALESCE(mr.amount_description,'')) || config_hash_part(mr.record_ref) ||
            config_hash_part(COALESCE(mr.positive_target,'')) || config_hash_part(COALESCE(mr.negative_target,'')),
            '' ORDER BY mr.source,mr.origin_file,mr.origin_line) AS value
        FROM mapping_rules mr WHERE mr.config_version_id=p_version
    )
    SELECT encode(digest(COALESCE(file_parts.value,'') || COALESCE(rule_parts.value,''),'sha256'),'hex')
    FROM file_parts,rule_parts
$$;

CREATE OR REPLACE FUNCTION set_canonical_config_hash() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state='DRAFT' AND NEW.state='FROZEN' THEN
        NEW.content_sha256 := compute_config_sha256(NEW.id);
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS config_versions_canonical_hash ON config_versions;
CREATE TRIGGER config_versions_canonical_hash BEFORE UPDATE ON config_versions
FOR EACH ROW EXECUTE FUNCTION set_canonical_config_hash();
