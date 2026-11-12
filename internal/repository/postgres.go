package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"xazavier/internal/apperr"
	"xazavier/internal/domain"
	"xazavier/migrations"
)

// Store is the repository contract used by the service layer.
type Store interface {
	CreateUser(user domain.User) error
	GetUserByEmail(email string) (domain.User, bool)
	GetUserByID(id string) (domain.User, bool)
	ListUsers() []domain.User
	AddSession(session domain.Session) error
	GetSessionByID(id string) (domain.Session, bool)
	GetSessionByRefreshHash(hash string) (domain.Session, bool)
	RevokeSessionByRefreshTokenHash(hash string)
	ListSessionsForUser(userID string) []domain.Session
	ListClusters(userID string) []domain.Cluster
	GetClusterByID(userID, id string) (domain.Cluster, bool)
	AddCluster(cluster domain.Cluster) error
	UpdateCluster(cluster domain.Cluster) error
	DeleteCluster(userID, id string) error
	ListActivities(userID string) []domain.Activity
	GetActivityByID(userID, id string) (domain.Activity, bool)
	AddActivity(activity domain.Activity) error
	UpdateActivity(activity domain.Activity) error
	DeleteActivity(userID, id string) error
	ListLogs(userID string, filter domain.LogListFilter) []domain.Log
	GetLogByID(userID, id string) (domain.Log, bool)
	AddLog(log domain.Log) error
	UpdateLog(log domain.Log) error
	DeleteLog(userID, id string) error
	UpdateLogWithAudit(log domain.Log, audit *domain.Audit) error
	DeleteLogWithAudit(userID, id string, audit domain.Audit) error
	ListAudit(userID string, filter domain.AuditListFilter) []domain.Audit
	AddAudit(audit domain.Audit) error
	ListAuditForLog(userID, logID string) []domain.Audit
	ResetUserData(userID string) error
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func EnsureDatabaseExists(ctx context.Context, databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return err
	}
	targetDB := strings.TrimPrefix(u.Path, "/")
	if targetDB == "" || targetDB == "postgres" || targetDB == "template1" {
		return nil
	}

	adminURL := *u
	adminURL.Path = "/postgres"

	conn, err := pgx.Connect(ctx, adminURL.String())
	if err != nil {
		return nil
	}
	defer conn.Close(ctx)

	var exists bool
	err = conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", targetDB).Scan(&exists)
	if err != nil {
		return nil
	}
	if !exists {
		query := fmt.Sprintf(`CREATE DATABASE "%s"`, strings.ReplaceAll(targetDB, `"`, `""`))
		if _, err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("create database %s: %w", targetDB, err)
		}
	}
	return nil
}

func AutoMigrate(ctx context.Context, databaseURL string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}

	_ = EnsureDatabaseExists(ctx, databaseURL)

	store, err := NewPostgresStore(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	// Ensure migration tracking table exists.
	_, err = store.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// Load already-applied versions.
	rows, err := store.pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("query schema_migrations: %w", err)
	}
	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	for _, name := range files {
		if applied[name] {
			continue // already applied, skip
		}
		content, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := store.pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("migrate %s: %w", name, err)
		}
		if _, err := store.pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *PostgresStore) CreateUser(user domain.User) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO users (id, email, password_hash, full_name, timezone, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, user.ID, strings.ToLower(user.Email), user.PasswordHash, user.FullName, user.Timezone, user.CreatedAt.UTC())
	return mapDBError(err, "Registration rejected", "This email address is already registered.")
}

func (s *PostgresStore) GetUserByEmail(email string) (domain.User, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,email,password_hash,full_name,timezone,created_at FROM users WHERE email = $1`, strings.ToLower(strings.TrimSpace(email)))
	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Timezone, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, false
		}
		return domain.User{}, false
	}
	return user, true
}

func (s *PostgresStore) GetUserByID(id string) (domain.User, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,email,password_hash,full_name,timezone,created_at FROM users WHERE id = $1`, id)
	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Timezone, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, false
		}
		return domain.User{}, false
	}
	return user, true
}

