CREATE OR REPLACE FUNCTION reject_frozen_config_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_version BIGINT;
    v_state TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_version := OLD.config_version_id;
    ELSE
        v_version := NEW.config_version_id;
    END IF;
    IF v_version IS NULL THEN
        IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
    END IF;
    SELECT state INTO v_state FROM config_versions WHERE id = v_version;
    IF v_state = 'FROZEN' THEN
        RAISE EXCEPTION 'config version % is frozen and cannot be changed', v_version;
    END IF;
    IF TG_OP = 'DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;

CREATE OR REPLACE FUNCTION reject_frozen_version_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.state = 'FROZEN' AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'config version % is frozen and cannot be changed', OLD.id;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS mapping_rules_frozen_guard ON mapping_rules;
CREATE TRIGGER mapping_rules_frozen_guard
    BEFORE INSERT OR UPDATE OR DELETE ON mapping_rules
    FOR EACH ROW EXECUTE FUNCTION reject_frozen_config_mutation();

DROP TRIGGER IF EXISTS config_version_files_frozen_guard ON config_version_files;
CREATE TRIGGER config_version_files_frozen_guard
    BEFORE INSERT OR UPDATE OR DELETE ON config_version_files
    FOR EACH ROW EXECUTE FUNCTION reject_frozen_config_mutation();

DROP TRIGGER IF EXISTS config_versions_frozen_guard ON config_versions;
CREATE TRIGGER config_versions_frozen_guard
    BEFORE UPDATE ON config_versions
    FOR EACH ROW EXECUTE FUNCTION reject_frozen_version_mutation();
