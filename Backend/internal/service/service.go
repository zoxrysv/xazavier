package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"xazavier/internal/apperr"
	"xazavier/internal/auth"
	"xazavier/internal/config"
	"xazavier/internal/domain"
	"xazavier/internal/repository"
)

const lateWindow = 24 * time.Hour
const lateAfter = 3 * time.Hour

type Service struct {
	cfg   config.Config
	store repository.Store
}

func NewService(cfg config.Config, store repository.Store) *Service {
	return &Service{cfg: cfg, store: store}
}

func (s *Service) JWTSecret() string { return s.cfg.JWTSecret }

func (s *Service) CurrentUser(ctx context.Context) (domain.User, bool) {
	return currentUser(ctx)
}

func (s *Service) ValidateSession(sessionID string) (domain.User, bool) {
	session, ok := s.store.GetSessionByID(sessionID)
	if !ok || session.IsRevoked || time.Now().After(session.ExpiresAt) {
		return domain.User{}, false
	}
	user, ok := s.store.GetUserByID(session.UserID)
	if !ok {
		return domain.User{}, false
	}
	return user, true
}

func (s *Service) Register(ctx context.Context, req domain.AuthRegisterRequest, ip string) (domain.TokenBundle, error) {
	if req.FullName == "" || len(req.Password) < 8 || req.Email == "" {
		return domain.TokenBundle{}, apperr.InvalidRequest("Name, email and a password of at least 8 characters are required.")
	}
	email := auth.NormalizeEmail(req.Email)
	if req.Timezone == "" {
		req.Timezone = "Asia/Jakarta"
	}
	if _, ok := s.store.GetUserByEmail(email); ok {
		return domain.TokenBundle{}, apperr.Conflict("Registration rejected", "This email address is already registered.")
	}
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return domain.TokenBundle{}, apperr.InvalidRequest("Password could not be processed.")
	}
	user := domain.User{ID: uuid.NewString(), Email: email, PasswordHash: passwordHash, FullName: req.FullName, Timezone: req.Timezone, CreatedAt: time.Now().UTC()}
	if err := s.store.CreateUser(user); err != nil {
		return domain.TokenBundle{}, apperr.Conflict("Registration rejected", "This email address is already registered.")
	}
	return s.issueTokensForUser(user, ip)
}

func (s *Service) Login(ctx context.Context, req domain.AuthLoginRequest, ip string) (domain.TokenBundle, error) {
	email := auth.NormalizeEmail(req.Email)
	user, ok := s.store.GetUserByEmail(email)
	if !ok {
		return domain.TokenBundle{}, apperr.Unauthorized("The email or password is incorrect.")
	}
	if !auth.VerifyPassword(req.Password, user.PasswordHash) {
		return domain.TokenBundle{}, apperr.Unauthorized("The email or password is incorrect.")
	}
	return s.issueTokensForUser(user, ip)
}

func (s *Service) Refresh(ctx context.Context, req domain.AuthRefreshRequest, ip string) (domain.TokenBundle, error) {
	if req.RefreshToken == "" {
		return domain.TokenBundle{}, apperr.Unauthorized("Session expired.")
	}
	hash := auth.HashRefreshToken(req.RefreshToken)
	session, ok := s.store.GetSessionByRefreshHash(hash)
	if !ok || session.IsRevoked || time.Now().After(session.ExpiresAt) {
		return domain.TokenBundle{}, apperr.Unauthorized("Session expired.")
	}
	user, ok := s.store.GetUserByID(session.UserID)
	if !ok {
		return domain.TokenBundle{}, apperr.NotFound()
	}
	access, err := auth.GenerateAccessToken(user.ID, session.ID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.TokenBundle{}, apperr.InternalServerError()
	}
	return domain.TokenBundle{AccessToken: access, User: user.Public()}, nil
}

func (s *Service) Logout(ctx context.Context, req domain.AuthLogoutRequest) error {
	if req.RefreshToken == "" {
		return nil
	}
	s.store.RevokeSessionByRefreshTokenHash(auth.HashRefreshToken(req.RefreshToken))
	return nil
}

func (s *Service) GetUserByID(ctx context.Context, id string) (domain.User, bool) {
	return s.store.GetUserByID(id)
}

