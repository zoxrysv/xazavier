package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"xazavier/internal/config"
	"xazavier/internal/domain"
	"xazavier/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	migrateTestDBOnce sync.Once
	migrateTestDBErr  error
)

func TestMain(m *testing.M) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "unable to resolve integration test source path")
		os.Exit(1)
	}
	backendDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	if err := os.Chdir(backendDir); err != nil {
		fmt.Fprintf(os.Stderr, "change to backend directory: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func requireTestDB(t *testing.T) *repository.PostgresStore {
	t.Helper()
	dbURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	ctx := context.Background()
	migrateTestDBOnce.Do(func() {
		conn, err := pgx.Connect(ctx, dbURL)
		if err != nil {
			migrateTestDBErr = err
			return
		}
		defer conn.Close(ctx)

		var logsTableExists bool
		if err := conn.QueryRow(ctx, `SELECT to_regclass('public.logs') IS NOT NULL`).Scan(&logsTableExists); err != nil {
			migrateTestDBErr = err
			return
		}
		if !logsTableExists {
			migrateTestDBErr = repository.AutoMigrate(ctx, dbURL)
		}
	})
	if migrateTestDBErr != nil {
		t.Fatalf("auto-migrate test database: %v", migrateTestDBErr)
	}

	store, err := repository.NewPostgresStore(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func newTestService(t *testing.T) (*Service, *repository.PostgresStore, domain.User, context.Context) {
	t.Helper()
	store := requireTestDB(t)
	cfg := config.Config{
		JWTSecret:       strings.Repeat("a", 32),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	svc := NewService(cfg, store)

	email := fmt.Sprintf("user-%s@example.com", strings.ReplaceAll(uuid.NewString(), "-", ""))
	userReq := domain.AuthRegisterRequest{
		Email:    email,
		Password: "Password123",
		FullName: "Test User",
		Timezone: "America/New_York",
	}
	if _, err := svc.Register(context.Background(), userReq, "127.0.0.1"); err != nil {
		t.Fatalf("register test user: %v", err)
	}
	user, ok := store.GetUserByEmail(email)
	if !ok {
		t.Fatalf("missing registered user: %s", email)
	}
	ctx := context.WithValue(context.Background(), "user", user)
	return svc, store, user, ctx
}

func TestIntegrationAuthFlow(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	email := fmt.Sprintf("auth-%s@example.com", strings.ReplaceAll(uuid.NewString(), "-", ""))
	password := "Password123"
	ctx := context.Background()

	bundle, err := svc.Register(ctx, domain.AuthRegisterRequest{
		Email:    email,
		Password: password,
		FullName: "Auth User",
		Timezone: "UTC",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if bundle.AccessToken == "" || bundle.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}

	login, err := svc.Login(ctx, domain.AuthLoginRequest{Email: email, Password: password}, "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.AccessToken == "" {
		t.Fatal("expected login access token")
	}

	refreshed, err := svc.Refresh(ctx, domain.AuthRefreshRequest{RefreshToken: bundle.RefreshToken}, "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Fatal("expected refreshed access token")
	}

	if err := svc.Logout(ctx, domain.AuthLogoutRequest{RefreshToken: bundle.RefreshToken}); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := svc.Refresh(ctx, domain.AuthRefreshRequest{RefreshToken: bundle.RefreshToken}, "127.0.0.1"); err == nil {
		t.Fatal("expected refresh to fail after logout")
	}
}

func TestIntegrationClusterLifecycle(t *testing.T) {
	svc, _, _, ctx := newTestService(t)

	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Work"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if cluster.Name != "Work" {
		t.Fatalf("unexpected cluster name: %s", cluster.Name)
	}

	updated, err := svc.UpdateCluster(ctx, cluster.ID, domain.ClusterUpdateRequest{Name: "Personal"})
	if err != nil {
		t.Fatalf("update cluster: %v", err)
	}
	if updated.Name != "Personal" {
		t.Fatalf("unexpected updated cluster name: %s", updated.Name)
	}

	if err := svc.DeleteCluster(ctx, cluster.ID); err != nil {
		t.Fatalf("delete cluster: %v", err)
	}

	clusters, err := svc.ListClusters(ctx)
	if err != nil {
		t.Fatalf("list clusters: %v", err)
	}
	if len(clusters) != 1 || clusters[0].DeletedAt == nil {
		t.Fatal("expected soft-deleted cluster to remain in list")
	}
}

func TestIntegrationActivityLifecycle(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Fitness"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Running",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if activity.Name != "Running" {
		t.Fatalf("unexpected activity name: %s", activity.Name)
	}

	updated, err := svc.UpdateActivity(ctx, activity.ID, domain.ActivityUpdateRequest{
		Name:         "Morning Run",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	if err != nil {
		t.Fatalf("update activity: %v", err)
	}
	if updated.Name != "Morning Run" {
		t.Fatalf("unexpected updated activity name: %s", updated.Name)
	}

	if err := svc.DeleteActivity(ctx, activity.ID); err != nil {
		t.Fatalf("delete activity: %v", err)
	}

	items, err := svc.ListActivities(ctx)
	if err != nil {
		t.Fatalf("list activities: %v", err)
	}
	if len(items) != 1 || items[0].DeletedAt == nil {
		t.Fatal("expected activity to be soft-deleted")
	}
}

func TestIntegrationCreateAndListLogs(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Habits"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Drinking water",
		TrackingType: "count",
		UnitName:     "glasses",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	start := time.Now().Add(-2 * time.Hour).UTC()
	logged, err := svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  activity.ID,
		StartTime:   start.Format(time.RFC3339),
		LoggedValue: 6.0,
		Notes:       "Hit my goal",
	})
	if err != nil {
		t.Fatalf("create log: %v", err)
	}
	if logged.LoggedValue == nil || *logged.LoggedValue != 6.0 {
		t.Fatal("expected stored count value")
	}

	logs, err := svc.ListLogs(ctx, domain.LogListFilter{
		FromDate: start.Format(time.RFC3339),
		ToDate:   time.Now().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != logged.ID {
		t.Fatal("expected created log to appear in user log list")
	}
}

func TestIntegrationCreateDurationLog(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Exercise"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Stretching",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	start := time.Now().Add(-30 * time.Minute).UTC()
	end := start.Add(15 * time.Minute)
	logged, err := svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID: activity.ID,
		StartTime:  start.Format(time.RFC3339),
		EndTime:    end.Format(time.RFC3339),
		Notes:      "felt good",
	})
	if err != nil {
		t.Fatalf("create duration log: %v", err)
	}
	if logged.EndTime == nil || logged.LoggedValue == nil || *logged.LoggedValue != 15.0 {
		t.Fatal("expected duration log to store end time and minutes")
	}
}

func TestIntegrationRejectFutureLog(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Future"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Planning",
		TrackingType: "count",
		UnitName:     "items",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  activity.ID,
		StartTime:   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		LoggedValue: 1.0,
	})
	if err == nil {
		t.Fatal("expected future log to fail")
	}
	if !strings.Contains(err.Error(), "FUTURE_LOG") && !strings.Contains(err.Error(), "future") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationRejectLateWindowLog(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Old"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Legacy",
		TrackingType: "count",
		UnitName:     "entries",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  activity.ID,
		StartTime:   time.Now().Add(-25 * time.Hour).Format(time.RFC3339),
		LoggedValue: 2.0,
	})
	if err == nil {
		t.Fatal("expected late log to fail")
	}
	if !strings.Contains(err.Error(), "LATE_WINDOW") && !strings.Contains(err.Error(), "24-hour") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationRejectNegativeCount(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Counts"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Books read",
		TrackingType: "count",
		UnitName:     "books",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  activity.ID,
		StartTime:   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		LoggedValue: -1.0,
	})
	if err == nil {
		t.Fatal("expected negative count to fail")
	}
	if !strings.Contains(err.Error(), "COUNT_INVALID") && !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationRejectDurationBeforeStart(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Timers"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Pomodoro",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	start := time.Now().Add(-30 * time.Minute).UTC()
	end := start.Add(-10 * time.Minute)
	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID: activity.ID,
		StartTime:  start.Format(time.RFC3339),
		EndTime:    end.Format(time.RFC3339),
	})
	if err == nil {
		t.Fatal("expected duration end before start to fail")
	}
	if !strings.Contains(err.Error(), "DURATION_INVALID") && !strings.Contains(err.Error(), "earlier") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationRejectLockedEdit(t *testing.T) {
	svc, store, user, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Locked"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Deep work",
		TrackingType: "count",
		UnitName:     "sessions",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	oldStart := time.Now().Add(-48 * time.Hour).UTC()
	log := domain.Log{
		ID:          uuid.NewString(),
		ActivityID:  activity.ID,
		UserID:      user.ID,
		LogDate:     formatLogDate(user.Timezone, oldStart),
		StartTime:   oldStart,
		EntryStatus: "LATE",
		Status:      "active",
		Notes:       "old",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Edited:      0,
		LoggedValue: float64Ptr(2),
	}
	if err := store.AddLog(log); err != nil {
		t.Fatalf("insert old log: %v", err)
	}

	_, err = svc.UpdateLog(ctx, log.ID, domain.LogUpdateRequest{Notes: "changed"})
	if err == nil {
		t.Fatal("expected locked log edit to fail")
	}
	if !strings.Contains(err.Error(), "LOCKED_LOG") && !strings.Contains(err.Error(), "24 hours") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationRejectLockedDelete(t *testing.T) {
	svc, store, user, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "DeleteLock"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Reading",
		TrackingType: "count",
		UnitName:     "pages",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}

	oldStart := time.Now().Add(-48 * time.Hour).UTC()
	log := domain.Log{
		ID:          uuid.NewString(),
		ActivityID:  activity.ID,
		UserID:      user.ID,
		LogDate:     formatLogDate(user.Timezone, oldStart),
		StartTime:   oldStart,
		EntryStatus: "LATE",
		Status:      "active",
		Notes:       "old",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		Edited:      0,
		LoggedValue: float64Ptr(3),
	}
	if err := store.AddLog(log); err != nil {
		t.Fatalf("insert old log: %v", err)
	}

	err = svc.DeleteLog(ctx, log.ID)
	if err == nil {
		t.Fatal("expected locked log delete to fail")
	}
	if !strings.Contains(err.Error(), "LOCKED_LOG") && !strings.Contains(err.Error(), "24 hours") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIntegrationLogDateUsesUserTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	stamp := time.Date(2025, time.March, 10, 0, 30, 0, 0, time.UTC)
	got := formatLogDate("America/Los_Angeles", stamp)
	want := stamp.In(loc).Format("2006-01-02")
	if got != want {
		t.Fatalf("formatLogDate mismatch: got %s want %s", got, want)
	}
	if got == stamp.UTC().Format("2006-01-02") {
		t.Fatalf("timezone conversion did not change the date for %s", stamp.String())
	}
}

func TestIntegrationResetUserData(t *testing.T) {
	svc, _, _, ctx := newTestService(t)
	cluster, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Reset"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	activity, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    cluster.ID,
		Name:         "Reading",
		TrackingType: "count",
		UnitName:     "pages",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  activity.ID,
		StartTime:   time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		LoggedValue: 12.0,
	})
	if err != nil {
		t.Fatalf("create log: %v", err)
	}

	if err := svc.Reset(ctx); err != nil {
		t.Fatalf("reset user data: %v", err)
	}
	if logs, err := svc.ListLogs(ctx, domain.LogListFilter{}); err != nil || len(logs) != 0 {
		t.Fatalf("logs should be cleared after reset, got len=%d err=%v", len(logs), err)
	}
}
