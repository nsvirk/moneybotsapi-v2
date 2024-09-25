// Package session handles the API for session operations
package service

import (
	"fmt"

	kitesession "github.com/nsvirk/gokitesession"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SessionService struct {
	repo        *repository.SessionRepository
	kiteSession *kitesession.Client
	logger      *logger.Logger
}

// NewSessionService creates a new service for the session API
func NewSessionService(db *gorm.DB) *SessionService {
	logger, err := logger.New(db, "SESSION SERVICE")
	if err != nil {
		zaplogger.Error("failed to create session logger", zaplogger.Fields{"error": err})
	}
	return &SessionService{
		repo:        repository.NewSessionRepository(db),
		kiteSession: kitesession.New(),
		logger:      logger,
	}
}

// GenerateSession generates a new session for the given user
func (s *SessionService) GenerateSession(userId, password, totpValue string) (models.SessionModel, error) {
	if userId == "" {
		return models.SessionModel{}, fmt.Errorf("`user_id` is required")
	}
	if password == "" {
		return models.SessionModel{}, fmt.Errorf("`password` is required")
	}
	if totpValue == "" {
		return models.SessionModel{}, fmt.Errorf("`totp_value` is required")
	}

	existingSession, err := s.repo.GetSessionByUserID(userId)
	if err == nil {
		if err := bcrypt.CompareHashAndPassword([]byte(existingSession.HashedPassword), []byte(password)); err == nil {
			isValid, err := s.kiteSession.CheckEnctokenValid(existingSession.Enctoken)
			if err == nil && isValid {
				s.logger.Info("Session exists", map[string]interface{}{
					"user_id":    userId,
					"enctoken":   fmt.Sprintf("%v***", existingSession.Enctoken[:4]),
					"login_time": existingSession.LoginTime,
				})
				return *existingSession, nil
			}
		}
	}

	session, err := s.kiteSession.GenerateSession(userId, password, totpValue)
	if err != nil {
		s.logger.Error("Failed to generate session", map[string]interface{}{
			"user_id":    userId,
			"password":   fmt.Sprintf("%v***", password[:4]),
			"totp_value": totpValue,
			"error":      err,
		})
		return models.SessionModel{}, fmt.Errorf("login failed: %v", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", map[string]interface{}{
			"user_id":  userId,
			"password": fmt.Sprintf("%v***", password[:4]),
			"error":    err,
		})
		return models.SessionModel{}, fmt.Errorf("failed to hash password: %v", err)
	}

	newSession := models.SessionModel{
		UserID:         session.UserID,
		UserName:       session.Username,
		UserShortname:  session.UserShortname,
		AvatarURL:      session.AvatarURL,
		PublicToken:    session.PublicToken,
		KFSession:      session.KFSession,
		Enctoken:       session.Enctoken,
		LoginTime:      session.LoginTime,
		HashedPassword: string(hashedPassword),
	}

	if err := s.repo.UpsertSession(&newSession); err != nil {
		s.logger.Error("Failed to upsert session", map[string]interface{}{
			"error": err,
		})
		return models.SessionModel{}, fmt.Errorf("failed to upsert session: %v", err)
	}

	s.logger.Info("Session generated", map[string]interface{}{
		"user_id":    userId,
		"enctoken":   fmt.Sprintf("%v***", newSession.Enctoken[:4]),
		"login_time": newSession.LoginTime,
	})

	return newSession, nil
}

// GenerateTOTP generates a TOTP value for the given secret
func (s *SessionService) GenerateTOTP(totpSecret string) (string, error) {
	if totpSecret == "" {
		return "", fmt.Errorf("totp_secret is required")
	}

	return kitesession.GenerateTOTPValue(totpSecret)
}

// CheckSessionValid checks if the given enctoken is valid
func (s *SessionService) CheckSessionValid(enctoken string) (bool, error) {
	if enctoken == "" {
		return false, fmt.Errorf("enctoken is required")
	}

	return s.kiteSession.CheckEnctokenValid(enctoken)
}

// Used by the AuthMiddleware to verify the session
// VerifySession verifies the session for the given user and enctoken
func (s *SessionService) VerifySession(userID, enctoken string) (*models.SessionModel, error) {
	session, err := s.repo.GetSessionByUserID(userID)
	if err != nil {
		return nil, err
	}

	if session.Enctoken != enctoken {
		return nil, fmt.Errorf("invalid enctoken")
	}

	// Optionally, you might want to check if the session is still valid
	// This could involve checking an expiration time, or making an API call to verify the enctoken
	isValid, err := s.kiteSession.CheckEnctokenValid(enctoken)
	if err != nil || !isValid {
		return nil, fmt.Errorf("expired or invalid session")
	}

	return session, nil
}