func (s *Service) ListClusters(ctx context.Context) ([]domain.Cluster, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return nil, apperr.Unauthorized("Missing session.")
	}
	return s.store.ListClusters(user.ID), nil
}

func (s *Service) CreateCluster(ctx context.Context, req domain.ClusterCreateRequest) (domain.Cluster, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Cluster{}, apperr.Unauthorized("Missing session.")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return domain.Cluster{}, apperr.InvalidRequest("Cluster name is required.")
	}
	cluster := domain.Cluster{ID: uuid.NewString(), UserID: user.ID, Name: name, Icon: req.Icon, Color: req.Color, CreatedAt: time.Now().UTC()}
	if err := s.store.AddCluster(cluster); err != nil {
		return domain.Cluster{}, err
	}
	return cluster, nil
}

func (s *Service) UpdateCluster(ctx context.Context, id string, req domain.ClusterUpdateRequest) (domain.Cluster, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Cluster{}, apperr.Unauthorized("Missing session.")
	}
	cluster, ok := s.store.GetClusterByID(user.ID, id)
	if !ok {
		return domain.Cluster{}, apperr.NotFound()
	}
	if req.Name != "" {
		cluster.Name = strings.TrimSpace(req.Name)
	}
	if req.Icon != "" {
		cluster.Icon = req.Icon
	}
	if req.Color != "" {
		cluster.Color = req.Color
	}
	if req.Archived != nil {
		if *req.Archived {
			now := time.Now().UTC(); cluster.DeletedAt = &now
		} else {
			cluster.DeletedAt = nil
		}
	}
	if err := s.store.UpdateCluster(cluster); err != nil {
		return domain.Cluster{}, err
	}
	return cluster, nil
}

func (s *Service) DeleteCluster(ctx context.Context, id string) error {
	user, ok := currentUser(ctx)
	if !ok {
		return apperr.Unauthorized("Missing session.")
	}
	cluster, ok := s.store.GetClusterByID(user.ID, id)
	if !ok {
		return apperr.NotFound()
	}
	if cluster.DeletedAt != nil {
		return nil
	}
	return s.store.DeleteCluster(user.ID, id)
}

func (s *Service) ListActivities(ctx context.Context) ([]domain.Activity, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return nil, apperr.Unauthorized("Missing session.")
	}
	return s.store.ListActivities(user.ID), nil
}

func (s *Service) CreateActivity(ctx context.Context, req domain.ActivityCreateRequest) (domain.Activity, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Activity{}, apperr.Unauthorized("Missing session.")
	}
	if strings.TrimSpace(req.Name) == "" {
		return domain.Activity{}, apperr.InvalidRequest("Activity name is required.")
	}
	cluster, ok := s.store.GetClusterByID(user.ID, req.ClusterID)
	if !ok || cluster.DeletedAt != nil {
		return domain.Activity{}, apperr.UnprocessableEntity("Activity not created", "The selected cluster does not exist or is archived.", "CLUSTER_INVALID")
	}
	for _, existing := range s.store.ListActivities(user.ID) {
		if existing.ClusterID == req.ClusterID && existing.DeletedAt == nil && strings.EqualFold(existing.Name, req.Name) {
			return domain.Activity{}, apperr.UnprocessableEntity("Activity not created", "An active activity with this name already exists in this cluster.", "DUPLICATE_ACTIVITY")
		}
	}
	activity := domain.Activity{ID: uuid.NewString(), UserID: user.ID, ClusterID: req.ClusterID, Name: req.Name, TrackingType: req.TrackingType, UnitName: req.UnitName, CreatedAt: time.Now().UTC()}
	if err := s.store.AddActivity(activity); err != nil {
		return domain.Activity{}, err
	}
	return activity, nil
}

