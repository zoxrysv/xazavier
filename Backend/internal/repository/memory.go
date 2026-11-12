package repository

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"xazavier/internal/apperr"
	"xazavier/internal/domain"
)

// MemStore is an in-memory thread-safe implementation of repository.Store.
// It allows running the Go backend without requiring external PostgreSQL
// and enables ultra-fast, dependency-free unit testing.
type MemStore struct {
	mu             sync.RWMutex
	users          map[string]domain.User       // id -> user
	usersByEmail   map[string]string          // lower(email) -> id
	sessions       map[string]domain.Session    // id -> session
	sessionsByHash map[string]string          // hash -> id
	clusters       map[string]domain.Cluster    // id -> cluster
	activities     map[string]domain.Activity   // id -> activity
	logs           map[string]domain.Log        // id -> log
	audit          []domain.Audit
}

func NewMemoryStore() *MemStore {
	return &MemStore{
		users:          make(map[string]domain.User),
		usersByEmail:   make(map[string]string),
		sessions:       make(map[string]domain.Session),
		sessionsByHash: make(map[string]string),
		clusters:       make(map[string]domain.Cluster),
		activities:     make(map[string]domain.Activity),
		logs:           make(map[string]domain.Log),
		audit:          make([]domain.Audit, 0),
	}
}

// User methods

func (m *MemStore) CreateUser(user domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	email := strings.ToLower(strings.TrimSpace(user.Email))
	if _, exists := m.usersByEmail[email]; exists {
		return apperr.Conflict("Registration rejected", "This email address is already registered.")
	}
	user.Email = email
	m.users[user.ID] = user
	m.usersByEmail[email] = user.ID
	return nil
}

func (m *MemStore) GetUserByEmail(email string) (domain.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.usersByEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return domain.User{}, false
	}
	user, ok := m.users[id]
	return user, ok
}

func (m *MemStore) GetUserByID(id string) (domain.User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, ok := m.users[id]
	return user, ok
}

func (m *MemStore) ListUsers() []domain.User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// Session methods

func (m *MemStore) AddSession(session domain.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[session.ID] = session
	m.sessionsByHash[session.RefreshTokenHash] = session.ID
	return nil
}

func (m *MemStore) GetSessionByID(id string) (domain.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	return s, ok
}

func (m *MemStore) GetSessionByRefreshHash(hash string) (domain.Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id, ok := m.sessionsByHash[hash]
	if !ok {
		return domain.Session{}, false
	}
	s, ok := m.sessions[id]
	return s, ok
}

func (m *MemStore) RevokeSessionByRefreshTokenHash(hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id, ok := m.sessionsByHash[hash]; ok {
		if s, ok := m.sessions[id]; ok {
			s.IsRevoked = true
			m.sessions[id] = s
		}
	}
}

func (m *MemStore) ListSessionsForUser(userID string) []domain.Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Session, 0)
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

// Cluster methods

func (m *MemStore) ListClusters(userID string) []domain.Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Cluster, 0)
	for _, c := range m.clusters {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (m *MemStore) GetClusterByID(userID, id string) (domain.Cluster, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.clusters[id]
	if !ok || c.UserID != userID {
		return domain.Cluster{}, false
	}
	return c, true
}

func (m *MemStore) AddCluster(cluster domain.Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.clusters {
		if c.UserID == cluster.UserID && c.DeletedAt == nil && strings.EqualFold(c.Name, cluster.Name) {
			return apperr.Conflict("Cluster not created", "An active cluster with this name already exists.")
		}
	}
	m.clusters[cluster.ID] = cluster
	return nil
}

func (m *MemStore) UpdateCluster(cluster domain.Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.clusters[cluster.ID]
	if !ok || existing.UserID != cluster.UserID {
		return apperr.NotFound()
	}
	for _, c := range m.clusters {
		if c.ID != cluster.ID && c.UserID == cluster.UserID && c.DeletedAt == nil && strings.EqualFold(c.Name, cluster.Name) {
			return apperr.Conflict("Cluster not updated", "An active cluster with this name already exists.")
		}
	}
	m.clusters[cluster.ID] = cluster
	return nil
}

func (m *MemStore) DeleteCluster(userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.clusters[id]
	if !ok || c.UserID != userID {
		return apperr.NotFound()
	}
	now := time.Now().UTC()
	c.DeletedAt = &now
	m.clusters[id] = c
	return nil
}

// Activity methods

func (m *MemStore) ListActivities(userID string) []domain.Activity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Activity, 0)
	for _, a := range m.activities {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (m *MemStore) GetActivityByID(userID, id string) (domain.Activity, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.activities[id]
	if !ok || a.UserID != userID {
		return domain.Activity{}, false
	}
	return a, true
}

func (m *MemStore) AddActivity(activity domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.activities {
		if a.UserID == activity.UserID && a.ClusterID == activity.ClusterID && a.DeletedAt == nil && strings.EqualFold(a.Name, activity.Name) {
			return apperr.Conflict("Activity not created", "An active activity with this name already exists in this cluster.")
		}
	}
	m.activities[activity.ID] = activity
	return nil
}

func (m *MemStore) UpdateActivity(activity domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.activities[activity.ID]
	if !ok || existing.UserID != activity.UserID {
		return apperr.NotFound()
	}
	for _, a := range m.activities {
		if a.ID != activity.ID && a.UserID == activity.UserID && a.ClusterID == activity.ClusterID && a.DeletedAt == nil && strings.EqualFold(a.Name, activity.Name) {
			return apperr.Conflict("Activity not updated", "An active activity with this name already exists in this cluster.")
		}
	}
	m.activities[activity.ID] = activity
	return nil
}

func (m *MemStore) DeleteActivity(userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.activities[id]
	if !ok || a.UserID != userID {
		return apperr.NotFound()
	}
	now := time.Now().UTC()
	a.DeletedAt = &now
	m.activities[id] = a
	return nil
}

// Log methods

func (m *MemStore) ListLogs(userID string, filter domain.LogListFilter) []domain.Log {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Log, 0)
	for _, l := range m.logs {
		if l.UserID != userID {
			continue
		}
		if filter.FromDate != "" && l.LogDate < filter.FromDate {
			continue
		}
		if filter.ToDate != "" && l.LogDate > filter.ToDate {
			continue
		}
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartTime.Before(out[j].StartTime)
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(out) {
			return []domain.Log{}
		}
		out = out[filter.Offset:]
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}

	return out
}

