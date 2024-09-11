package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	kiteticker "github.com/nsvirk/gokiteticker"
	"github.com/nsvirk/moneybotsapi/api/instrument"
	"gorm.io/gorm"
)

type Service struct {
	instrumentService *instrument.InstrumentService
	ticker            *kiteticker.Ticker
	mu                sync.Mutex
	tokenMap          map[uint32]string
	tokens            []uint32
	clientClosed      chan struct{}
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		instrumentService: instrument.NewInstrumentService(db),
		tokenMap:          make(map[uint32]string),
		tokens:            make([]uint32, 0),
	}
}

func (s *Service) RunTickerStream(ctx context.Context, c echo.Context, userId, enctoken string, instruments []string, errChan chan<- error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenToInstrumentMap, tokens, err := s.prepareTokens(instruments)
	if err != nil {
		errChan <- err
		return
	}
	s.tokenMap = tokenToInstrumentMap
	s.tokens = tokens

	if err := s.initTicker(userId, enctoken); err != nil {
		errChan <- fmt.Errorf("failed to initialize ticker: %v", err)
		return
	}

	s.clientClosed = make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)

	go s.runTicker(ctx, &wg, c, tokens, errChan)

	// Handle client disconnection
	go func() {
		<-c.Request().Context().Done()
		log.Println("Client connection closed")
		close(s.clientClosed)
	}()

	wg.Wait()
}

func (s *Service) prepareTokens(strInstruments []string) (map[uint32]string, []uint32, error) {
	instrumentToTokenMap, err := s.instrumentService.GetInstrumentToTokenMap(strInstruments)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get instrument tokens: %w", err)
	}
	tokens := make([]uint32, 0, len(instrumentToTokenMap))
	tokenToInstrumentMap := make(map[uint32]string)
	for instrument, token := range instrumentToTokenMap {
		tokens = append(tokens, token)
		tokenToInstrumentMap[token] = instrument
	}

	if len(tokens) == 0 {
		return nil, nil, fmt.Errorf("no tokens to subscribe")
	}

	return tokenToInstrumentMap, tokens, nil
}

func (s *Service) initTicker(userId, enctoken string) error {
	s.ticker = kiteticker.New(userId, enctoken)
	if s.ticker == nil {
		return fmt.Errorf("failed to create ticker: returned nil")
	}
	return nil
}

func (s *Service) runTicker(ctx context.Context, wg *sync.WaitGroup, c echo.Context, tokens []uint32, errChan chan<- error) {
	defer wg.Done()
	defer s.ticker.Close()

	s.setupCallbacks(c, tokens, errChan)

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		s.ticker.Serve()
	}()

	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping ticker")
	case <-serveDone:
		log.Println("Ticker.Serve() returned unexpectedly")
		errChan <- fmt.Errorf("ticker.Serve() stopped unexpectedly")
	}
}

func (s *Service) setupCallbacks(c echo.Context, tokens []uint32, errChan chan<- error) {
	s.ticker.OnError(func(err error) {
		log.Printf("Ticker error: %v", err)
		errChan <- fmt.Errorf("ticker error: %w", err)
	})

	s.ticker.OnClose(func(code int, reason string) {
		log.Printf("Ticker closed: code=%d, reason=%s", code, reason)
		errChan <- fmt.Errorf("ticker closed: code=%d, reason=%s", code, reason)
	})

	s.ticker.OnConnect(func() {
		log.Println("Ticker connected")
		if err := s.ticker.Subscribe(tokens); err != nil {
			errChan <- fmt.Errorf("subscription error: %w", err)
			return
		}
		if err := s.ticker.SetMode(kiteticker.ModeFull, tokens); err != nil {
			errChan <- fmt.Errorf("set mode error: %w", err)
		}
	})

	s.ticker.OnReconnect(func(attempt int, delay time.Duration) {
		log.Printf("Ticker reconnecting: attempt=%d, delay=%.2fs", attempt, delay.Seconds())
	})

	s.ticker.OnNoReconnect(func(attempt int) {
		log.Printf("Ticker max reconnect attempts reached: attempt=%d", attempt)
		errChan <- fmt.Errorf("ticker max reconnect attempts reached: %d", attempt)
	})

	s.ticker.OnTick(func(tick kiteticker.Tick) {
		if err := s.sendTickData(c, tick); err != nil {
			log.Printf("Error sending tick data: %v", err)
		}
	})
}

func (s *Service) sendTickData(c echo.Context, tick kiteticker.Tick) error {
	select {
	case <-s.clientClosed:
		return fmt.Errorf("client connection closed")
	default:
	}

	symbolInfo, ok := s.tokenMap[tick.InstrumentToken]
	if !ok {
		log.Printf("symbolInfo not found for token: %d", tick.InstrumentToken)
		return nil // Skip ticks for unknown symbols
	}

	exchange, tradingsymbol, _ := strings.Cut(symbolInfo, ":")

	tickData := map[string]interface{}{
		"exchange":      exchange,
		"tradingsymbol": tradingsymbol,
		"last_price":    tick.LastPrice,
		"volume":        tick.VolumeTraded,
		"avg_price":     tick.AverageTradePrice,
	}

	jsonData, err := json.Marshal(tickData)
	if err != nil {
		return fmt.Errorf("error marshaling tick data: %w", err)
	}

	// Use a context with timeout for writing and flushing
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before writing data")
	case <-s.clientClosed:
		return fmt.Errorf("client connection closed before writing data")
	default:
		if _, err := c.Response().Write([]byte(fmt.Sprintf("data: %s\n\n", jsonData))); err != nil {
			return fmt.Errorf("error writing to response: %w", err)
		}
		c.Response().Flush()
	}

	return nil
}
