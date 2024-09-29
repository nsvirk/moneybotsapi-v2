// Package models contains the models for the Moneybots API
package models

import (
	"time"
)

const SessionsTableName = "sessions"

type SessionModel struct {
	UserId         string    `gorm:"primaryKey;uniqueIndex;index:idx_uid_hpw,priority:1" json:"user_id"`
	UserName       string    `json:"user_name"`
	UserShortname  string    `json:"user_shortname"`
	AvatarUrl      string    `json:"avatar_url"`
	PublicToken    string    `json:"public_token"`
	KfSession      string    `json:"kf_session"`
	Enctoken       string    `gorm:"index" json:"enctoken"`
	LoginTime      string    `json:"login_time"`
	HashedPassword string    `gorm:"index:idx_uid_hpw,priority:2" json:"-"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"-"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"-"`
}

func (SessionModel) TableName() string {
	return SessionsTableName
}