func (m *MemStore) GetLogByID(userID, id string) (domain.Log, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l, ok := m.logs[id]
	if !ok || l.UserID != userID {
		return domain.Log{}, false
	}
	return l, true
}

func (m *MemStore) AddLog(log domain.Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logs[log.ID] = log
	return nil
}

func (m *MemStore) UpdateLog(log domain.Log) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.logs[log.ID]
	if !ok || existing.UserID != log.UserID {
		return apperr.NotFound()
	}
	m.logs[log.ID] = log
	return nil
}

func (m *MemStore) DeleteLog(userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.logs[id]
	if !ok || l.UserID != userID {
		return apperr.NotFound()
	}
	delete(m.logs, id)
	return nil
}

func (m *MemStore) UpdateLogWithAudit(log domain.Log, audit *domain.Audit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.logs[log.ID]
	if !ok || l.UserID != log.UserID {
		return apperr.NotFound()
	}
	if audit != nil {
		m.audit = append(m.audit, *audit)
	}
	m.logs[log.ID] = log
	return nil
}

func (m *MemStore) DeleteLogWithAudit(userID, id string, audit domain.Audit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	l, ok := m.logs[id]
	if !ok || l.UserID != userID {
		return apperr.NotFound()
	}
	m.audit = append(m.audit, audit)
	delete(m.logs, id)
	return nil
}

// Audit methods

func (m *MemStore) ListAudit(userID string, filter domain.AuditListFilter) []domain.Audit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Audit, 0)
	for _, a := range m.audit {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ChangedAt.Before(out[j].ChangedAt)
	})

	if filter.Offset > 0 {
		if filter.Offset >= len(out) {
			return []domain.Audit{}
		}
		out = out[filter.Offset:]
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}

	return out
}

func (m *MemStore) AddAudit(audit domain.Audit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.audit = append(m.audit, audit)
	return nil
}

func (m *MemStore) ListAuditForLog(userID, logID string) []domain.Audit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]domain.Audit, 0)
	for _, a := range m.audit {
		if a.UserID == userID && a.LogID == logID {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ChangedAt.Before(out[j].ChangedAt)
	})
	return out
}

// ResetUserData removes all data associated with the user
func (m *MemStore) ResetUserData(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear audit
	filteredAudit := make([]domain.Audit, 0)
	for _, a := range m.audit {
		if a.UserID != userID {
			filteredAudit = append(filteredAudit, a)
		}
	}
	m.audit = filteredAudit

	// Clear logs
	for id, l := range m.logs {
		if l.UserID == userID {
			delete(m.logs, id)
		}
	}

	// Clear activities
	for id, a := range m.activities {
		if a.UserID == userID {
			delete(m.activities, id)
		}
	}

	// Clear clusters
	for id, c := range m.clusters {
		if c.UserID == userID {
			delete(m.clusters, id)
		}
	}

	// Clear sessions
	for id, s := range m.sessions {
		if s.UserID == userID {
			delete(m.sessionsByHash, s.RefreshTokenHash)
			delete(m.sessions, id)
		}
	}

	return nil
}

