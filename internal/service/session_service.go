// Package session handles the API for session operations
package service

import (
	"fmt"

	kitesession "github.com/nsvirk/gokitesession"
	"github.com/nsvirk/moneybotsapi/internal/models"
	"github.com/nsvirk/moneybotsapi/internal/repository"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type SessionService struct {
	repo        *repository.SessionRepository
	kiteSession *kitesession.Client
}

// NewSessionService creates a new service for the session API
func NewSessionService(db *gorm.DB) *SessionService {
	return &SessionService{
		repo:        repository.NewSessionRepository(db),
		kiteSession: kitesession.New(),
	}
}

// GenerateSession generates a new session for the given user
func (s *SessionService) GenerateSession(userId, password, totpValue string) (models.SessionModel, error) {

	existingSession, err := s.repo.GetSessionByUserId(userId)
	if err == nil {
		if err := bcrypt.CompareHashAndPassword([]byte(existingSession.HashedPassword), []byte(password)); err == nil {
			isValid, err := s.kiteSession.CheckEnctokenValid(existingSession.Enctoken)
			if err == nil && isValid {
				return *existingSession, nil
			}
		}
	}

	session, err := s.kiteSession.GenerateSession(userId, password, totpValue)
	if err != nil {
		return models.SessionModel{}, fmt.Errorf("login failed: %v", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.SessionModel{}, fmt.Errorf("failed to hash password: %v", err)
	}

	newSession := models.SessionModel{
		UserId:         session.UserID,
		UserName:       session.Username,
		UserShortname:  session.UserShortname,
		AvatarUrl:      session.AvatarURL,
		PublicToken:    session.PublicToken,
		KfSession:      session.KFSession,
		Enctoken:       session.Enctoken,
		LoginTime:      session.LoginTime,
		HashedPassword: string(hashedPassword),
	}

	if err := s.repo.UpsertSession(&newSession); err != nil {
		return models.SessionModel{}, fmt.Errorf("failed to upsert session: %v", err)
	}

	return newSession, nil
}

// GenerateTOTP generates a TOTP value for the given secret
func (s *SessionService) GenerateTOTP(totpSecret string) (string, error) {
	return kitesession.GenerateTOTPValue(totpSecret)
}

// DeleteSession deletes the session for the given user
func (s *SessionService) DeleteSession(userId, enctoken string) (int64, error) {
	return s.repo.DeleteSession(userId, enctoken)
}

// CheckEnctokenValid checks if the enctoken is valid
// Checks from KiteConnect API
func (s *SessionService) CheckEnctokenValid(enctoken string) (bool, error) {
	return s.kiteSession.CheckEnctokenValid(enctoken)
}

// VerifySessionForAuthorization verifies the session for the given enctoken
// If valid also returns the session details
// Used by the AuthMiddleware to verify the session
func (s *SessionService) VerifySessionForAuthorization(enctoken string) (*models.SessionModel, error) {
	session, err := s.repo.GetSessionByEnctoken(enctoken)
	if err != nil {
		return nil, err
	}

	// Verify if the session is still valid with KiteConnect API
	isValid, err := s.kiteSession.CheckEnctokenValid(enctoken)
	if err != nil || !isValid {
		return nil, fmt.Errorf("expired or invalid session")
	}

	return session, nil
}
