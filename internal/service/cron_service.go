// Package service contains the service layer for the Moneybots API
package service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/internal/config"
	"github.com/nsvirk/moneybotsapi/pkg/utils/zaplogger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// CronService is the service for the cron jobs
type CronService struct {
	e                 *echo.Echo
	cfg               *config.Config
	db                *gorm.DB
	redisClient       *redis.Client
	c                 *cron.Cron
	sessionService    *SessionService
	instrumentService *InstrumentService
	indexService      *IndexService
	tickerService     *TickerService
}

// NewCronService creates a new CronService
func NewCronService(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client) *CronService {
	// Initialize services
	sessionService := NewSessionService(db)
	instrumentService := NewInstrumentService(db)
	indexService := NewIndexService(db)
	tickerService := NewTickerService(db, redisClient)

	return &CronService{
		e:                 e,
		cfg:               cfg,
		db:                db,
		redisClient:       redisClient,
		c:                 cron.New(),
		sessionService:    sessionService,
		instrumentService: instrumentService,
		tickerService:     tickerService,
		indexService:      indexService,
	}
}

// Start starts the cron service
func (cs *CronService) Start() {
	// Log the initialization to logger
	zaplogger.Info("Initializing CronService")

	// ------------------------------------------------------------
	// Add your SCHEDULED jobs here
	// ------------------------------------------------------------
	cs.addScheduledJob("API Instruments UPDATE Job", cs.apiInstrumentsUpdateJob, "0 8 * * 1-5")      // Once at 08:00am, Mon-Fri
	cs.addScheduledJob("API Indices UPDATE Job", cs.apiIndicesUpdateJob, "1 8 * * 1-5")              // Once at 08:01am, Mon-Fri
	cs.addScheduledJob("TickerInstruments UPDATE Job", cs.tickerInstrumentsUpdateJob, "2 8 * * 1-5") // Once at 08:02am, Mon-Fri
	cs.addScheduledJob("Ticker START Job", cs.tickerStartJob, "55 8	* * 1-5")                        // Once at 08:55am, Mon-Fri
	cs.addScheduledJob("Ticker STOP Job", cs.tickerStopJob, "59 23 * * 1-5")                         // Once at 11:59pm, Mon-Fri

	// ------------------------------------------------------------
	// Add your STARTUP jobs here
	// ------------------------------------------------------------
	cs.addStartupJob("API Instruments UPDATE Job", cs.apiInstrumentsUpdateJob, 1*time.Second)
	cs.addStartupJob("API Indices UPDATE Job", cs.apiIndicesUpdateJob, 5*time.Second)
	cs.addStartupJob("TickerInstruments UPDATE Job", cs.tickerInstrumentsUpdateJob, 19*time.Second)
	cs.addStartupJob("TickerData TRUNCATE Job", cs.tickerDataTruncateJob, 25*time.Second)
	cs.addStartupJob("Ticker START Job", cs.tickerStartJob, 28*time.Second)
	// ------------------------------------------------------------

	cs.c.Start()
}

// addStartupJob adds a startup job to the cron service
func (cs *CronService) addStartupJob(name string, job func(), delay time.Duration) {
	go func() {
		time.Sleep(delay)
		zaplogger.Info("STARTED STARTUP job", zaplogger.Fields{
			"job": name,
		})
		job()
		zaplogger.Info("COMPLETED STARTUP job", zaplogger.Fields{
			"job": name,
		})
	}()
	zaplogger.Info("QUEUED STARTUP job", zaplogger.Fields{
		"job": name,
	})
}

func (cs *CronService) addScheduledJob(name string, job func(), schedule string) {
	_, err := cs.c.AddFunc(schedule, func() {
		zaplogger.Info("STARTED SCHEDULED JOB", zaplogger.Fields{
			"job": name,
		})
		job()
		zaplogger.Info("COMPLETED SCHEDULED JOB", zaplogger.Fields{
			"job": name,
		})
	})
	if err != nil {
		zaplogger.Error("FAILED TO QUEUE SCHEDULED JOB", zaplogger.Fields{
			"job":   name,
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info("QUEUED SCHEDULED job", zaplogger.Fields{
		"job": name,
	})
}

// apiInstrumentsUpdateJob updates the instruments from the API
func (cs *CronService) apiInstrumentsUpdateJob() {
	jobName := "API Instruments UPDATE Job "

	rowsInserted, err := cs.instrumentService.UpdateInstruments()
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"rows_inserted": strconv.FormatInt(rowsInserted, 10),
	})
}

func (cs *CronService) apiIndicesUpdateJob() {
	jobName := "API Indices UPDATE Job "
	rowsInserted, err := cs.indexService.UpdateIndices()
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"rows_inserted": strconv.FormatInt(rowsInserted, 10),
	})
}

