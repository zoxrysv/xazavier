-- ============================================================================
-- FILE: schema.sql
-- DESCRIPTION: Skema database Habit & Daily Activity Tracker (PostgreSQL)
-- ============================================================================

-- ==========================================
-- ⚙️ 1. SETUP EXTENSIONS & UTILITY FUNCTIONS
-- ==========================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "citext";

-- Otomatisasi kolom updated_at
CREATE OR REPLACE FUNCTION trigger_set_timestamp() 
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Validasi format IANA Timezone standar global
CREATE OR REPLACE FUNCTION validate_timezone() 
RETURNS TRIGGER AS $$
BEGIN
    PERFORM 1 FROM pg_timezone_names WHERE name = NEW.timezone;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Timezone % tidak valid! Gunakan format IANA (cth: Asia/Jakarta)', NEW.timezone;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ==========================================
-- 👥 2. TABEL USERS & SESSIONS
-- ==========================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email CITEXT UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    timezone VARCHAR(100) NOT NULL DEFAULT 'Asia/Jakarta',
    reset_password_token_hash VARCHAR(255),
    reset_token_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TRIGGER set_timestamp_users 
BEFORE UPDATE ON users 
FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

CREATE TRIGGER check_timezone_users 
BEFORE INSERT OR UPDATE ON users 
FOR EACH ROW EXECUTE FUNCTION validate_timezone();

CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) UNIQUE NOT NULL,
    user_agent VARCHAR(255),
    ip_address VARCHAR(50),
    is_revoked BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ==========================================
-- 🗂️ 3. MASTER DATA (CLUSTERS & ACTIVITIES)
-- ==========================================
CREATE TABLE clusters (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    icon VARCHAR(255),
    color VARCHAR(50),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (id, user_id)
);

CREATE TABLE activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cluster_id UUID NOT NULL,
    user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    tracking_type VARCHAR(50) NOT NULL CHECK (tracking_type IN ('duration', 'count')),
    unit_name VARCHAR(50),
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (cluster_id, user_id) REFERENCES clusters(id, user_id) ON DELETE RESTRICT,
    UNIQUE (id, user_id)
);

-- ==========================================
-- 📝 4. TRANSAKSI (DAILY LOGS)
-- ==========================================
CREATE TABLE daily_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    activity_id UUID NOT NULL,
    user_id UUID NOT NULL,
    log_date DATE NOT NULL,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ,
    logged_value DECIMAL(10, 2),
    entry_status VARCHAR(10) NOT NULL DEFAULT 'NORMAL' CHECK (entry_status IN ('NORMAL', 'LATE')),
    notes TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (activity_id, user_id) REFERENCES activities(id, user_id) ON DELETE RESTRICT,
    CHECK (end_time IS NULL OR end_time >= start_time),
    CHECK (logged_value IS NULL OR logged_value >= 0)
);

CREATE TRIGGER set_timestamp_logs 
BEFORE UPDATE ON daily_logs 
FOR EACH ROW EXECUTE FUNCTION trigger_set_timestamp();

-- Cegah mutasi tracking_type jika log sudah terbentuk
-- (Diletakkan setelah daily_logs dibuat agar aman dari dependency error)
CREATE OR REPLACE FUNCTION prevent_tracking_type_mutation() 
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.tracking_type IS DISTINCT FROM NEW.tracking_type THEN
        IF EXISTS (SELECT 1 FROM daily_logs WHERE activity_id = OLD.id) THEN
            RAISE EXCEPTION 'Mutasi Ditolak: Tracking Type tidak dapat diubah karena riwayat log sudah terbentuk.';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER check_activity_mutation 
BEFORE UPDATE ON activities 
FOR EACH ROW EXECUTE FUNCTION prevent_tracking_type_mutation();

-- Trigger Validasi Integritas: Soft Delete, Window 24 Jam, & Status Late
CREATE OR REPLACE FUNCTION validate_daily_log_integrity() 
RETURNS TRIGGER AS $$
DECLARE
    v_tracking_type VARCHAR;
    v_act_deleted TIMESTAMPTZ;
    v_cluster_deleted TIMESTAMPTZ;
    v_user_tz VARCHAR;
    v_expected_date DATE;