func (s *Service) UpdateActivity(ctx context.Context, id string, req domain.ActivityUpdateRequest) (domain.Activity, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Activity{}, apperr.Unauthorized("Missing session.")
	}
	activity, ok := s.store.GetActivityByID(user.ID, id)
	if !ok {
		return domain.Activity{}, apperr.NotFound()
	}
	if req.Name != "" {
		activity.Name = strings.TrimSpace(req.Name)
	}
	if req.ClusterID != "" {
		activity.ClusterID = req.ClusterID
	}
	if req.UnitName != "" {
		activity.UnitName = req.UnitName
	}
	if req.TrackingType != "" {
		activity.TrackingType = req.TrackingType
	}
	if req.Archived != nil {
		if *req.Archived {
			now := time.Now().UTC(); activity.DeletedAt = &now
		} else {
			activity.DeletedAt = nil
		}
	}
	if err := s.store.UpdateActivity(activity); err != nil {
		return domain.Activity{}, err
	}
	return activity, nil
}

func (s *Service) DeleteActivity(ctx context.Context, id string) error {
	user, ok := currentUser(ctx)
	if !ok {
		return apperr.Unauthorized("Missing session.")
	}
	_, ok = s.store.GetActivityByID(user.ID, id)
	if !ok {
		return apperr.NotFound()
	}
	return s.store.DeleteActivity(user.ID, id)
}

func (s *Service) ListLogs(ctx context.Context, filter domain.LogListFilter) ([]domain.Log, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return nil, apperr.Unauthorized("Missing session.")
	}
	logs := s.store.ListLogs(user.ID, filter)
	for i := range logs {
		logs[i].Status = strings.ToLower(logs[i].EntryStatus)
	}
	return logs, nil
}

func (s *Service) CreateLog(ctx context.Context, req domain.LogCreateRequest) (domain.Log, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Log{}, apperr.Unauthorized("Missing session.")
	}
	activity, ok := s.store.GetActivityByID(user.ID, req.ActivityID)
	if !ok || activity.DeletedAt != nil {
		return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "This activity or its cluster is archived or does not exist.", "ACTIVITY_INVALID")
	}
	cluster, ok := s.store.GetClusterByID(user.ID, activity.ClusterID)
	if !ok || cluster.DeletedAt != nil {
		return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "This activity or its cluster is archived or does not exist.", "CLUSTER_INVALID")
	}
	start, err := parseTime(req.StartTime)
	if err != nil {
		return domain.Log{}, apperr.InvalidRequest("A valid start time is required.")
	}
	now := time.Now().UTC()
	if start.After(now.Add(5 * time.Minute)) {
		return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "This activity is set in the future. Only activity that has already started can be recorded.", "FUTURE_LOG")
	}
	if now.Sub(start) > lateWindow {
		return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", fmt.Sprintf("This activity started %s ago, beyond the 24-hour limit, and can no longer be added.", formatDuration(now.Sub(start))), "LATE_WINDOW")
	}
	logDate := formatLogDate(user.Timezone, start)
	entryStatus := "NORMAL"
	if now.Sub(start) > lateAfter || formatLogDate(user.Timezone, now) != logDate {
		entryStatus = "LATE"
	}
	newLog := domain.Log{ID: uuid.NewString(), ActivityID: activity.ID, UserID: user.ID, LogDate: logDate, StartTime: start, EntryStatus: entryStatus, Status: strings.ToLower(entryStatus), Notes: req.Notes, CreatedAt: now, UpdatedAt: now, Edited: 0}
	if activity.TrackingType == "duration" {
		end, err := parseTime(req.EndTime)
		if err != nil || end.Before(start) {
			return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "A duration activity needs an end time that is not earlier than its start time.", "DURATION_INVALID")
		}
		if end.After(now.Add(5 * time.Minute)) {
			return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "The end time is in the future.", "FUTURE_END_TIME")
		}
		newLog.EndTime = &end
		newLog.LoggedValue = float64Ptr(math.Round(end.Sub(start).Minutes()))
	} else {
		if req.LoggedValue == nil {
			return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "A count activity needs a non-negative quantity.", "COUNT_INVALID")
		}
		val, err := parseFloat(req.LoggedValue)
		if err != nil || val < 0 {
			return domain.Log{}, apperr.UnprocessableEntity("Recording rejected", "A count activity needs a non-negative quantity.", "COUNT_INVALID")
		}
		newLog.LoggedValue = &val
	}
	if err := s.store.AddLog(newLog); err != nil {
		return domain.Log{}, err
	}
	return newLog, nil
}

