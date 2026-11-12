package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"xazavier/internal/config"
	"xazavier/internal/domain"
	"xazavier/internal/repository"
)

func setupTestServer() (http.Handler, string, *repository.MemStore) {
	store := repository.NewMemoryStore()
	repository.SeedMemoryStore(store)

	cfg := config.Config{
		Port:            "8080",
		JWTSecret:       strings.Repeat("k", 32),
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	server := NewServer(cfg, store)

	// Login with pre-seeded demo user
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "demo@xazavier.app",
		"password": "demo1234",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var bundle domain.TokenBundle
	_ = json.NewDecoder(w.Body).Decode(&bundle)

	return server, bundle.AccessToken, store
}

func TestHealthEndpoints(t *testing.T) {
	server, _, _ := setupTestServer()

	for _, endpoint := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest("GET", endpoint, nil)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("%s returned status %d", endpoint, w.Code)
		}
	}
}

func TestMeEndpoint(t *testing.T) {
	server, token, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var u domain.UserPublic
	if err := json.NewDecoder(w.Body).Decode(&u); err != nil {
		t.Fatal(err)
	}
	if u.Email != "demo@xazavier.app" {
		t.Fatalf("unexpected email: %s", u.Email)
	}
}

func TestClustersAPI(t *testing.T) {
	server, token, _ := setupTestServer()

	// List clusters
	req := httptest.NewRequest("GET", "/api/clusters", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list clusters: %d", w.Code)
	}
	var clusters []domain.Cluster
	_ = json.NewDecoder(w.Body).Decode(&clusters)
	if len(clusters) < 3 {
		t.Fatalf("expected at least 3 clusters, got %d", len(clusters))
	}

	// Create cluster
	body, _ := json.Marshal(map[string]string{"name": "New Project"})
	req = httptest.NewRequest("POST", "/api/clusters", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create cluster: %d", w.Code)
	}
}

func TestActivitiesAPI(t *testing.T) {
	server, token, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/api/activities", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list activities: %d", w.Code)
	}
	var acts []domain.Activity
	_ = json.NewDecoder(w.Body).Decode(&acts)
	if len(acts) < 7 {
		t.Fatalf("expected at least 7 activities, got %d", len(acts))
	}
}

func TestLogsAndAuditAPI(t *testing.T) {
	server, token, _ := setupTestServer()

	// List logs
	req := httptest.NewRequest("GET", "/api/logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list logs: %d", w.Code)
	}
	var logs []domain.Log
	_ = json.NewDecoder(w.Body).Decode(&logs)
	if len(logs) == 0 {
		t.Fatal("expected pre-seeded logs")
	}

	// Verify status is lowercase "normal" or "late"
	for _, l := range logs {
		if l.Status != "normal" && l.Status != "late" {
			t.Fatalf("expected lowercase status 'normal' or 'late', got '%s'", l.Status)
		}
	}

	// List user audit
	req = httptest.NewRequest("GET", "/api/audit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("list audit: %d", w.Code)
	}

	// Add log
	now := time.Now().UTC()
	actID := logs[0].ActivityID
	createBody, _ := json.Marshal(map[string]any{
		"activity_id": actID,
		"start_time":  now.Add(-30 * time.Minute).UnixMilli(),
		"end_time":    now.Add(-10 * time.Minute).UnixMilli(),
		"notes":       "Testing",
	})
	req = httptest.NewRequest("POST", "/api/logs", bytes.NewReader(createBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create log: %d %s", w.Code, w.Body.String())
	}
	var createdLog domain.Log
	_ = json.NewDecoder(w.Body).Decode(&createdLog)
	if createdLog.Status != "normal" && createdLog.Status != "late" {
		t.Fatalf("expected lowercase status on created log, got %s", createdLog.Status)
	}

	// Delete log
	req = httptest.NewRequest("DELETE", "/api/logs/"+createdLog.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete log: %d %s", w.Code, w.Body.String())
	}

	// Audit must have the DELETE record
	req = httptest.NewRequest("GET", "/api/audit", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var allAudit []domain.Audit
	_ = json.NewDecoder(w.Body).Decode(&allAudit)
	var foundDelete bool
	for _, a := range allAudit {
		if a.LogID == createdLog.ID && a.ActionType == "DELETE" {
			foundDelete = true
			break
		}
	}
	if !foundDelete {
		t.Fatal("expected DELETE audit record in /api/audit")
	}
}

func TestResetAPI(t *testing.T) {
	server, token, _ := setupTestServer()

	req := httptest.NewRequest("POST", "/api/reset", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("reset API: %d", w.Code)
	}

	// Check logs are cleared
	req = httptest.NewRequest("GET", "/api/logs", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var logs []domain.Log
	_ = json.NewDecoder(w.Body).Decode(&logs)
	if len(logs) != 0 {
		t.Fatalf("expected 0 logs after reset, got %d", len(logs))
	}
}

