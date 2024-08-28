package session

import (
	"time"
)

const SessionsTableName = "api_sessions"

type SessionModel struct {
	UserID         string    `gorm:"primaryKey;uniqueIndex" json:"user_id"`
	UserName       string    `json:"user_name"`
	UserShortname  string    `json:"user_shortname"`
	AvatarURL      string    `json:"avatar_url"`
	PublicToken    string    `json:"public_token"`
	KFSession      string    `json:"kf_session"`
	Enctoken       string    `json:"enctoken"`
	LoginTime      string    `json:"login_time"`
	HashedPassword string    `json:"-"` // Store hashed password, but don't include in JSON output
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"-"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"-"`
}

func (SessionModel) TableName() string {
	return SessionsTableName
}
