package session

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	DB *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{DB: db}
}

func (r *Repository) UpsertSession(session *SessionModel) error {
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_name", "user_shortname", "avatar_url", "public_token", "kf_session", "enctoken", "login_time", "hashed_password", "updated_at"}),
	}).Create(session).Error
}

func (r *Repository) GetSessionByUserID(userID string) (*SessionModel, error) {
	var session SessionModel
	err := r.DB.Where("user_id = ?", userID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}
