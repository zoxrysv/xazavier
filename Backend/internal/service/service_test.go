package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"xazavier/internal/config"
	"xazavier/internal/domain"
	"xazavier/internal/repository"
)

func newTestMemService() (*Service, *repository.MemStore, domain.User, context.Context) {
	store := repository.NewMemoryStore()
	cfg := config.Config{
		JWTSecret:       strings.Repeat("s", 32),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	svc := NewService(cfg, store)

	userReq := domain.AuthRegisterRequest{
		Email:    "tester@example.com",
		Password: "Password123!",
		FullName: "Test User",
		Timezone: "Asia/Jakarta",
	}
	_, err := svc.Register(context.Background(), userReq, "127.0.0.1")
	if err != nil {
		panic(err)
	}
	user, _ := store.GetUserByEmail("tester@example.com")
	ctx := context.WithValue(context.Background(), "user", user)
	return svc, store, user, ctx
}

func TestAuthRegisterAndLogin(t *testing.T) {
	svc, _, _, _ := newTestMemService()
	ctx := context.Background()

	// Duplicate register should fail
	_, err := svc.Register(ctx, domain.AuthRegisterRequest{
		Email:    "tester@example.com",
		Password: "Password123!",
		FullName: "Test User",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected duplicate registration to fail")
	}

	// Login with correct password
	loginBundle, err := svc.Login(ctx, domain.AuthLoginRequest{
		Email:    "tester@example.com",
		Password: "Password123!",
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("expected login to succeed, got: %v", err)
	}
	if loginBundle.AccessToken == "" || loginBundle.RefreshToken == "" {
		t.Fatal("expected tokens in login bundle")
	}

	// Login with incorrect password
	_, err = svc.Login(ctx, domain.AuthLoginRequest{
		Email:    "tester@example.com",
		Password: "WrongPassword",
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected login with wrong password to fail")
	}

	// Token refresh
	refreshed, err := svc.Refresh(ctx, domain.AuthRefreshRequest{
		RefreshToken: loginBundle.RefreshToken,
	}, "127.0.0.1")
	if err != nil {
		t.Fatalf("refresh token failed: %v", err)
	}
	if refreshed.AccessToken == "" {
		t.Fatal("expected new access token")
	}

	// Logout
	err = svc.Logout(ctx, domain.AuthLogoutRequest{
		RefreshToken: loginBundle.RefreshToken,
	})
	if err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Refresh after logout must fail
	_, err = svc.Refresh(ctx, domain.AuthRefreshRequest{
		RefreshToken: loginBundle.RefreshToken,
	}, "127.0.0.1")
	if err == nil {
		t.Fatal("expected refresh to fail after logout")
	}
}

func TestClusterCRUD(t *testing.T) {
	svc, _, _, ctx := newTestMemService()

	// Create cluster
	c, err := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Foundation"})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if c.Name != "Foundation" {
		t.Fatalf("unexpected name: %s", c.Name)
	}

	// Duplicate active cluster name should fail
	_, err = svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Foundation"})
	if err == nil {
		t.Fatal("expected duplicate active cluster name to fail")
	}

	// Update / rename cluster
	updated, err := svc.UpdateCluster(ctx, c.ID, domain.ClusterUpdateRequest{Name: "Base"})
	if err != nil {
		t.Fatalf("update cluster: %v", err)
	}
	if updated.Name != "Base" {
		t.Fatalf("expected Base, got %s", updated.Name)
	}

	// Archive cluster
	archived := true
	updated, err = svc.UpdateCluster(ctx, c.ID, domain.ClusterUpdateRequest{Archived: &archived})
	if err != nil {
		t.Fatalf("archive cluster: %v", err)
	}
	if updated.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}

	// Unarchive cluster
	unarchived := false
	updated, err = svc.UpdateCluster(ctx, c.ID, domain.ClusterUpdateRequest{Archived: &unarchived})
	if err != nil {
		t.Fatalf("unarchive cluster: %v", err)
	}
	if updated.DeletedAt != nil {
		t.Fatal("expected DeletedAt to be nil")
	}
}

func TestActivityCRUD(t *testing.T) {
	svc, _, _, ctx := newTestMemService()

	c, _ := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Work"})

	// Create activity
	act, err := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    c.ID,
		Name:         "Coding",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	if err != nil {
		t.Fatalf("create activity: %v", err)
	}
	if act.Name != "Coding" || act.TrackingType != "duration" {
		t.Fatalf("unexpected activity: %+v", act)
	}

	// Duplicate in same cluster
	_, err = svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    c.ID,
		Name:         "coding",
		TrackingType: "duration",
	})
	if err == nil {
		t.Fatal("expected duplicate activity name to fail")
	}

	// Archive activity
	archived := true
	upAct, err := svc.UpdateActivity(ctx, act.ID, domain.ActivityUpdateRequest{Archived: &archived})
	if err != nil {
		t.Fatalf("archive activity: %v", err)
	}
	if upAct.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
}