func (s *PostgresStore) ListUsers() []domain.User {
	rows, err := s.pool.Query(context.Background(), `SELECT id,email,password_hash,full_name,timezone,created_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FullName, &user.Timezone, &user.CreatedAt); err != nil {
			continue
		}
		out = append(out, user)
	}
	return out
}

func (s *PostgresStore) AddSession(session domain.Session) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO sessions (id, user_id, refresh_token_hash, user_agent, ip_address, is_revoked, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, session.ID, session.UserID, session.RefreshTokenHash, session.UserAgent, session.IPAddress, session.IsRevoked, session.ExpiresAt.UTC(), session.CreatedAt.UTC())
	return mapDBError(err, "Session rejected", "Unable to create session.")
}

func (s *PostgresStore) GetSessionByID(id string) (domain.Session, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,user_id,refresh_token_hash,user_agent,ip_address,is_revoked,expires_at,created_at FROM sessions WHERE id = $1`, id)
	var session domain.Session
	if err := row.Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &session.UserAgent, &session.IPAddress, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, false
		}
		return domain.Session{}, false
	}
	return session, true
}

func (s *PostgresStore) GetSessionByRefreshHash(hash string) (domain.Session, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,user_id,refresh_token_hash,user_agent,ip_address,is_revoked,expires_at,created_at FROM sessions WHERE refresh_token_hash = $1 ORDER BY created_at DESC LIMIT 1`, hash)
	var session domain.Session
	if err := row.Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &session.UserAgent, &session.IPAddress, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Session{}, false
		}
		return domain.Session{}, false
	}
	return session, true
}

func (s *PostgresStore) RevokeSessionByRefreshTokenHash(hash string) {
	_, _ = s.pool.Exec(context.Background(), `UPDATE sessions SET is_revoked = true WHERE refresh_token_hash = $1`, hash)
}

func (s *PostgresStore) ListSessionsForUser(userID string) []domain.Session {
	rows, err := s.pool.Query(context.Background(), `SELECT id,user_id,refresh_token_hash,user_agent,ip_address,is_revoked,expires_at,created_at FROM sessions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Session, 0)
	for rows.Next() {
		var session domain.Session
		if err := rows.Scan(&session.ID, &session.UserID, &session.RefreshTokenHash, &session.UserAgent, &session.IPAddress, &session.IsRevoked, &session.ExpiresAt, &session.CreatedAt); err != nil {
			continue
		}
		out = append(out, session)
	}
	return out
}

func (s *PostgresStore) ListClusters(userID string) []domain.Cluster {
	rows, err := s.pool.Query(context.Background(), `SELECT id,user_id,name,icon,color,deleted_at,created_at FROM clusters WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Cluster, 0)
	for rows.Next() {
		var cluster domain.Cluster
		var deletedAt *time.Time
		if err := rows.Scan(&cluster.ID, &cluster.UserID, &cluster.Name, &cluster.Icon, &cluster.Color, &deletedAt, &cluster.CreatedAt); err != nil {
			continue
		}
		cluster.DeletedAt = deletedAt
		out = append(out, cluster)
	}
	return out
}

func (s *PostgresStore) GetClusterByID(userID, id string) (domain.Cluster, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,user_id,name,icon,color,deleted_at,created_at FROM clusters WHERE user_id = $1 AND id = $2`, userID, id)
	var cluster domain.Cluster
	var deletedAt *time.Time
	if err := row.Scan(&cluster.ID, &cluster.UserID, &cluster.Name, &cluster.Icon, &cluster.Color, &deletedAt, &cluster.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Cluster{}, false
		}
		return domain.Cluster{}, false
	}
	cluster.DeletedAt = deletedAt
	return cluster, true
}

func (s *PostgresStore) AddCluster(cluster domain.Cluster) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO clusters (id, user_id, name, icon, color, deleted_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, cluster.ID, cluster.UserID, cluster.Name, cluster.Icon, cluster.Color, cluster.DeletedAt, cluster.CreatedAt.UTC())
	return mapDBError(err, "Cluster not created", "An active cluster with this name already exists.")
}

func (s *PostgresStore) UpdateCluster(cluster domain.Cluster) error {
	_, err := s.pool.Exec(context.Background(), `
		UPDATE clusters
		SET name = $1, icon = $2, color = $3, deleted_at = $4
		WHERE id = $5 AND user_id = $6
	`, cluster.Name, cluster.Icon, cluster.Color, cluster.DeletedAt, cluster.ID, cluster.UserID)
	return mapDBError(err, "Cluster update rejected", "The cluster could not be updated.")
}

