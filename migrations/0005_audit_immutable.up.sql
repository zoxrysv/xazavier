-- Protect audit log from modifications and deletions (append-only)
CREATE OR REPLACE FUNCTION prevent_audit_tampering()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'AUDIT_IMMUTABLE: modifications or deletions in audit log are forbidden' USING ERRCODE = 'P0001';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_immutable_trigger ON audit;
CREATE TRIGGER audit_immutable_trigger
BEFORE UPDATE OR DELETE ON audit
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_tampering();

