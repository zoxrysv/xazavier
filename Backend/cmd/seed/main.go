package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
	"xazavier/internal/config"
	"xazavier/internal/repository"
)

func main() {
	cfg := config.Load()
	dsn := cfg.DatabaseURL
	if dsn == "" || dsn == "memory" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		panic("DATABASE_URL is required")
	}

	ctx := context.Background()

	// 0. Automatically create xazavier database & run migrations if needed
	_ = repository.EnsureDatabaseExists(ctx, dsn)
	if err := repository.AutoMigrate(ctx, dsn); err != nil {
		fmt.Printf("note: migration check: %v\n", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		panic(err)
	}

	pwHash := hashPassword("demo1234")
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	userID := "00000000-0000-0000-0000-000000000001"

	// 1. Seed demo user
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, timezone, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
		userID, "demo@xazavier.app", pwHash, "Demo User", "Asia/Jakarta", now.Add(-30*24*time.Hour))
	if err != nil {
		panic(fmt.Errorf("seed user: %w", err))
	}

	// 2. Seed clusters
	cFoundation := uuid.NewString()
	cExecution := uuid.NewString()
	cNonEssential := uuid.NewString()

	type clusterSeed struct {
		id   string
		name string
	}
	clusters := []clusterSeed{
		{cFoundation, "Foundation"},
		{cExecution, "Execution"},
		{cNonEssential, "Non-essential"},
	}

	clusterIDs := make(map[string]string)
	for _, c := range clusters {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO clusters (id, user_id, name, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, name) DO UPDATE SET deleted_at = NULL
			RETURNING id`, c.id, userID, c.name, now.Add(-30*24*time.Hour)).Scan(&id)
		if err != nil {
			panic(fmt.Errorf("seed cluster %s: %w", c.name, err))
		}
		clusterIDs[c.name] = id
	}

	// 3. Seed activities
	type activitySeed struct {
		cluster string
		name    string
		track   string
		unit    string
	}
	actDefs := []activitySeed{
		{"Foundation", "Sleep", "duration", "minutes"},
		{"Foundation", "Meal", "count", "meals"},
		{"Foundation", "Prayer", "count", "prayers"},
		{"Execution", "Coding", "duration", "minutes"},
		{"Execution", "Studying", "duration", "minutes"},
		{"Non-essential", "Smoking", "count", "cigarettes"},
		{"Non-essential", "Gaming", "duration", "minutes"},
	}

	actIDs := make(map[string]string)
	for _, a := range actDefs {
		var id string
		cID := clusterIDs[a.cluster]
		err := pool.QueryRow(ctx, `
			INSERT INTO activities (id, cluster_id, user_id, name, tracking_type, unit_name, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (cluster_id, name) DO UPDATE SET deleted_at = NULL
			RETURNING id`, uuid.NewString(), cID, userID, a.name, a.track, a.unit, now.Add(-30*24*time.Hour)).Scan(&id)
		if err != nil {
			panic(fmt.Errorf("seed activity %s: %w", a.name, err))
		}
		actIDs[a.name] = id
	}

	// 4. Seed logs across 7 days
	var codingLogID string
	var codingStartTime time.Time
	var codingEndTime time.Time
	var codingLoggedVal float64
	var codingCreatedAt time.Time

	for i := 0; i < 7; i++ {
		dayOffset := time.Duration(i) * 24 * time.Hour
		baseDay := startOfDay.Add(-dayOffset)
		k := float64((i * 3) % 5)

		insertLog := func(actName string, startHour, endHour float64, countVal float64, lagMin float64) (string, time.Time, *time.Time, *float64, time.Time) {
			actID := actIDs[actName]
			st := baseDay.Add(time.Duration(startHour * float64(time.Hour)))
			if st.After(now) {
				return "", time.Time{}, nil, nil, time.Time{}
			}
			var et *time.Time
			var val *float64
			if endHour > 0 {
				e := baseDay.Add(time.Duration(endHour * float64(time.Hour)))
				if e.After(now) {
					e = now
				}
				et = &e
				durM := float64(int(e.Sub(st).Minutes()))
				val = &durM
			} else {
				val = &countVal
			}

			cr := st.Add(time.Duration(lagMin) * time.Minute)
			if et != nil {
				cr = et.Add(time.Duration(lagMin) * time.Minute)
			}
			if cr.After(now) {
				cr = now
			}

			entryStatus := "NORMAL"
			if cr.Sub(st) > 3*time.Hour || cr.Format("2006-01-02") != st.Format("2006-01-02") {
				entryStatus = "LATE"
			}

			logID := uuid.NewString()
			_, err := pool.Exec(ctx, `
				INSERT INTO logs (id, activity_id, user_id, log_date, start_time, end_time, logged_value, entry_status, status, notes, created_at, updated_at, edited)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
				logID, actID, userID, st.Format("2006-01-02"), st, et, val, entryStatus, "active", "", cr, cr, 0)
			if err != nil {
				// Non-fatal if already seeded
				return "", time.Time{}, nil, nil, time.Time{}
			}
			return logID, st, et, val, cr
		}

		insertLog("Sleep", 0.2, 6.4+k/10, 0, 5)
		insertLog("Meal", 8.9, 0, 1, 3)
		clID, cSt, cEt, cVal, cCr := insertLog("Coding", 9.5+k/20, 10.3+k/10, 0, func() float64 {
			if i == 2 {
				return 500
			}
			return 4
		}())
		if i == 0 && clID != "" && cEt != nil && cVal != nil {
			codingLogID = clID
			codingStartTime = cSt
			codingEndTime = *cEt
			codingLoggedVal = *cVal
			codingCreatedAt = cCr
		}
		insertLog("Meal", 13.2, 0, 1, 6)
		insertLog("Prayer", 12.1, 0, 1, 8)
		insertLog("Studying", 14.5, 15.5+k/5, 0, 7)
		insertLog("Smoking", 16.3, 0, 1+float64(int(k)%3), 2)
		insertLog("Gaming", 19.7, 20.7+k*0.5, 0, 6)
		insertLog("Meal", 20.4, 0, 1, 9)
	}

	// 5. Seed audit entry for today's Coding log
	if codingLogID != "" {
		oldEnd := codingEndTime.Add(-5 * time.Minute)
		oldVal := codingLoggedVal - 5
		auditTime := codingCreatedAt.Add(90 * time.Minute)
		if auditTime.After(now) {
			auditTime = now
		}

		_, _ = pool.Exec(ctx, `
			INSERT INTO audit (id, user_id, log_id, action_type, old_start_time, old_end_time, old_logged_value, changed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			uuid.NewString(), userID, codingLogID, "UPDATE", codingStartTime, oldEnd, oldVal, auditTime)
		_, _ = pool.Exec(ctx, `UPDATE logs SET updated_at = $1, edited = 1 WHERE id = $2`, auditTime, codingLogID)
	}

	fmt.Println("Successfully seeded full demo dataset:")
	fmt.Println("- User: demo@xazavier.app (password: demo1234)")
	fmt.Println("- 3 Clusters: Foundation, Execution, Non-essential")
	fmt.Println("- 7 Activities: Sleep, Meal, Prayer, Coding, Studying, Smoking, Gaming")
	fmt.Println("- 7 Days of realistic daily logs & audit history")
}

func hashPassword(password string) string {
	salt := []byte("xazavier-seed-salt")
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%x$%x", salt, key)
}