func (s *PostgresStore) DeleteCluster(userID, id string) error {
	_, err := s.pool.Exec(context.Background(), `UPDATE clusters SET deleted_at = NOW() WHERE user_id = $1 AND id = $2`, userID, id)
	return mapDBError(err, "Cluster not deleted", "The selected cluster could not be deleted.")
}

func (s *PostgresStore) ListActivities(userID string) []domain.Activity {
	rows, err := s.pool.Query(context.Background(), `SELECT id,cluster_id,user_id,name,tracking_type,unit_name,deleted_at,created_at FROM activities WHERE user_id = $1 ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Activity, 0)
	for rows.Next() {
		var activity domain.Activity
		var deletedAt *time.Time
		if err := rows.Scan(&activity.ID, &activity.ClusterID, &activity.UserID, &activity.Name, &activity.TrackingType, &activity.UnitName, &deletedAt, &activity.CreatedAt); err != nil {
			continue
		}
		activity.DeletedAt = deletedAt
		out = append(out, activity)
	}
	return out
}

func (s *PostgresStore) GetActivityByID(userID, id string) (domain.Activity, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,cluster_id,user_id,name,tracking_type,unit_name,deleted_at,created_at FROM activities WHERE user_id = $1 AND id = $2`, userID, id)
	var activity domain.Activity
	var deletedAt *time.Time
	if err := row.Scan(&activity.ID, &activity.ClusterID, &activity.UserID, &activity.Name, &activity.TrackingType, &activity.UnitName, &deletedAt, &activity.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Activity{}, false
		}
		return domain.Activity{}, false
	}
	activity.DeletedAt = deletedAt
	return activity, true
}

func (s *PostgresStore) AddActivity(activity domain.Activity) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO activities (id, cluster_id, user_id, name, tracking_type, unit_name, deleted_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, activity.ID, activity.ClusterID, activity.UserID, activity.Name, activity.TrackingType, activity.UnitName, activity.DeletedAt, activity.CreatedAt.UTC())
	return mapDBError(err, "Activity not created", "An active activity with this name already exists in this cluster.")
}

func (s *PostgresStore) UpdateActivity(activity domain.Activity) error {
	_, err := s.pool.Exec(context.Background(), `
		UPDATE activities SET cluster_id=$1, name=$2, tracking_type=$3, unit_name=$4, deleted_at=$5 WHERE id=$6 AND user_id=$7
	`, activity.ClusterID, activity.Name, activity.TrackingType, activity.UnitName, activity.DeletedAt, activity.ID, activity.UserID)
	return mapDBError(err, "Activity update rejected", "The activity could not be updated.")
}

func (s *PostgresStore) DeleteActivity(userID, id string) error {
	_, err := s.pool.Exec(context.Background(), `UPDATE activities SET deleted_at = NOW() WHERE user_id = $1 AND id = $2`, userID, id)
	return mapDBError(err, "Activity not deleted", "The selected activity could not be deleted.")
}

func (s *PostgresStore) ListLogs(userID string, filter domain.LogListFilter) []domain.Log {
	query := `SELECT id,activity_id,user_id,log_date::text,start_time,end_time,logged_value,entry_status,status,notes,created_at,updated_at,edited FROM logs WHERE user_id = $1`
	args := []any{userID}
	argIdx := 2

	if filter.FromDate != "" {
		query += fmt.Sprintf(` AND log_date >= $%d`, argIdx)
		args = append(args, filter.FromDate)
		argIdx++
	}
	if filter.ToDate != "" {
		query += fmt.Sprintf(` AND log_date <= $%d`, argIdx)
		args = append(args, filter.ToDate)
		argIdx++
	}

	query += ` ORDER BY start_time ASC`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
		if filter.Offset > 0 {
			query += fmt.Sprintf(` OFFSET $%d`, argIdx)
			args = append(args, filter.Offset)
			argIdx++
		}
	}

	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Log, 0)
	for rows.Next() {
		var log domain.Log
		var endTime *time.Time
		var loggedValue *float64
		if err := rows.Scan(&log.ID, &log.ActivityID, &log.UserID, &log.LogDate, &log.StartTime, &endTime, &loggedValue, &log.EntryStatus, &log.Status, &log.Notes, &log.CreatedAt, &log.UpdatedAt, &log.Edited); err != nil {
			continue
		}
		log.EndTime = endTime
		log.LoggedValue = loggedValue
		out = append(out, log)
	}
	return out
}