BEGIN
    -- 1. Validasi status soft delete Activity dan Cluster induk
    SELECT a.tracking_type, a.deleted_at, c.deleted_at
    INTO v_tracking_type, v_act_deleted, v_cluster_deleted
    FROM activities a
    JOIN clusters c ON a.cluster_id = c.id
    WHERE a.id = NEW.activity_id;
    
    IF v_act_deleted IS NOT NULL OR v_cluster_deleted IS NOT NULL THEN
        RAISE EXCEPTION 'Integritas Ditolak: Aktivitas atau Cluster terkait sedang diarsipkan.';
    END IF;

    -- 2. Validasi kesesuaian log_date dengan tanggal lokal start_time
    SELECT timezone INTO v_user_tz FROM users WHERE id = NEW.user_id;
    v_expected_date := (NEW.start_time AT TIME ZONE v_user_tz)::DATE;
    
    IF NEW.log_date != v_expected_date THEN
        RAISE EXCEPTION 'Integritas Ditolak: log_date (%) harus sama dengan tanggal lokal aktivitas dimulai (%) pada timezone %',
            NEW.log_date, v_expected_date, v_user_tz;
    END IF;

    -- 3. Aturan Window Waktu (Batas Masa Depan & Batas Backdate 24 Jam)
    IF NEW.start_time > (NOW() + INTERVAL '5 minutes') THEN
        RAISE EXCEPTION 'Integritas Ditolak: Aktivitas tidak dapat dicatat di masa depan.';
    END IF;

    IF TG_OP = 'INSERT' THEN
        -- Maksimal input ke belakang adalah 24 jam dari waktu server saat ini
        IF (NOW() - NEW.start_time) > INTERVAL '24 hours' THEN
            RAISE EXCEPTION 'Integritas Ditolak: Batas toleransi input data maksimal 24 jam dari waktu aktivitas dimulai.';
        END IF;

        -- Otomatisasi pelabelan NORMAL vs LATE
        -- LATE jika: diinput di beda hari lokal ATAU jeda pencatatan > 3 jam dari waktu mulai
        IF ((NOW() AT TIME ZONE v_user_tz)::DATE > v_expected_date) OR ((NOW() - NEW.start_time) > INTERVAL '3 hours') THEN
            NEW.entry_status := 'LATE';
        ELSE
            NEW.entry_status := 'NORMAL';
        END IF;

    ELSIF TG_OP = 'UPDATE' THEN
        -- Data yang umurnya sudah melebihi 24 jam terkunci permanen (Read-Only)
        IF (NOW() - OLD.start_time) > INTERVAL '24 hours' THEN
            RAISE EXCEPTION 'Integritas Ditolak: Log aktivitas yang telah melewati 24 jam telah terkunci permanen.';
        END IF;

        -- Pertahankan entry_status bawaan saat awal dicatat
        NEW.entry_status := OLD.entry_status;
    END IF;

    -- 4. Validasi tracking_type
    IF v_tracking_type = 'count' THEN
        IF NEW.end_time IS NOT NULL THEN
            RAISE EXCEPTION 'Aktivitas tipe Count tidak mendukung end_time.';
        END IF;
        IF NEW.logged_value IS NULL THEN
            RAISE EXCEPTION 'Aktivitas tipe Count wajib menyertakan logged_value.';
        END IF;
    ELSIF v_tracking_type = 'duration' THEN
        IF NEW.end_time IS NULL AND NEW.logged_value IS NOT NULL THEN
            RAISE EXCEPTION 'Aktivitas tipe Duration yang berjalan tidak boleh memiliki logged_value.';
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER enforce_log_integrity 
BEFORE INSERT OR UPDATE ON daily_logs 
FOR EACH ROW EXECUTE FUNCTION validate_daily_log_integrity();

-- ==========================================
-- 🕵️‍♂️ 5. AUDIT LOG (Terkait Lifecycle Akun)
-- ==========================================
CREATE TABLE daily_logs_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    log_id UUID NOT NULL,
    action_type VARCHAR(10) NOT NULL CHECK (action_type IN ('UPDATE', 'DELETE')),
    old_start_time TIMESTAMPTZ,
    old_end_time TIMESTAMPTZ,
    old_logged_value DECIMAL(10, 2),
    changed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION audit_daily_logs() 
RETURNS TRIGGER AS $$
BEGIN
    IF (TG_OP = 'DELETE') THEN
        INSERT INTO daily_logs_audit (user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value)
        VALUES (OLD.user_id, OLD.id, 'DELETE', OLD.start_time, OLD.end_time, OLD.logged_value);
        RETURN OLD;
    ELSIF (TG_OP = 'UPDATE') THEN
        IF (OLD.start_time IS DISTINCT FROM NEW.start_time OR OLD.end_time IS DISTINCT FROM NEW.end_time OR OLD.logged_value IS DISTINCT FROM NEW.logged_value) THEN
            INSERT INTO daily_logs_audit (user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value)
            VALUES (OLD.user_id, OLD.id, 'UPDATE', OLD.start_time, OLD.end_time, OLD.logged_value);
        END IF;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_audit_logs 
AFTER UPDATE OR DELETE ON daily_logs 
FOR EACH ROW EXECUTE FUNCTION audit_daily_logs();

-- ==========================================
-- 🚀 6. INDEX OPTIMIZATION
-- ==========================================
-- Query filters & dashboard
CREATE INDEX idx_logs_dashboard_date ON daily_logs(user_id, log_date);
CREATE INDEX idx_logs_discipline ON daily_logs(user_id, entry_status);
CREATE UNIQUE INDEX idx_unique_active_cluster ON clusters(user_id, LOWER(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_unique_active_activity ON activities(cluster_id, LOWER(name)) WHERE deleted_at IS NULL;

-- Foreign key lookups & cascade delete performance
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_activities_cluster_user ON activities(cluster_id, user_id);
CREATE INDEX idx_daily_logs_activity_user ON daily_logs(activity_id, user_id);
CREATE INDEX idx_daily_logs_audit_log_id ON daily_logs_audit(log_id);