package domain

import (
	"time"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Timezone     string    `json:"timezone"`
	CreatedAt    time.Time `json:"created_at"`
}

func (u User) Public() UserPublic {
	return UserPublic{ID: u.ID, Email: u.Email, FullName: u.FullName, Timezone: u.Timezone}
}

type UserPublic struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
	Timezone string `json:"timezone"`
}

type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	UserAgent        string    `json:"user_agent"`
	IPAddress        string    `json:"ip_address"`
	IsRevoked        bool      `json:"is_revoked"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type Cluster struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Icon      string     `json:"icon,omitempty"`
	Color     string     `json:"color,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Activity struct {
	ID          string     `json:"id"`
	ClusterID   string     `json:"cluster_id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	TrackingType string    `json:"tracking_type"`
	UnitName    string     `json:"unit_name,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Log struct {
	ID          string     `json:"id"`
	ActivityID  string     `json:"activity_id"`
	UserID      string     `json:"user_id"`
	LogDate     string     `json:"log_date"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	LoggedValue *float64   `json:"logged_value,omitempty"`
	EntryStatus string     `json:"entry_status"`
	Status      string     `json:"status"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Edited      int        `json:"edited"`
}

type SessionClaims struct {
	UserID    string `json:"sub"`
	SessionID string `json:"sid"`
}

type Audit struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	LogID          string     `json:"log_id"`
	ActionType     string     `json:"action_type"`
	OldStartTime   *time.Time `json:"old_start_time,omitempty"`
	OldEndTime     *time.Time `json:"old_end_time,omitempty"`
	OldLoggedValue *float64   `json:"old_logged_value,omitempty"`
	ChangedAt      time.Time  `json:"changed_at"`
}

type AuthRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Timezone string `json:"timezone,omitempty"`
}

type AuthLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthLogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type ClusterCreateRequest struct {
	Name  string `json:"name"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

type ClusterUpdateRequest struct {
	Name     string `json:"name,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Color    string `json:"color,omitempty"`
	Archived *bool  `json:"archived,omitempty"`
}

type ActivityCreateRequest struct {
	ClusterID    string `json:"cluster_id"`
	Name         string `json:"name"`
	TrackingType string `json:"tracking_type"`
	UnitName     string `json:"unit_name,omitempty"`
}

type ActivityUpdateRequest struct {
	Name         string `json:"name,omitempty"`
	ClusterID    string `json:"cluster_id,omitempty"`
	UnitName     string `json:"unit_name,omitempty"`
	Archived     *bool  `json:"archived,omitempty"`
	TrackingType string `json:"tracking_type,omitempty"`
}

type LogCreateRequest struct {
	ActivityID string      `json:"activity_id"`
	StartTime  interface{} `json:"start_time"`
	EndTime    interface{} `json:"end_time,omitempty"`
	LoggedValue interface{} `json:"logged_value,omitempty"`
	Notes      string      `json:"notes,omitempty"`
}

type LogUpdateRequest struct {
	EndTime    interface{} `json:"end_time,omitempty"`
	LoggedValue interface{} `json:"logged_value,omitempty"`
	Notes      string      `json:"notes,omitempty"`
	StartTime  interface{} `json:"start_time,omitempty"`
}

type TokenBundle struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	User         UserPublic `json:"user"`
}

type LogListFilter struct {
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
	FromDate string `json:"from_date,omitempty"`
	ToDate   string `json:"to_date,omitempty"`
}

type AuditListFilter struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}