func (s *Service) GetLog(ctx context.Context, id string) (domain.Log, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Log{}, apperr.Unauthorized("Missing session.")
	}
	log, ok := s.store.GetLogByID(user.ID, id)
	if !ok {
		return domain.Log{}, apperr.NotFound()
	}
	log.Status = strings.ToLower(log.EntryStatus)
	return log, nil
}

func (s *Service) UpdateLog(ctx context.Context, id string, req domain.LogUpdateRequest) (domain.Log, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return domain.Log{}, apperr.Unauthorized("Missing session.")
	}
	log, ok := s.store.GetLogByID(user.ID, id)
	if !ok {
		return domain.Log{}, apperr.NotFound()
	}
	now := time.Now().UTC()
	if now.Sub(log.StartTime) >= lateWindow {
		return domain.Log{}, apperr.UnprocessableEntity("Edit rejected", "This record started more than 24 hours ago and is permanently locked.\n\nThis restriction protects the integrity of historical data.", "LOCKED_LOG")
	}
	activity, ok := s.store.GetActivityByID(user.ID, log.ActivityID)
	if !ok {
		return domain.Log{}, apperr.NotFound()
	}
	oldStart := log.StartTime
	oldEnd := log.EndTime
	oldValue := log.LoggedValue
	if req.StartTime != nil {
		newStart, err := parseTime(req.StartTime)
		if err != nil {
			return domain.Log{}, apperr.InvalidRequest("The start time is invalid.")
		}
		if newStart.After(now.Add(5*time.Minute)) {
			return domain.Log{}, apperr.UnprocessableEntity("Edit rejected", "The start time is in the future.", "FUTURE_START")
		}
		log.StartTime = newStart
		log.LogDate = formatLogDate(user.Timezone, newStart)
	}
	if activity.TrackingType == "duration" {
		if req.EndTime != nil {
			newEnd, err := parseTime(req.EndTime)
			if err != nil {
				return domain.Log{}, apperr.InvalidRequest("The end time is invalid.")
			}
			if newEnd.Before(log.StartTime) {
				return domain.Log{}, apperr.UnprocessableEntity("Edit rejected", "The end time cannot be earlier than the start time.", "DURATION_INVALID")
			}
			if newEnd.After(now.Add(5*time.Minute)) {
				return domain.Log{}, apperr.UnprocessableEntity("Edit rejected", "The end time is in the future.", "FUTURE_END_TIME")
			}
			log.EndTime = &newEnd
			log.LoggedValue = float64Ptr(math.Round(newEnd.Sub(log.StartTime).Minutes()))
		}
	} else if req.LoggedValue != nil {
		value, err := parseFloat(req.LoggedValue)
		if err != nil || value < 0 {
			return domain.Log{}, apperr.UnprocessableEntity("Edit rejected", "The quantity cannot be negative.", "COUNT_INVALID")
		}
		log.LoggedValue = &value
	}
	if req.Notes != "" || req.Notes == "" && log.Notes != "" {
		log.Notes = req.Notes
	}
	log.UpdatedAt = now
	log.Status = strings.ToLower(log.EntryStatus)
	if oldStart != log.StartTime || !sameTimePtr(oldEnd, log.EndTime) || !sameFloatPtr(oldValue, log.LoggedValue) {
		audit := domain.Audit{ID: uuid.NewString(), UserID: user.ID, LogID: log.ID, ActionType: "UPDATE", OldStartTime: &oldStart, OldEndTime: oldEnd, OldLoggedValue: oldValue, ChangedAt: now}
		if err := s.store.AddAudit(audit); err != nil {
			return domain.Log{}, err
		}
		log.Edited = log.Edited + 1
	}
	if err := s.store.UpdateLog(log); err != nil {
		return domain.Log{}, err
	}
	return log, nil
}

func (s *Service) DeleteLog(ctx context.Context, id string) error {
	user, ok := currentUser(ctx)
	if !ok {
		return apperr.Unauthorized("Missing session.")
	}
	log, ok := s.store.GetLogByID(user.ID, id)
	if !ok {
		return apperr.NotFound()
	}
	if time.Now().UTC().Sub(log.StartTime) >= lateWindow {
		return apperr.UnprocessableEntity("Delete rejected", "This record started more than 24 hours ago and is locked.\n\nThis restriction protects the integrity of historical data.", "LOCKED_LOG")
	}
	audit := domain.Audit{ID: uuid.NewString(), UserID: user.ID, LogID: log.ID, ActionType: "DELETE", OldStartTime: &log.StartTime, OldEndTime: log.EndTime, OldLoggedValue: log.LoggedValue, ChangedAt: time.Now().UTC()}
	if err := s.store.AddAudit(audit); err != nil {
		return err
	}
	return s.store.DeleteLog(user.ID, id)
}

