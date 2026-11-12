-- Drop foreign key constraint on audit.log_id so audit rows for deleted logs
-- are preserved when daily logs are removed.
ALTER TABLE audit DROP CONSTRAINT IF EXISTS audit_log_id_fkey;