func (s *PostgresStore) GetLogByID(userID, id string) (domain.Log, bool) {
	row := s.pool.QueryRow(context.Background(), `SELECT id,activity_id,user_id,log_date::text,start_time,end_time,logged_value,entry_status,status,notes,created_at,updated_at,edited FROM logs WHERE user_id = $1 AND id = $2`, userID, id)
	var log domain.Log
	var endTime *time.Time
	var loggedValue *float64
	if err := row.Scan(&log.ID, &log.ActivityID, &log.UserID, &log.LogDate, &log.StartTime, &endTime, &loggedValue, &log.EntryStatus, &log.Status, &log.Notes, &log.CreatedAt, &log.UpdatedAt, &log.Edited); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Log{}, false
		}
		return domain.Log{}, false
	}
	log.EndTime = endTime
	log.LoggedValue = loggedValue
	return log, true
}

func (s *PostgresStore) AddLog(log domain.Log) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO logs (id, activity_id, user_id, log_date, start_time, end_time, logged_value, entry_status, status, notes, created_at, updated_at, edited)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, log.ID, log.ActivityID, log.UserID, log.LogDate, log.StartTime.UTC(), log.EndTime, log.LoggedValue, log.EntryStatus, log.Status, log.Notes, log.CreatedAt.UTC(), log.UpdatedAt.UTC(), log.Edited)
	return mapDBError(err, "Recording rejected", "This activity could not be recorded.")
}

func (s *PostgresStore) UpdateLog(log domain.Log) error {
	_, err := s.pool.Exec(context.Background(), `
		UPDATE logs
		SET end_time = $1, logged_value = $2, notes = $3, start_time = $4, log_date = $5, entry_status = $6, status = $7, updated_at = $8, edited = $9
		WHERE id = $10 AND user_id = $11
	`, log.EndTime, log.LoggedValue, log.Notes, log.StartTime.UTC(), log.LogDate, log.EntryStatus, log.Status, log.UpdatedAt.UTC(), log.Edited, log.ID, log.UserID)
	return mapDBError(err, "Edit rejected", "This record could not be updated.")
}

func (s *PostgresStore) DeleteLog(userID, id string) error {
	_, err := s.pool.Exec(context.Background(), `DELETE FROM logs WHERE user_id = $1 AND id = $2`, userID, id)
	return mapDBError(err, "Delete rejected", "This record could not be deleted.")
}