// tickerStartJob starts the ticker
func (cs *CronService) tickerStartJob() {
	jobName := "Ticker START Job "
	// Generate the session
	userId := cs.cfg.KitetickerUserID
	password := cs.cfg.KitetickerPassword
	totpSecret := cs.cfg.KitetickerTotpSecret

	sessionData, err := cs.sessionService.GenerateSession(userId, password, totpSecret)
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"step":        "GenerateSession",
			"user_id":     userId,
			"password":    password[:2] + "..." + password[len(password)-2:],
			"totp_secret": totpSecret[:8] + "..." + totpSecret[len(totpSecret)-8:],
			"error":       err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"step":       "GenerateSession",
		"user_id":    sessionData.UserId,
		"enctoken":   sessionData.Enctoken[:4] + "..." + sessionData.Enctoken[len(sessionData.Enctoken)-4:],
		"login_time": sessionData.LoginTime,
	})

	// Start the ticker
	err = cs.tickerService.Start(sessionData.UserId, sessionData.Enctoken)
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"step":  "TickerStart",
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"step": "TickerStart",
	})
}

// tickerStopJob stops the ticker
func (cs *CronService) tickerStopJob() {
	jobName := "Ticker STOP Job "
	// Stop the ticker
	userId := cs.cfg.KitetickerUserID
	err := cs.tickerService.Stop(userId)
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"step":  "TickerStop",
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"step": "TickerStop",
	})
}

// tickerDataTruncateJob truncates the ticker data
func (cs *CronService) tickerDataTruncateJob() {
	jobName := "TickerData TRUNCATE Job "
	// Truncate the table
	if err := cs.tickerService.TruncateTickerData(); err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"error": err.Error(),
		})
		return
	}
}

