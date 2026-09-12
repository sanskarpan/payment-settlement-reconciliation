-- Historical configuration identities are append-only audit evidence.

CREATE OR REPLACE FUNCTION reject_config_hash_history_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'configuration hash history is immutable';
END;
$$;

DROP TRIGGER IF EXISTS config_hash_history_immutable_guard ON config_hash_history;
CREATE TRIGGER config_hash_history_immutable_guard
BEFORE UPDATE OR DELETE ON config_hash_history
FOR EACH ROW EXECUTE FUNCTION reject_config_hash_history_mutation();
