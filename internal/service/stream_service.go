// Package service contains the service layer for the Moneybots API
package service

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
	"github.com/nsvirk/moneybotsapi/pkg/utils/logger"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"

	"gorm.io/gorm"
)

// StreamClient is a client that is subscribed to the stream
type StreamClient struct {
	ID          string
	Instruments []string
	Tokens      []uint32
	TokenMap    map[uint32]string
	Channel     chan<- []byte
}

// StreamSubscriptionRequest is a request to subscribe to a list of tokens
type StreamSubscriptionRequest struct {
	tokens []uint32
	respCh chan error
}

// StreamService is the service for the stream API
type StreamService struct {
	instrumentService *InstrumentService
	ticker            *kiteticker.Ticker
	globalTokenMap    map[uint32]string
	mu                sync.RWMutex
	clients           map[string]*StreamClient
	isConnected       bool
	connectChan       chan struct{}
	subscriptionChan  chan StreamSubscriptionRequest
	logger            *logger.Logger
}

// NewStreamService creates a new service for the stream API
func NewStreamService(db *gorm.DB) *StreamService {
	logger, err := logger.New(db, "STREAM SERVICE")
	if err != nil {
		zaplogger.Error("failed to create stream logger", zaplogger.Fields{"error": err})
	}

	s := &StreamService{
		instrumentService: NewInstrumentService(db),
		globalTokenMap:    make(map[uint32]string),
		clients:           make(map[string]*StreamClient),
		connectChan:       make(chan struct{}),
		subscriptionChan:  make(chan StreamSubscriptionRequest),
		logger:            logger,
	}
	go s.subscriptionHandler()
	return s
}

// RunTickerStream runs the ticker stream for the given client
func (s *StreamService) RunTickerStream(ctx context.Context, c echo.Context, userId, enctoken string, instruments []string, errChan chan<- error) {
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
	client := &StreamClient{
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

// subscriptionHandler handles the subscription requests
func (s *StreamService) subscriptionHandler() {
	for req := range s.subscriptionChan {
		err := s.ticker.Subscribe(req.tokens)
		if err == nil {
			err = s.ticker.SetMode(kiteticker.ModeFull, req.tokens)
		}
		req.respCh <- err
	}
}

// subscribeClientTokens subscribes the client to the given tokens
func (s *StreamService) subscribeClientTokens(tokens []uint32) error {
	respCh := make(chan error)
	s.subscriptionChan <- StreamSubscriptionRequest{tokens: tokens, respCh: respCh}
	return <-respCh
}

// waitForConnection waits for the ticker to connect
func (s *StreamService) waitForConnection(ctx context.Context) error {
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

// addClient adds a client to the service
func (s *StreamService) addClient(client *StreamClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client.ID] = client
	for token, instrument := range client.TokenMap {
		s.globalTokenMap[token] = instrument
	}
}

// removeClient removes a client from the service
func (s *StreamService) removeClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if client, ok := s.clients[clientID]; ok {
		close(client.Channel)
		delete(s.clients, clientID)
	}
	s.cleanupGlobalTokenMap()
}

// cleanupGlobalTokenMap cleans up the global token map
func (s *StreamService) cleanupGlobalTokenMap() {
	newGlobalTokenMap := make(map[uint32]string)
	for _, client := range s.clients {
		for token, instrument := range client.TokenMap {
			newGlobalTokenMap[token] = instrument
		}
	}
	s.globalTokenMap = newGlobalTokenMap
}

// prepareTokens prepares the tokens for the given instruments
func (s *StreamService) prepareTokens(strInstruments []string) (map[uint32]string, []uint32, error) {
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

// initTicker initializes the ticker
func (s *StreamService) initTicker(userId, enctoken string) error {
	s.ticker = kiteticker.New(userId, enctoken)
	if s.ticker == nil {
		return fmt.Errorf("failed to create ticker: returned nil")
	}
	s.setupCallbacks()
	go s.ticker.Serve()
	return nil
}

// setupCallbacks sets up the callbacks for the ticker
func (s *StreamService) setupCallbacks() {
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

// broadcastTick broadcasts the tick to all clients
func (s *StreamService) broadcastTick(tick kiteticker.Tick) {
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
