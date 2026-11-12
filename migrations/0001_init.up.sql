CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    user_agent TEXT,
    ip_address TEXT,
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS clusters (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    icon TEXT,
    color TEXT,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, name)
);

CREATE TABLE IF NOT EXISTS activities (
    id UUID PRIMARY KEY,
    cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    tracking_type TEXT NOT NULL CHECK (tracking_type IN ('count', 'duration')),
    unit_name TEXT,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (cluster_id, name)
);

CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY,
    activity_id UUID NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    log_date DATE NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    logged_value DOUBLE PRECISION,
    entry_status TEXT NOT NULL CHECK (entry_status IN ('NORMAL', 'LATE')),
    status TEXT NOT NULL DEFAULT 'active',
    notes TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    edited INTEGER NOT NULL DEFAULT 0 CHECK (edited >= 0),
    CHECK (logged_value IS NULL OR logged_value >= 0),
    CHECK (end_time IS NULL OR end_time >= start_time)
);

CREATE TABLE IF NOT EXISTS audit (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    log_id UUID NOT NULL,
    action_type TEXT NOT NULL CHECK (action_type IN ('UPDATE', 'DELETE')),
    old_start_time TIMESTAMPTZ,
    old_end_time TIMESTAMPTZ,
    old_logged_value DOUBLE PRECISION,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION enforce_log_integrity()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.start_time > NOW() + INTERVAL '5 minutes' THEN
        RAISE EXCEPTION 'FUTURE_LOG' USING ERRCODE = 'P0001';
    END IF;

    IF NEW.log_date != CAST(NEW.start_time AT TIME ZONE COALESCE((SELECT timezone FROM users WHERE id = NEW.user_id), 'UTC') AS DATE) THEN
        RAISE EXCEPTION 'LOG_DATE_MISMATCH' USING ERRCODE = 'P0001';
    END IF;

    IF NEW.entry_status NOT IN ('NORMAL', 'LATE') THEN
        RAISE EXCEPTION 'INVALID_ENTRY_STATUS' USING ERRCODE = '23514';
    END IF;

    IF NEW.logged_value IS NOT NULL AND NEW.logged_value < 0 THEN
        RAISE EXCEPTION 'NEGATIVE_LOGGED_VALUE' USING ERRCODE = '23514';
    END IF;

    IF NEW.end_time IS NOT NULL AND NEW.end_time < NEW.start_time THEN
        RAISE EXCEPTION 'END_BEFORE_START' USING ERRCODE = '23514';
    END IF;

    IF TG_OP = 'UPDATE' THEN
        IF EXISTS (
            SELECT 1 FROM logs l
            WHERE l.id = NEW.id
              AND l.user_id = NEW.user_id
              AND l.start_time < NOW() - INTERVAL '24 hours'
        ) THEN
            RAISE EXCEPTION 'LOCKED_LOG' USING ERRCODE = 'P0001';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS logs_integrity_trigger ON logs;
CREATE TRIGGER logs_integrity_trigger
BEFORE INSERT OR UPDATE ON logs
FOR EACH ROW
EXECUTE FUNCTION enforce_log_integrity();

CREATE OR REPLACE FUNCTION guard_delete_log()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.start_time < NOW() - INTERVAL '24 hours' THEN
        RAISE EXCEPTION 'LOCKED_LOG' USING ERRCODE = 'P0001';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS logs_delete_guard ON logs;
CREATE TRIGGER logs_delete_guard
BEFORE DELETE ON logs
FOR EACH ROW
EXECUTE FUNCTION guard_delete_log();
