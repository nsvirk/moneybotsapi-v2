package session

import (
	"fmt"
	"log"

	kitesession "github.com/nsvirk/gokitesession"
	"github.com/nsvirk/moneybotsapi/shared/logger"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var SessionLogsTableName = "_session_logs"

type Service struct {
	repo        *Repository
	kiteSession *kitesession.Client
	logger      *logger.Logger
}

func NewService(db *gorm.DB) *Service {
	logger, err := logger.New(db, SessionLogsTableName)
	if err != nil {
		log.Fatalf("failed to create cron logger: %v", err)
	}
	return &Service{
		repo:        NewRepository(db),
		kiteSession: kitesession.New(),
		logger:      logger,
	}
}

type LoginRequest struct {
	UserID     string `json:"user_id"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
}

func (s *Service) GenerateSession(req LoginRequest) (SessionModel, error) {
	if req.UserID == "" || req.Password == "" || req.TOTPSecret == "" {
		return SessionModel{}, fmt.Errorf("user_id, password, and totp_secret are required")
	}

	totpValue, err := kitesession.GenerateTOTPValue(req.TOTPSecret)
	if err != nil {
		s.logger.Error("Failed to generate TOTP value", map[string]interface{}{
			"user_id":         req.UserID,
			"len_totp_secret": len(req.TOTPSecret),
			"error":           err,
		})
		return SessionModel{}, fmt.Errorf("failed to generate TOTP value: %v", err)
	}

	existingSession, err := s.repo.GetSessionByUserID(req.UserID)
	if err == nil {
		if err := bcrypt.CompareHashAndPassword([]byte(existingSession.HashedPassword), []byte(req.Password)); err == nil {
			isValid, err := s.kiteSession.CheckEnctokenValid(existingSession.Enctoken)
			if err == nil && isValid {
				s.logger.Info("Session exists", map[string]interface{}{
					"user_id":    req.UserID,
					"enctoken":   fmt.Sprintf("%v***", existingSession.Enctoken[:4]),
					"login_time": existingSession.LoginTime,
				})
				return *existingSession, nil
			}
		}
	}

	session, err := s.kiteSession.GenerateSession(req.UserID, req.Password, totpValue)
	if err != nil {
		s.logger.Error("Failed to generate session", map[string]interface{}{
			"user_id":    req.UserID,
			"password":   fmt.Sprintf("%v***", req.Password[:4]),
			"totp_value": totpValue,
			"error":      err,
		})
		return SessionModel{}, fmt.Errorf("login failed: %v", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password", map[string]interface{}{
			"user_id":  req.UserID,
			"password": fmt.Sprintf("%v***", req.Password[:4]),
			"error":    err,
		})
		return SessionModel{}, fmt.Errorf("failed to hash password: %v", err)
	}

	newSession := SessionModel{
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
		return SessionModel{}, fmt.Errorf("failed to upsert session: %v", err)
	}

	s.logger.Info("Session generated", map[string]interface{}{
		"user_id":    req.UserID,
		"enctoken":   fmt.Sprintf("%v***", newSession.Enctoken[:4]),
		"login_time": newSession.LoginTime,
	})

	return newSession, nil
}

func (s *Service) GenerateTOTP(totpSecret string) (string, error) {
	if totpSecret == "" {
		return "", fmt.Errorf("totp_secret is required")
	}

	return kitesession.GenerateTOTPValue(totpSecret)
}

func (s *Service) CheckSessionValid(enctoken string) (bool, error) {
	if enctoken == "" {
		return false, fmt.Errorf("enctoken is required")
	}

	return s.kiteSession.CheckEnctokenValid(enctoken)
}

// Used by the AuthMiddleware to verify the session
func (s *Service) VerifySession(userID, enctoken string) (*SessionModel, error) {
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