// SeedMemoryStore seeds the in-memory store with demo data matching mockServer.js
func SeedMemoryStore(store Store) {
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Demo user
	salt := []byte("xazavier-seed-salt")
	key := argon2.IDKey([]byte("demo1234"), salt, 1, 64*1024, 4, 32)
	pwHash := fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%x$%x", salt, key)

	user := domain.User{
		ID:           "demo",
		Email:        "demo@xazavier.app",
		PasswordHash: pwHash,
		FullName:     "Demo User",
		Timezone:     "Asia/Jakarta",
		CreatedAt:    now.Add(-30 * 24 * time.Hour),
	}
	_ = store.CreateUser(user)

	// Clusters
	clusterFoundation := domain.Cluster{ID: "c1", UserID: user.ID, Name: "Foundation", CreatedAt: now.Add(-30 * 24 * time.Hour)}
	clusterExecution := domain.Cluster{ID: "c2", UserID: user.ID, Name: "Execution", CreatedAt: now.Add(-30 * 24 * time.Hour)}
	clusterNonEssential := domain.Cluster{ID: "c3", UserID: user.ID, Name: "Non-essential", CreatedAt: now.Add(-30 * 24 * time.Hour)}
	_ = store.AddCluster(clusterFoundation)
	_ = store.AddCluster(clusterExecution)
	_ = store.AddCluster(clusterNonEssential)

	// Activities
	activities := []domain.Activity{
		{ID: "a1", UserID: user.ID, ClusterID: clusterFoundation.ID, Name: "Sleep", TrackingType: "duration", UnitName: "minutes", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a2", UserID: user.ID, ClusterID: clusterFoundation.ID, Name: "Meal", TrackingType: "count", UnitName: "meals", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a3", UserID: user.ID, ClusterID: clusterFoundation.ID, Name: "Prayer", TrackingType: "count", UnitName: "prayers", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a4", UserID: user.ID, ClusterID: clusterExecution.ID, Name: "Coding", TrackingType: "duration", UnitName: "minutes", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a5", UserID: user.ID, ClusterID: clusterExecution.ID, Name: "Studying", TrackingType: "duration", UnitName: "minutes", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a6", UserID: user.ID, ClusterID: clusterNonEssential.ID, Name: "Smoking", TrackingType: "count", UnitName: "cigarettes", CreatedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: "a7", UserID: user.ID, ClusterID: clusterNonEssential.ID, Name: "Gaming", TrackingType: "duration", UnitName: "minutes", CreatedAt: now.Add(-30 * 24 * time.Hour)},
	}
	for _, a := range activities {
		_ = store.AddActivity(a)
	}

	// 7 days of logs
	var codingLogID string
	var codingLog domain.Log

	for i := 0; i < 7; i++ {
		dayOffset := time.Duration(i) * 24 * time.Hour
		baseDay := startOfDay.Add(-dayOffset)
		k := float64((i * 3) % 5)

		addLog := func(act domain.Activity, startHour, endHour float64, countVal float64, lagMin float64) domain.Log {
			st := baseDay.Add(time.Duration(startHour * float64(time.Hour)))
			if st.After(now) {
				return domain.Log{}
			}
			var et *time.Time
			var val *float64
			if act.TrackingType == "duration" {
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

			l := domain.Log{
				ID:          uuid.NewString(),
				ActivityID:  act.ID,
				UserID:      user.ID,
				LogDate:     st.Format("2006-01-02"),
				StartTime:   st,
				EndTime:     et,
				LoggedValue: val,
				EntryStatus: entryStatus,
				Status:      strings.ToLower(entryStatus),
				Notes:       "",
				CreatedAt:   cr,
				UpdatedAt:   cr,
				Edited:      0,
			}
			_ = store.AddLog(l)
			return l
		}

		addLog(activities[0], 0.2, 6.4+k/10, 0, 5) // Sleep
		addLog(activities[1], 8.9, 0, 1, 3)         // Meal
		cl := addLog(activities[3], 9.5+k/20, 10.3+k/10, 0, func() float64 {
			if i == 2 {
				return 500
			}
			return 4
		}()) // Coding
		if i == 0 {
			codingLogID = cl.ID
			codingLog = cl
		}
		addLog(activities[1], 13.2, 0, 1, 6)               // Meal
		addLog(activities[2], 12.1, 0, 1, 8)               // Prayer
		addLog(activities[4], 14.5, 15.5+k/5, 0, 7)        // Studying
		addLog(activities[5], 16.3, 0, 1+float64(int(k)%3), 2) // Smoking
		addLog(activities[6], 19.7, 20.7+k*0.5, 0, 6)      // Gaming
		addLog(activities[1], 20.4, 0, 1, 9)               // Meal
	}

	// Add 1 audit entry for the Coding log
	if codingLogID != "" && codingLog.EndTime != nil && codingLog.LoggedValue != nil {
		oldEnd := codingLog.EndTime.Add(-5 * time.Minute)
		oldVal := *codingLog.LoggedValue - 5
		auditTime := codingLog.CreatedAt.Add(90 * time.Minute)
		if auditTime.After(now) {
			auditTime = now
		}
		_ = store.AddAudit(domain.Audit{
			ID:             uuid.NewString(),
			UserID:         user.ID,
			LogID:          codingLogID,
			ActionType:     "UPDATE",
			OldStartTime:   &codingLog.StartTime,
			OldEndTime:     &oldEnd,
			OldLoggedValue: &oldVal,
			ChangedAt:      auditTime,
		})
		codingLog.UpdatedAt = auditTime
		codingLog.Edited = 1
		_ = store.UpdateLog(codingLog)
	}
}

