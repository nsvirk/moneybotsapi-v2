// Package repository contains the repository layer for the Moneybots API
package repository

import (
	"github.com/nsvirk/moneybotsapi/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionRepository struct {
	DB *gorm.DB
}

// NewSessionRepository creates a new repository for the session API
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{DB: db}
}

// UpsertSession upserts a session into the database
func (r *SessionRepository) UpsertSession(session *models.SessionModel) error {
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_name", "user_shortname", "avatar_url", "public_token", "kf_session", "enctoken", "login_time", "hashed_password", "updated_at"}),
	}).Create(session).Error
}

// GetSessionByUserID gets a session by user ID
func (r *SessionRepository) GetSessionByUserID(userID string) (*models.SessionModel, error) {
	var session models.SessionModel
	err := r.DB.Where("user_id = ?", userID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}