func (s *Service) ListAuditForLog(ctx context.Context, logID string) ([]domain.Audit, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return nil, apperr.Unauthorized("Missing session.")
	}
	return s.store.ListAuditForLog(user.ID, logID), nil
}

func (s *Service) ListAudit(ctx context.Context, filter domain.AuditListFilter) ([]domain.Audit, error) {
	user, ok := currentUser(ctx)
	if !ok {
		return nil, apperr.Unauthorized("Missing session.")
	}
	return s.store.ListAudit(user.ID, filter), nil
}

func (s *Service) Reset(ctx context.Context) error {
	user, ok := currentUser(ctx)
	if !ok {
		return apperr.Unauthorized("Missing session.")
	}
	return s.store.ResetUserData(user.ID)
}

func currentUser(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value("user").(domain.User)
	return user, ok
}

func (s *Service) issueTokensForUser(user domain.User, ip string) (domain.TokenBundle, error) {
	refresh := auth.GenerateRefreshToken()
	sessionID := uuid.NewString()
	access, err := auth.GenerateAccessToken(user.ID, sessionID, s.cfg.JWTSecret, s.cfg.AccessTokenTTL)
	if err != nil {
		return domain.TokenBundle{}, apperr.InternalServerError()
	}
	session := domain.Session{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: auth.HashRefreshToken(refresh),
		UserAgent:        "",
		IPAddress:        ip,
		IsRevoked:        false,
		ExpiresAt:        time.Now().Add(s.cfg.RefreshTokenTTL),
		CreatedAt:        time.Now().UTC(),
	}
	if err := s.store.AddSession(session); err != nil {
		return domain.TokenBundle{}, err
	}
	return domain.TokenBundle{AccessToken: access, RefreshToken: refresh, User: user.Public()}, nil
}

func parseTime(value any) (time.Time, error) {
	if value == nil {
		return time.Time{}, fmt.Errorf("missing time")
	}
	switch v := value.(type) {
	case time.Time:
		return v.UTC(), nil
	case string:
		trim := strings.TrimSpace(v)
		if trim == "" { return time.Time{}, fmt.Errorf("empty time") }
		if ts, err := time.Parse(time.RFC3339, trim); err == nil { return ts.UTC(), nil }
		if ts, err := time.Parse("2006-01-02T15:04:05", trim); err == nil { return ts.UTC(), nil }
		if ts, err := time.Parse("2006-01-02 15:04:05", trim); err == nil { return ts.UTC(), nil }
		return time.Time{}, fmt.Errorf("invalid time: %s", trim)
	case float64:
		return time.Unix(0, int64(v)*int64(time.Millisecond)).UTC(), nil
	case int64:
		return time.Unix(0, v*int64(time.Millisecond)).UTC(), nil
	case int:
		return time.Unix(0, int64(v)*int64(time.Millisecond)).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported time type %T", value)
	}
}

func parseFloat(value any) (float64, error) {
	if value == nil {
		return 0, fmt.Errorf("missing value")
	}
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(strings.TrimSpace(v), 64)
	default:
		return 0, fmt.Errorf("unsupported value type %T", value)
	}
}

func float64Ptr(v float64) *float64 { return &v }

func sameTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return a.Equal(*b)
}

func sameFloatPtr(a, b *float64) bool {
	if a == nil || b == nil { return a == nil && b == nil }
	return *a == *b
}

func formatDuration(d time.Duration) string {
	minutes := int(math.Round(d.Minutes()))
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %dm", minutes/60, minutes%60)
}

func formatLogDate(timezone string, ts time.Time) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil || loc == nil {
		return ts.UTC().Format("2006-01-02")
	}
	return ts.In(loc).Format("2006-01-02")
}