// tickerInstrumentsUpdateJob updates the ticker instruments
func (cs *CronService) tickerInstrumentsUpdateJob() {
	jobName := "TickerInstruments UPDATE Job "
	userId := cs.cfg.KitetickerUserID
	var grandTotalInserted int64 = 0

	// Truncate the table
	truncatedCount, err := cs.tickerService.TruncateTickerInstruments()
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"step":  "TruncateTickerInstruments",
			"error": err.Error(),
		})
		return
	}
	zaplogger.Info(jobName, zaplogger.Fields{
		"step":            "TruncateTickerInstruments",
		"truncated_count": strconv.FormatInt(truncatedCount, 10),
	})

	// -----------------------------------
	// Add Instruments
	// -----------------------------------

	// m0NFO, _, _ := cs.GetNFOFilterMonths()
	// m0NFOFutFilter := "%" + m0NFO + "FUT"
	// m0NFONiftyOptFilter := "NIFTY" + m0NFO + "%00_E"
	// m0NFOBankNiftyOptFilter := "BANKNIFTY" + m0NFO + "%00_E"
	// // m0NFOFinNiftyOptFilter := "FINNIFTY" + m0NFO + "%00_E"
	// // m0NFOFinMidcapNiftyOptFilter := "MIDCPNIFTY" + m0NFO + "%00_E"

	// _, m1NFO, _ := cs.GetNFOFilterMonths()
	// m1NFOFutFilter := "%" + m1NFO + "FUT"
	// m1NFONiftyOptFilter := "NIFTY" + m1NFO + "%00_E"
	// m1NFOBankNiftyOptFilter := "BANKNIFTY" + m1NFO + "%00_E"
	// // m1NFOFinNiftyOptFilter := "FINNIFTY" + m1NFO + "%00_E"
	// // m1NFOFinMidcapNiftyOptFilter := "MIDCPNIFTY" + m1NFO + "%00_E"

	// _, _, m2NFO := cs.GetNFOFilterMonths()
	// m2NFOFutFilter := "%" + m2NFO + "FUT"
	// m2NFONiftyOptFilter := "NIFTY" + m2NFO + "%00_E"
	// m2NFOBankNiftyOptFilter := "BANKNIFTY" + m2NFO + "%00_E"
	// // m2NFOFinNiftyOptFilter := "FINNIFTY" + m2NFO + "%00_E"
	// // m2NFOFinMidcapNiftyOptFilter := "MIDCPNIFTY" + m2NFO + "%00_E"

	// Define instrument queries
	queries := []struct {
		exchange       string
		tradingsymbol  string
		name           string
		expiry         string
		strike         string
		segment        string
		instrumentType string
		description    string
	}{
		{"", "", "", "", "", "INDICES", "", "ALL:INDICES"}, // ALL:INDICES - ~144
		{"NFO", "", "", "", "", "", "FUT", "NFO:FUTURES"},  // NFO All Futures - ~553
		{"MCX", "", "", "", "", "", "FUT", "MCX:FUTURES"},  // MCX All Futures - ~118

		// // NIFTY and BANKNIFTY Options for the next 3 months
		// {"NFO", m0NFOFutFilter, "", "", "", "NFO ALL FUT - m0 [" + m0NFO + "]"},
		// {"NFO", m0NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m0 [" + m0NFO + "]"},
		// {"NFO", m0NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m0 [" + m0NFO + "]"},

		// {"NFO", m1NFOFutFilter, "", "", "", "NFO ALL FUT - m1 [" + m1NFO + "]"},
		// {"NFO", m1NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m1 [" + m1NFO + "]"},
		// {"NFO", m1NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m1 [" + m1NFO + "]"},

		// {"NFO", m2NFOFutFilter, "", "", "", "NFO ALL FUT - m2 [" + m2NFO + "]"},
		// {"NFO", m2NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m2 [" + m2NFO + "]"},
		// {"NFO", m2NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m2 [" + m2NFO + "]"},
	}

	// Process each query
	for _, q := range queries {

		result, err := cs.tickerService.UpsertQueriedInstruments(userId, q.exchange, q.tradingsymbol, q.name, q.expiry, q.strike, q.segment, q.instrumentType)
		if err != nil {
			zaplogger.Error(jobName, zaplogger.Fields{
				"step":  "UpsertQueriedInstruments-Instruments",
				"query": q.description,
				"error": err.Error(),
			})
			continue
		}
		grandTotalInserted += result.Inserted + result.Updated

		zaplogger.Info(q.description+" added", zaplogger.Fields{
			"queried":  result.Queried,
			"inserted": result.Inserted,
			"updated":  result.Updated,
			"total":    result.Total,
		})
	}

	// -----------------------------------
	// Add All Indices
	// -----------------------------------
	indices, err := cs.indexService.GetAllIndexNames()
	if err != nil {
		zaplogger.Error(jobName, zaplogger.Fields{
			"step":  "GetIndexNames",
			"error": err.Error(),
		})
		return
	}

	var idxCount int64 = 0
	var idxQueried, idxInserted, idxUpdated, idxTotal int64 = 0, 0, 0, 0
	for _, indexName := range indices {

		indexInstruments, err := cs.indexService.GetIndexInstruments(indexName)
		if err != nil {
			zaplogger.Error(jobName, zaplogger.Fields{
				"step":  "GetNSEIndexInstruments",
				"index": indexName,
				"error": err.Error(),
			})
			continue
		}

		failedInstruments := []string{}

		idxQueried = 0
		idxInserted = 0
		idxUpdated = 0
		idxTotal = 0
		for _, instrument := range indexInstruments {
			exchange := instrument.Exchange
			tradingsymbol := instrument.Tradingsymbol
			result, err := cs.tickerService.UpsertQueriedInstruments(userId, exchange, tradingsymbol, "", "", "", "", "")
			if err != nil {
				zaplogger.Error(indexName, zaplogger.Fields{
					"step":       "UpsertQueriedInstruments-Indices",
					"instrument": fmt.Sprintf("%s:%s", exchange, tradingsymbol),
					"error":      err.Error(),
				})
				failedInstruments = append(failedInstruments, tradingsymbol)
				continue
			}
			idxQueried += result.Queried
			idxInserted += result.Inserted
			idxUpdated += result.Updated
			idxTotal += result.Total
			idxCount++
		}

		zaplogger.Info(indexName+" added", zaplogger.Fields{
			"queried":  idxQueried,
			"inserted": idxInserted,
			"updated":  idxUpdated,
			"total":    idxTotal,
		})

		grandTotalInserted += idxTotal

		if len(failedInstruments) > 0 {
			zaplogger.Error(jobName, zaplogger.Fields{
				"step":               "UpsertQueriedInstruments-Indices",
				"index":              indexName,
				"error":              "Failed to insert " + strconv.Itoa(len(failedInstruments)) + " instruments",
				"failed_instruments": failedInstruments,
			})
		}
	}

	// Log the ticker instrument count
	totalTickerInstruments, err := cs.tickerService.GetTickerInstrumentCount(userId)
	if err != nil {
		zaplogger.Error(jobName+"FAILED", zaplogger.Fields{
			"step":  "GetTickerInstrumentCount",
			"error": err.Error(),
		})
		return
	}

	zaplogger.Info(jobName, zaplogger.Fields{
		"total_ticker_instruments": strconv.FormatInt(totalTickerInstruments, 10),
	})
}

// GetNFOFilterMonths gets the NFO filter months
func (cs *CronService) GetNFOFilterMonths() (string, string, string) {
	now := time.Now()
	month0 := strings.ToUpper(now.Format("06Jan"))
	month1 := strings.ToUpper(now.AddDate(0, 1, 0).Format("06Jan"))
	month2 := strings.ToUpper(now.AddDate(0, 2, 0).Format("06Jan"))
	return month0, month1, month2
}
