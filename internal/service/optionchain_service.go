// Package service contains the service layer for the Moneybots API
package service

import (
	"fmt"

	"github.com/nsvirk/moneybotsapi/internal/repository"
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/state"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"gorm.io/gorm"
)

// InstrumentService is the service for managing instruments
type OptionchainService struct {
	repo   *repository.InstrumentRepository
	state  *state.State
	logger *logger.Logger
}

// NewOptionchainService creates a new optionchain service
func NewOptionchainService(db *gorm.DB) *OptionchainService {
	stateManager, err := state.NewState(db)
	if err != nil {
		zaplogger.Fatal("failed to create state manager", zaplogger.Fields{"error": err})
	}
	logger, err := logger.New(db, "OPTIONCHAIN SERVICE")
	if err != nil {
		zaplogger.Error("failed to create optionchain logger", zaplogger.Fields{"error": err})
	}

	return &OptionchainService{
		repo:   repository.NewInstrumentRepository(db),
		state:  stateManager,
		logger: logger,
	}
}

// GetOptionChainNames returns a list of exchange:name for a given expiry
func (s *OptionchainService) GetOptionChainNames(expiry string) ([]string, error) {
	return s.repo.GetOptionChainNames(expiry)
}

// GetOptionChainInstruments returns a list of instruments for a given exchange, name and expiry
func (s *OptionchainService) GetOptionChainInstruments(exchange, name, expiry, details string) ([]interface{}, error) {

	instruments, err := s.repo.GetOptionChainInstruments(exchange, name, expiry)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(instruments))
	if details == "t" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%d", instrument.InstrumentToken)
		}
	} else if details == "i" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%s:%s", instrument.Exchange, instrument.Tradingsymbol)
		}
	} else if details == "it" {
		for i, instrument := range instruments {
			result[i] = fmt.Sprintf("%s:%s:%d", instrument.Exchange, instrument.Tradingsymbol, instrument.InstrumentToken)
		}
	} else {
		for i, instrument := range instruments {
			result[i] = instrument
		}
	}

	return result, nil
}
