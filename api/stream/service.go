package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	kiteticker "github.com/nsvirk/gokiteticker"
	"github.com/nsvirk/moneybotsapi/services/instrument"
	"gorm.io/gorm"
)

type Service struct {
	instrumentService *instrument.InstrumentService
	ticker            *kiteticker.Ticker
	globalTokenMap    map[uint32]string
	mu                sync.RWMutex
	clients           map[string]*Client
	isConnected       bool
	connectChan       chan struct{}
	subscriptionChan  chan subscriptionRequest
}

type Client struct {
	ID          string
	Instruments []string
	Tokens      []uint32
	TokenMap    map[uint32]string
	Channel     chan<- []byte
}

type subscriptionRequest struct {
	tokens []uint32
	respCh chan error
}

func NewService(db *gorm.DB) *Service {
	s := &Service{
		instrumentService: instrument.NewInstrumentService(db),
		globalTokenMap:    make(map[uint32]string),
		clients:           make(map[string]*Client),
		connectChan:       make(chan struct{}),
		subscriptionChan:  make(chan subscriptionRequest),
	}
	go s.subscriptionHandler()
	return s
}

func (s *Service) RunTickerStream(ctx context.Context, c echo.Context, userId, enctoken string, instruments []string, errChan chan<- error) {
	clientID := c.Response().Header().Get(echo.HeaderXRequestID)
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	tokenToInstrumentMap, tokens, err := s.prepareTokens(instruments)
	if err != nil {
		errChan <- err
		return
	}

	clientChan := make(chan []byte, 100)
	client := &Client{
		ID:          clientID,
		Instruments: instruments,
		Tokens:      tokens,
		TokenMap:    tokenToInstrumentMap,
		Channel:     clientChan,
	}

	s.addClient(client)
	defer s.removeClient(clientID)

	s.mu.Lock()
	if s.ticker == nil {
		if err := s.initTicker(userId, enctoken); err != nil {
			s.mu.Unlock()
			errChan <- fmt.Errorf("failed to initialize ticker: %v", err)
			return
		}
	}
	s.mu.Unlock()

	if err := s.waitForConnection(ctx); err != nil {
		errChan <- fmt.Errorf("connection timeout: %v", err)
		return
	}

	if err := s.subscribeClientTokens(client.Tokens); err != nil {
		errChan <- fmt.Errorf("failed to subscribe client tokens: %v", err)
		return
	}

	// Set headers for SSE
	c.Response().Header().Set(echo.HeaderContentType, "text/event-stream")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderConnection, "keep-alive")
	c.Response().WriteHeader(http.StatusOK)

	// Send an initial message to establish the connection
	if _, err := c.Response().Write([]byte("data: connected\n\n")); err != nil {
		log.Printf("Error writing initial message: %v", err)
		return
	}
	c.Response().Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case data := <-clientChan:
			if _, err := c.Response().Write(data); err != nil {
				log.Printf("Error writing to client %s: %v", clientID, err)
				return
			}
			c.Response().Flush()
		case <-ticker.C:
			// Send a keep-alive message every 30 seconds
			if _, err := c.Response().Write([]byte(": keep-alive\n\n")); err != nil {
				log.Printf("Error writing keep-alive: %v", err)
				return
			}
			c.Response().Flush()
		}
	}
}

func (s *Service) subscriptionHandler() {
	for req := range s.subscriptionChan {
		err := s.ticker.Subscribe(req.tokens)
		if err == nil {
			err = s.ticker.SetMode(kiteticker.ModeFull, req.tokens)
		}
		req.respCh <- err
	}
}

func (s *Service) subscribeClientTokens(tokens []uint32) error {
	respCh := make(chan error)
	s.subscriptionChan <- subscriptionRequest{tokens: tokens, respCh: respCh}
	return <-respCh
}

func (s *Service) waitForConnection(ctx context.Context) error {
	s.mu.RLock()
	if s.isConnected {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	select {
	case <-s.connectChan:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("connection timeout")
	}
}

func (s *Service) addClient(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
	for token, instrument := range client.TokenMap {
		s.globalTokenMap[token] = instrument
	}
}

func (s *Service) removeClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if client, ok := s.clients[clientID]; ok {
		close(client.Channel)
		delete(s.clients, clientID)
	}
	s.cleanupGlobalTokenMap()
}

func (s *Service) cleanupGlobalTokenMap() {
	newGlobalTokenMap := make(map[uint32]string)
	for _, client := range s.clients {
		for token, instrument := range client.TokenMap {
			newGlobalTokenMap[token] = instrument
		}
	}
	s.globalTokenMap = newGlobalTokenMap
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
	s.setupCallbacks()
	go s.ticker.Serve()
	return nil
}

func (s *Service) setupCallbacks() {
	s.ticker.OnError(func(err error) {
		log.Printf("Ticker error: %v", err)
	})

	s.ticker.OnClose(func(code int, reason string) {
		log.Printf("Ticker closed: code=%d, reason=%s", code, reason)
		s.mu.Lock()
		s.isConnected = false
		s.connectChan = make(chan struct{}) // Reset connect channel
		s.mu.Unlock()
	})

	s.ticker.OnConnect(func() {
		log.Println("Ticker connected")
		s.mu.Lock()
		s.isConnected = true
		close(s.connectChan)
		s.mu.Unlock()
	})

	s.ticker.OnReconnect(func(attempt int, delay time.Duration) {
		log.Printf("Ticker reconnecting: attempt=%d, delay=%.2fs", attempt, delay.Seconds())
	})

	s.ticker.OnNoReconnect(func(attempt int) {
		log.Printf("Ticker max reconnect attempts reached: attempt=%d", attempt)
	})

	s.ticker.OnTick(func(tick kiteticker.Tick) {
		s.broadcastTick(tick)
	})
}

func (s *Service) broadcastTick(tick kiteticker.Tick) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	symbolInfo, ok := s.globalTokenMap[tick.InstrumentToken]
	if !ok {
		return
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
		log.Printf("Error marshaling tick data: %v", err)
		return
	}

	data := []byte(fmt.Sprintf("data: %s\n\n", jsonData))

	for _, client := range s.clients {
		if _, ok := client.TokenMap[tick.InstrumentToken]; ok {
			select {
			case client.Channel <- data:
			default:
				log.Printf("Skipping slow client: %s", client.ID)
			}
		}
	}
}