func TestLogIntegrityRules(t *testing.T) {
	svc, _, _, ctx := newTestMemService()

	c, _ := svc.CreateCluster(ctx, domain.ClusterCreateRequest{Name: "Health"})
	sleep, _ := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    c.ID,
		Name:         "Sleep",
		TrackingType: "duration",
		UnitName:     "minutes",
	})
	water, _ := svc.CreateActivity(ctx, domain.ActivityCreateRequest{
		ClusterID:    c.ID,
		Name:         "Water",
		TrackingType: "count",
		UnitName:     "glasses",
	})

	now := time.Now().UTC()

	// 1. Future log rejected
	_, err := svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  water.ID,
		StartTime:   now.Add(10 * time.Minute).UnixMilli(),
		LoggedValue: 1.0,
	})
	if err == nil {
		t.Fatal("expected future log to be rejected")
	}

	// 2. Beyond 24 hours backdating rejected
	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  water.ID,
		StartTime:   now.Add(-25 * time.Hour).UnixMilli(),
		LoggedValue: 1.0,
	})
	if err == nil {
		t.Fatal("expected log >24h old to be rejected")
	}

	// 3. Negative count rejected
	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID:  water.ID,
		StartTime:   now.Add(-10 * time.Minute).UnixMilli(),
		LoggedValue: -2.0,
	})
	if err == nil {
		t.Fatal("expected negative count to be rejected")
	}

	// 4. Duration end before start rejected
	_, err = svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID: sleep.ID,
		StartTime:  now.Add(-30 * time.Minute).UnixMilli(),
		EndTime:    now.Add(-40 * time.Minute).UnixMilli(),
	})
	if err == nil {
		t.Fatal("expected duration end before start to be rejected")
	}

	// 5. Valid duration log -> verify lowercase status and calculated value
	st := now.Add(-60 * time.Minute)
	et := now.Add(-15 * time.Minute)
	validLog, err := svc.CreateLog(ctx, domain.LogCreateRequest{
		ActivityID: sleep.ID,
		StartTime:  st.UnixMilli(),
		EndTime:    et.UnixMilli(),
		Notes:      "Good sleep",
	})
	if err != nil {
		t.Fatalf("create valid duration log: %v", err)
	}
	if validLog.LoggedValue == nil || *validLog.LoggedValue != 45.0 {
		t.Fatalf("expected 45 minutes logged_value, got %v", validLog.LoggedValue)
	}
	if validLog.Status != "normal" && validLog.Status != "late" {
		t.Fatalf("expected lowercase status 'normal' or 'late', got: %s", validLog.Status)
	}

	// 6. Edit log -> creates audit entry and updates log
	newEnd := et.Add(5 * time.Minute)
	editedLog, err := svc.UpdateLog(ctx, validLog.ID, domain.LogUpdateRequest{
		EndTime: newEnd.UnixMilli(),
		Notes:   "Updated notes",
	})
	if err != nil {
		t.Fatalf("update log: %v", err)
	}
	if editedLog.Edited != 1 {
		t.Fatalf("expected edited count = 1, got %d", editedLog.Edited)
	}
	if editedLog.Status != "normal" && editedLog.Status != "late" {
		t.Fatalf("expected lowercase status, got %s", editedLog.Status)
	}

	// 7. Check audit history for log
	audits, err := svc.ListAuditForLog(ctx, validLog.ID)
	if err != nil {
		t.Fatalf("list audit for log: %v", err)
	}
	if len(audits) != 1 || audits[0].ActionType != "UPDATE" {
		t.Fatalf("expected 1 update audit entry, got %d", len(audits))
	}

	// 8. Delete log -> creates DELETE audit entry and keeps it in user audit list
	err = svc.DeleteLog(ctx, validLog.ID)
	if err != nil {
		t.Fatalf("delete log: %v", err)
	}

	allAudits, err := svc.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list all audits: %v", err)
	}
	var foundDelete bool
	for _, a := range allAudits {
		if a.LogID == validLog.ID && a.ActionType == "DELETE" {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Fatal("expected DELETE audit record to be preserved after log deletion")
	}
}
