-- Reassert the final frozen-lineage policy on upgraded databases. Checking both
-- OLD and NEW prevents moving a child row out of or into a frozen version.

CREATE OR REPLACE FUNCTION reject_frozen_config_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE v_old_state TEXT; v_new_state TEXT;
BEGIN
    IF TG_OP <> 'INSERT' AND OLD.config_version_id IS NOT NULL THEN
        SELECT state INTO v_old_state FROM config_versions WHERE id=OLD.config_version_id;
        IF v_old_state='FROZEN' THEN
            RAISE EXCEPTION 'config version % is frozen and cannot be changed',OLD.config_version_id;
        END IF;
    END IF;
    IF TG_OP <> 'DELETE' AND NEW.config_version_id IS NOT NULL THEN
        SELECT state INTO v_new_state FROM config_versions WHERE id=NEW.config_version_id;
        IF v_new_state='FROZEN' THEN
            RAISE EXCEPTION 'config version % is frozen and cannot be changed',NEW.config_version_id;
        END IF;
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END;
$$;

CREATE OR REPLACE FUNCTION reject_frozen_version_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.state='FROZEN' THEN
        RAISE EXCEPTION 'config version % is frozen and cannot be changed',OLD.id;
    END IF;
    IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
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
BEFORE UPDATE OR DELETE ON config_versions
FOR EACH ROW EXECUTE FUNCTION reject_frozen_version_mutation();
