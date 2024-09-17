package services

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/config"

	"github.com/nsvirk/moneybotsapi/services/index"
	"github.com/nsvirk/moneybotsapi/services/instrument"
	"github.com/nsvirk/moneybotsapi/services/session"
	"github.com/nsvirk/moneybotsapi/services/ticker"
	"github.com/nsvirk/moneybotsapi/shared/logger"
	"github.com/nsvirk/moneybotsapi/shared/zaplogger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type CronService struct {
	e                 *echo.Echo
	cfg               *config.Config
	db                *gorm.DB
	redisClient       *redis.Client
	logger            *logger.Logger
	c                 *cron.Cron
	sessionService    *session.SessionService
	instrumentService *instrument.InstrumentService
	indexService      *index.IndexService
	tickerService     *ticker.TickerService
}

func NewCronService(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client) *CronService {
	// Initialize services
	sessionService := session.NewService(db)
	instrumentService := instrument.NewInstrumentService(db)
	indexService := index.NewIndexService(db)
	tickerService := ticker.NewService(db, redisClient)

	cronLogger, err := logger.New(db, "CRON SERVICE")
	if err != nil {
		log.Fatalf("failed to create cron logger: %v", err)
	}

	return &CronService{
		e:                 e,
		cfg:               cfg,
		db:                db,
		redisClient:       redisClient,
		logger:            cronLogger,
		c:                 cron.New(),
		sessionService:    sessionService,
		instrumentService: instrumentService,
		tickerService:     tickerService,
		indexService:      indexService,
	}
}

func (cs *CronService) Start() {
	// Log the initialization to logger
	zaplogger.Info(config.SingleLine)
	zaplogger.Info("Initializing CronService")
	zaplogger.Info(config.SingleLine)

	// Add your scheduled jobs here
	cs.addScheduledJob("API Instruments UPDATE job", cs.apiInstrumentsUpdateJob, "0 8 * * 1-5")      // Once at 08:00am, Mon-Fri
	cs.addScheduledJob("API Indices UPDATE job", cs.apiIndicesUpdateJob, "1 8 * * 1-5")              // Once at 08:01am, Mon-Fri
	cs.addScheduledJob("TickerInstruments UPDATE job", cs.tickerInstrumentsUpdateJob, "2 8 * * 1-5") // Once at 08:02am, Mon-Fri

	// Ticker starts at 9:00am and stops at 11:45pm - Covers NSE and MCX trading hours
	cs.addScheduledJob("Ticker START job", cs.tickerStartJob, "0 9 * * 1-5") // Once at 09:00am, Mon-Fri
	cs.addScheduledJob("Ticker STOP job", cs.tickerStopJob, "45 23 * * 1-5") // Once at 11:45pm, Mon-Fri

	// Add your startup jobs here
	cs.addStartupJob("TickerData TRUNCATE job", cs.tickerDataTruncateJob, 1*time.Second)
	cs.addStartupJob("ApiInstruments UPDATE job", cs.apiInstrumentsUpdateJob, 3*time.Second)
	cs.addStartupJob("ApiIndices UPDATE job", cs.apiIndicesUpdateJob, 8*time.Second)
	cs.addStartupJob("TickerInstruments UPDATE job", cs.tickerInstrumentsUpdateJob, 30*time.Second)
	cs.addStartupJob("Ticker START job", cs.tickerStartJob, 40*time.Second)

	// Log the initialization to database
	cs.logger.Info("Initializing CronService", map[string]interface{}{
		"jobs": len(cs.c.Entries()),
	})

	cs.c.Start()
}