func (s *PostgresStore) UpdateLogWithAudit(log domain.Log, audit *domain.Audit) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if audit != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO audit (id, user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value, changed_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		`, audit.ID, audit.UserID, audit.LogID, audit.ActionType, audit.OldStartTime, audit.OldEndTime, audit.OldLoggedValue, audit.ChangedAt.UTC())
		if err != nil {
			return mapDBError(err, "Audit rejected", "This change could not be recorded.")
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE logs
		SET end_time = $1, logged_value = $2, notes = $3, start_time = $4, log_date = $5, entry_status = $6, status = $7, updated_at = $8, edited = $9
		WHERE id = $10 AND user_id = $11
	`, log.EndTime, log.LoggedValue, log.Notes, log.StartTime.UTC(), log.LogDate, log.EntryStatus, log.Status, log.UpdatedAt.UTC(), log.Edited, log.ID, log.UserID)
	if err != nil {
		return mapDBError(err, "Edit rejected", "This record could not be updated.")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) DeleteLogWithAudit(userID, id string, audit domain.Audit) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO audit (id, user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value, changed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, audit.ID, audit.UserID, audit.LogID, audit.ActionType, audit.OldStartTime, audit.OldEndTime, audit.OldLoggedValue, audit.ChangedAt.UTC())
	if err != nil {
		return mapDBError(err, "Audit rejected", "This change could not be recorded.")
	}

	_, err = tx.Exec(ctx, `DELETE FROM logs WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return mapDBError(err, "Delete rejected", "This record could not be deleted.")
	}

	return tx.Commit(ctx)
}

func (s *PostgresStore) ListAudit(userID string, filter domain.AuditListFilter) []domain.Audit {
	query := `SELECT id,user_id,log_id,action_type,old_start_time,old_end_time,old_logged_value,changed_at FROM audit WHERE user_id = $1 ORDER BY changed_at ASC`
	args := []any{userID}
	argIdx := 2

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, filter.Limit)
		argIdx++
		if filter.Offset > 0 {
			query += fmt.Sprintf(` OFFSET $%d`, argIdx)
			args = append(args, filter.Offset)
			argIdx++
		}
	}

	rows, err := s.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Audit, 0)
	for rows.Next() {
		var audit domain.Audit
		var oldStart *time.Time
		var oldEnd *time.Time
		var oldLoggedValue *float64
		if err := rows.Scan(&audit.ID, &audit.UserID, &audit.LogID, &audit.ActionType, &oldStart, &oldEnd, &oldLoggedValue, &audit.ChangedAt); err != nil {
			continue
		}
		audit.OldStartTime = oldStart
		audit.OldEndTime = oldEnd
		audit.OldLoggedValue = oldLoggedValue
		out = append(out, audit)
	}
	return out
}

func (s *PostgresStore) AddAudit(audit domain.Audit) error {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO audit (id, user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value, changed_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, audit.ID, audit.UserID, audit.LogID, audit.ActionType, audit.OldStartTime, audit.OldEndTime, audit.OldLoggedValue, audit.ChangedAt.UTC())
	return mapDBError(err, "Audit rejected", "This change could not be recorded.")
}

func (s *PostgresStore) ListAuditForLog(userID, logID string) []domain.Audit {
	rows, err := s.pool.Query(context.Background(), `SELECT id,user_id,log_id,action_type,old_start_time,old_end_time,old_logged_value,changed_at FROM audit WHERE user_id = $1 AND log_id = $2 ORDER BY changed_at ASC`, userID, logID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]domain.Audit, 0)
	for rows.Next() {
		var audit domain.Audit
		var oldStart *time.Time
		var oldEnd *time.Time
		var oldLoggedValue *float64
		if err := rows.Scan(&audit.ID, &audit.UserID, &audit.LogID, &audit.ActionType, &oldStart, &oldEnd, &oldLoggedValue, &audit.ChangedAt); err != nil {
			continue
		}
		audit.OldStartTime = oldStart
		audit.OldEndTime = oldEnd
		audit.OldLoggedValue = oldLoggedValue
		out = append(out, audit)
	}
	return out
}

func (s *PostgresStore) ResetUserData(userID string) error {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// In PostgreSQL, reset must bypass 24h delete guard trigger or disable trigger for the transaction
	// if dropping all logs. Alternatively, set session_replication_role or drop logs within tx.
	// Since logs has a BEFORE DELETE trigger "logs_delete_guard", standard DELETE will fail on logs older than 24h.
	// We disable the trigger locally in this transaction.
	_, _ = tx.Exec(ctx, `ALTER TABLE logs DISABLE TRIGGER logs_delete_guard`)

	_, err = tx.Exec(ctx, `DELETE FROM audit WHERE user_id = $1`, userID)
	if err != nil {
		return mapDBError(err, "Reset rejected", "The data could not be reset.")
	}
	_, err = tx.Exec(ctx, `DELETE FROM logs WHERE user_id = $1`, userID)
	if err != nil {
		return mapDBError(err, "Reset rejected", "The data could not be reset.")
	}
	_, _ = tx.Exec(ctx, `ALTER TABLE logs ENABLE TRIGGER logs_delete_guard`)

	_, err = tx.Exec(ctx, `DELETE FROM activities WHERE user_id = $1`, userID)
	if err != nil {
		return mapDBError(err, "Reset rejected", "The data could not be reset.")
	}
	_, err = tx.Exec(ctx, `DELETE FROM clusters WHERE user_id = $1`, userID)
	if err != nil {
		return mapDBError(err, "Reset rejected", "The data could not be reset.")
	}
	_, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return mapDBError(err, "Reset rejected", "The data could not be reset.")
	}
	return tx.Commit(ctx)
}

func mapDBError(err error, fallbackTitle, fallbackMessage string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperr.Conflict(fallbackTitle, fallbackMessage)
		case "23503":
			return apperr.UnprocessableEntity("Reference rejected", "This record references a missing or archived object.", "RELATION_ERROR")
		case "23514":
			return apperr.UnprocessableEntity(fallbackTitle, fallbackMessage, "CHECK_VIOLATION")
		case "P0001":
			return apperr.UnprocessableEntity(fallbackTitle, fallbackMessage, "BUSINESS_RULE")
		}
	}
	return apperr.UnprocessableEntity(fallbackTitle, fallbackMessage, "DB_ERROR")
}
