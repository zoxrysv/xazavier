-- Performance indexes for production scale
CREATE INDEX IF NOT EXISTS idx_logs_user_start ON logs(user_id, start_time DESC);
CREATE INDEX IF NOT EXISTS idx_logs_user_date ON logs(user_id, log_date);
CREATE INDEX IF NOT EXISTS idx_audit_user_log ON audit(user_id, log_id);
CREATE INDEX IF NOT EXISTS idx_audit_user_changed ON audit(user_id, changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_refresh_hash ON sessions(refresh_token_hash);
CREATE INDEX IF NOT EXISTS idx_activities_cluster ON activities(cluster_id, user_id);