func (cs *CronService) addScheduledJob(name string, job func(), schedule string) {
	_, err := cs.c.AddFunc(schedule, func() {
		cs.logger.Info("Executing SCHEDULED job", map[string]interface{}{
			"job":  name,
			"time": time.Now().Format("15:04:05"),
		})
		zaplogger.Info("")
		zaplogger.Info("Executing SCHEDULED job: ")
		zaplogger.Info("  >> job  : " + name)
		zaplogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		zaplogger.Info("")
		job()
	})
	if err != nil {
		cs.logger.Error("Failed to SCHEDULE job", map[string]interface{}{
			"job":   name,
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Failed to SCHEDULE job")
		zaplogger.Error("  >> job  : " + name)
		zaplogger.Error("  >> error: " + err.Error())
		zaplogger.Info("")
		return
	}
	zaplogger.Info("  * Queued SCHEDULED job: " + name)
}

func (cs *CronService) addStartupJob(name string, job func(), delay time.Duration) {
	go func() {
		time.Sleep(delay)
		cs.logger.Info("Executing STARTUP job", map[string]interface{}{
			"job":  name,
			"time": time.Now().Format("15:04:05"),
		})
		zaplogger.Info("")
		zaplogger.Info("Executing STARTUP job: ")
		zaplogger.Info("  >> job  : " + name)
		zaplogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		zaplogger.Info("")
		zaplogger.Info(config.SingleLine)
		job()
	}()
	zaplogger.Info("  * Queued STARTUP job : " + name)
}

func (cs *CronService) apiInstrumentsUpdateJob() {

	totalInserted, err := cs.instrumentService.UpdateInstruments()
	if err != nil {
		cs.logger.Error("ApiInstruments UPDATE job failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("ApiInstruments UPDATE job failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("ApiInstruments UPDATE job successful", map[string]interface{}{
		"total_inserted": totalInserted,
	})
	zaplogger.Info("")
	zaplogger.Info("ApiInstruments UPDATE job successful")
	zaplogger.Info("  * total_inserted    : " + strconv.Itoa(totalInserted))
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)
}

func (cs *CronService) apiIndicesUpdateJob() {

	totalInserted, err := cs.indexService.UpdateNSEIndices()
	if err != nil {
		cs.logger.Error("ApiIndices UPDATE job failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("ApiIndices UPDATE job failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("ApiIndices UPDATE job successful", map[string]interface{}{
		"total_inserted": totalInserted,
	})
	zaplogger.Info("")
	zaplogger.Info("ApiIndices UPDATE job successful")
	zaplogger.Info("  * total_inserted    : " + strconv.FormatInt(totalInserted, 10))
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)
}

func (cs *CronService) tickerStartJob() {

	// Generate the session
	userId := cs.cfg.KitetickerUserID
	password := cs.cfg.KitetickerPassword
	totpSecret := cs.cfg.KitetickerTotpSecret

	sessionData, err := cs.sessionService.GenerateSession(userId, password, totpSecret)
	if err != nil {
		cs.logger.Error("Ticker START job failed [GenerateSession]", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Ticker START job failed [GenerateSession]")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")

		return
	}
	cs.logger.Info("Ticker START job successful [GenerateSession]", map[string]interface{}{
		"user_id":    sessionData.UserID,
		"login_time": sessionData.LoginTime,
	})
	zaplogger.Info("")
	zaplogger.Info("Ticker START job successful [GenerateSession]")
	zaplogger.Info("  * user_id    : " + sessionData.UserID)
	zaplogger.Info("  * login_time : " + sessionData.LoginTime)
	zaplogger.Info("")

	// Start the ticker
	err = cs.tickerService.Start(sessionData.UserID, sessionData.Enctoken)
	if err != nil {
		cs.logger.Error("Ticker START job failed [Ticker]", map[string]interface{}{
			"error": err.Error(),
		})
		//
		zaplogger.Info("")
		zaplogger.Error("Ticker START job failed [Ticker]")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("Ticker START job successful [Ticker]", nil)
	//
	zaplogger.Info("")
	zaplogger.Info("Ticker START job successful [Ticker]")
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)

}

func (cs *CronService) tickerStopJob() {

	// Stop the ticker
	userId := cs.cfg.KitetickerUserID
	err := cs.tickerService.Stop(userId)
	if err != nil {
		cs.logger.Error("Ticker STOP job failed [Ticker]", map[string]interface{}{
			"error": err.Error(),
		})
		//
		zaplogger.Info("")
		zaplogger.Error("Ticker STOP job failed [Ticker]")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("Ticker STOP job successful [Ticker]", nil)
	//
	zaplogger.Info("")
	zaplogger.Info("Ticker STOP job successful [Ticker]")
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)

}

func (cs *CronService) tickerDataTruncateJob() {
	// Truncate the table
	if err := cs.tickerService.TruncateTickerData(); err != nil {
		cs.logger.Error("TickerData TRUNCATE job failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("TickerData TRUNCATE job failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}
	cs.logger.Info("TickerData TRUNCATE job successful:", nil)
	//
	zaplogger.Info("")
	zaplogger.Info("TickerData TRUNCATE job successful")
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)
}

func (cs *CronService) tickerInstrumentsUpdateJob() {
	userID := cs.cfg.KitetickerUserID
	totalInserted := 0

	// Truncate the table
	count, err := cs.tickerService.TruncateTickerInstruments()
	if err != nil {
		cs.logger.Error("TickerInstruments TRUNCATE job failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Error("TickerInstruments TRUNCATE job failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}
	//
	cs.logger.Info("TickerInstruments TRUNCATE job successful", map[string]interface{}{
		"count": count,
	})
	//
	zaplogger.Info("")
	zaplogger.Info("TickerInstruments TRUNCATE job successful")
	zaplogger.Info("  * count      : " + strconv.FormatInt(count, 10))
	zaplogger.Info("")

	m0NFO, _, _ := cs.tickerService.GetNFOFilterMonths()
	m0NFOFutFilter := "%" + m0NFO + "FUT"
	m0NFONiftyOptFilter := "NIFTY" + m0NFO + "%00_E"
	m0NFOBankNiftyOptFilter := "BANKNIFTY" + m0NFO + "%00_E"

	_, m1NFO, _ := cs.tickerService.GetNFOFilterMonths()
	m1NFOFutFilter := "%" + m1NFO + "FUT"
	m1NFONiftyOptFilter := "NIFTY" + m1NFO + "%00_E"
	m1NFOBankNiftyOptFilter := "BANKNIFTY" + m1NFO + "%00_E"

	_, _, m2NFO := cs.tickerService.GetNFOFilterMonths()
	m2NFOFutFilter := "%" + m2NFO + "FUT"
	m2NFONiftyOptFilter := "NIFTY" + m2NFO + "%00_E"
	m2NFOBankNiftyOptFilter := "BANKNIFTY" + m2NFO + "%00_E"

	// Define instrument queries
	queries := []struct {
		exchange      string
		tradingsymbol string
		expiry        string
		strike        string
		segment       string
		description   string
	}{
		{"NSE", "INDIA VIX", "", "", "", "NSE:INDIA VIX"}, // NSE:INDIA VIX
		{"NSE", "", "", "", "INDICES", "NSE:INDICES"},     // NSE:INDICES - ~78
		{"MCX", "", "", "", "INDICES", "MCX:INDICES"},     // MCX:INDICES - ~10
		{"NFO", "%FUT", "", "", "", "NFO All Futures"},    // NFO All Futures - ~120
		{"MCX", "%FUT", "", "", "", "MCX All Futures"},    // MCX All Futures - ~550
		// {"NFO", "NIFTY%", "", "", "", "NFO NIFTY All Futures & Options"},         // NFO NIFTY All Futures & Options - ~2720
		// {"NFO", "BANKNIFTY%", "", "", "", "NFO BANKNIFTY All Futures & Options"}, // NFO BANKNIFTY All Futures & Options - ~1520
		// {"NFO", "FINNIFTY%", "", "", "", "NFO FINNIFTY All Futures & Options"},   // NFO FINNIFTY All Futures & Options - ~1160

		// NIFTY and BANKNIFTY Options for the next 3 months
		{"NFO", m0NFOFutFilter, "", "", "", "NFO ALL FUT - m0 [" + m0NFO + "]"},
		{"NFO", m0NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m0 [" + m0NFO + "]"},
		{"NFO", m0NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m0 [" + m0NFO + "]"},

		{"NFO", m1NFOFutFilter, "", "", "", "NFO ALL FUT - m1 [" + m1NFO + "]"},
		{"NFO", m1NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m1 [" + m1NFO + "]"},
		{"NFO", m1NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m1 [" + m1NFO + "]"},

		{"NFO", m2NFOFutFilter, "", "", "", "NFO ALL FUT - m2 [" + m2NFO + "]"},
		{"NFO", m2NFONiftyOptFilter, "", "", "", "NFO NIFTY OPT - m2 [" + m2NFO + "]"},
		{"NFO", m2NFOBankNiftyOptFilter, "", "", "", "NFO BANKNIFTY OPT - m2 [" + m2NFO + "]"},
	}

	// Process each query
	for _, q := range queries {

		result, err := cs.tickerService.UpsertQueriedInstruments(userID, q.exchange, q.tradingsymbol, q.expiry, q.strike, q.segment)
		if err != nil {
			zaplogger.Error("TickerInstruments UPSERT for query failed:")
			zaplogger.Error("  * query      : " + q.description)
			zaplogger.Error("  * error      : " + err.Error())
			zaplogger.Info("")
			continue
		}
		totalInserted += result["inserted"].(int) + result["updated"].(int)

		zaplogger.Info("TickerInstruments UPSERT for query successful:")
		zaplogger.Info("  * query      : " + q.description)
		zaplogger.Info("  * queried    : " + strconv.Itoa(result["queried"].(int)))
		zaplogger.Info("  * inserted   : " + strconv.Itoa(result["inserted"].(int)))
		zaplogger.Info("  * updated    : " + strconv.Itoa(result["updated"].(int)))
		zaplogger.Info("  * total      : " + strconv.Itoa(totalInserted))
		zaplogger.Info("")
	}

	// Add provision for upserting selected indices
	indices := []string{"NSE:NIFTY 200", "NSE:NIFTY BANK"}
	for _, indexName := range indices {

		indexInstruments, err := cs.indexService.GetNSEIndexInstruments(indexName)
		if err != nil {
			zaplogger.Error("TickerInstruments FETCH for index failed:")
			zaplogger.Error("  * index : " + indexName)
			zaplogger.Error("  * error : " + err.Error())
			zaplogger.Info("")
			continue
		}

		queried, inserted, updated := 0, 0, 0
		failedInstruments := []string{}

		for _, instrument := range indexInstruments {
			parts := strings.SplitN(instrument, ":", 2)
			if len(parts) != 2 {
				failedInstruments = append(failedInstruments, instrument)
				continue
			}
			exchange := parts[0]
			tradingsymbol := parts[1]

			result, err := cs.tickerService.UpsertQueriedInstruments(userID, exchange, tradingsymbol, "", "", "")
			if err != nil {
				failedInstruments = append(failedInstruments, instrument)
				continue
			}

			queried += result["queried"].(int)
			inserted += result["inserted"].(int)
			updated += result["updated"].(int)
		}

		totalInserted += inserted + updated

		// Log the accumulated results for the index
		zaplogger.Info("TickerInstruments UPSERT for index successful:")
		zaplogger.Info("  * index       : " + indexName + " [INDEX]")
		zaplogger.Info("  * instruments : " + strconv.Itoa(len(indexInstruments)))
		zaplogger.Info("  * queried     : " + strconv.Itoa(queried))
		zaplogger.Info("  * inserted    : " + strconv.Itoa(inserted))
		zaplogger.Info("  * updated     : " + strconv.Itoa(updated))
		zaplogger.Info("  * total       : " + strconv.Itoa(totalInserted))

		if len(failedInstruments) > 0 {
			zaplogger.Error("  * failed      : " + strconv.Itoa(len(failedInstruments)))
			zaplogger.Error("  * failed instruments:")
			for _, failedInstr := range failedInstruments {
				zaplogger.Error("    - " + failedInstr)
			}
		}
		zaplogger.Info("")
	}
	zaplogger.Info("")
	zaplogger.Info("TickerInstruments UPDATE job successful")
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)

	// Log the ticker instrument count
	totalTickerInstruments, err := cs.tickerService.GetTickerInstrumentCount(userID)
	if err != nil {
		cs.logger.Error("TickerInstruments COUNT job failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Error("TickerInstruments COUNT job failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("TickerInstruments COUNT job successful", map[string]interface{}{
		"total_ticker_instruments": totalTickerInstruments,
	})
	zaplogger.Info("")
	zaplogger.Info("TickerInstruments COUNT job successful")
	zaplogger.Info("  * total_ticker_instruments    : " + strconv.Itoa(int(totalTickerInstruments)))
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)

}
