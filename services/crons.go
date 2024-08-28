package services

import (
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nsvirk/moneybotsapi/config"

	"github.com/nsvirk/moneybotsapi/api/instrument"
	"github.com/nsvirk/moneybotsapi/api/session"
	"github.com/nsvirk/moneybotsapi/api/ticker"
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
	sessionService    *session.Service
	instrumentService *instrument.InstrumentService
	tickerService     *ticker.Service
	indexService      *instrument.IndexService
}

func NewCronService(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client, logger *logger.Logger) *CronService {
	// Initialize services
	sessionService := session.NewService(db)
	tickerService := ticker.NewService(db, redisClient)
	instrumentService := instrument.NewInstrumentService(db)
	indexService := instrument.NewIndexService()

	return &CronService{
		e:                 e,
		cfg:               cfg,
		db:                db,
		redisClient:       redisClient,
		logger:            logger,
		c:                 cron.New(),
		sessionService:    sessionService,
		instrumentService: instrumentService,
		tickerService:     tickerService,
		indexService:      indexService,
	}
}

func (cs *CronService) Start() {
	zaplogger.Info(config.SingleLine)
	zaplogger.Info("Initializing CronService")

	// Add your scheduled jobs here
	cs.addScheduledJob("API Instruments Update Job", cs.apiInstrumentsUpdateJob, "45 7 * * 1-5")       // Once at 07:45am, Mon-Fri
	cs.addScheduledJob("Ticker Instruments Update Job", cs.tickerInstrumentsUpdateJob, "55 7 * * 1-5") // Once at 07:55am, Mon-Fri
	// cs.addScheduledJob("Ticker Restart Job", cs.tickerRestartJob, "30 8-23 * * 1-5")                   // Every half hour from 8am to 11pm, Mon-Fri

	// Add your startup jobs here
	cs.addStartupJob("API Instruments Update Job", cs.apiInstrumentsUpdateJob, 1*time.Second)
	cs.addStartupJob("Ticker Instruments Update Job", cs.tickerInstrumentsUpdateJob, 5*time.Second)
	cs.addStartupJob("Ticker Data Truncate Job", cs.tickerDataTruncateJob, 5*time.Second)
	cs.addStartupJob("Ticker Start Job", cs.tickerStartJob, 15*time.Second)

	cs.c.Start()
}

func (cs *CronService) addScheduledJob(name string, job func(), schedule string) {
	_, err := cs.c.AddFunc(schedule, func() {
		cs.logger.Info("Executing scheduled job", map[string]interface{}{
			"job":  name,
			"time": time.Now().Format("15:04:05"),
		})
		zaplogger.Info("")
		zaplogger.Info("Executing scheduled job: ")
		zaplogger.Info("  >> job  : " + name)
		zaplogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		zaplogger.Info("")
		job()
	})
	if err != nil {
		cs.logger.Error("Failed to schedule job", map[string]interface{}{
			"job":   name,
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Failed to schedule job")
		zaplogger.Error("  >> job  : " + name)
		zaplogger.Error("  >> error: " + err.Error())
		zaplogger.Info("")
		return
	}
	zaplogger.Info("  * Scheduled job added: " + name)
}

func (cs *CronService) addStartupJob(name string, job func(), delay time.Duration) {
	go func() {
		time.Sleep(delay)
		cs.logger.Info("Executing startup job", map[string]interface{}{
			"job":  name,
			"time": time.Now().Format("15:04:05"),
		})
		zaplogger.Info("")
		zaplogger.Info("Executing startup job: ")
		zaplogger.Info("  >> job  : " + name)
		zaplogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		zaplogger.Info("")
		job()
	}()
	zaplogger.Info("  * Startup job queued : " + name)
}

func (cs *CronService) apiInstrumentsUpdateJob() {

	totalInserted, err := cs.instrumentService.UpdateInstruments()
	if err != nil {
		cs.logger.Error("Failed to update instruments", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Failed to update instruments")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("Instruments update successful", map[string]interface{}{
		"total_inserted": totalInserted,
	})
	zaplogger.Info("")
	zaplogger.Info("Instruments update successful")
	zaplogger.Info("  * total_inserted    : " + strconv.Itoa(totalInserted))
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)
}

func (cs *CronService) tickerStartJob() {

	// Create a login request
	loginRequest := session.LoginRequest{
		UserID:     cs.cfg.KitetickerUserID,
		Password:   cs.cfg.KitetickerPassword,
		TOTPSecret: cs.cfg.KitetickerTotpSecret,
	}

	// Generate or fetch the session
	sessionData, err := cs.sessionService.GenerateSession(loginRequest)
	if err != nil {
		cs.logger.Error("Ticker generate session failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Ticker generate session failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")

		return
	}
	cs.logger.Info("Ticker generate session successful", map[string]interface{}{
		"user_id":    sessionData.UserID,
		"login_time": sessionData.LoginTime,
	})
	zaplogger.Info("Ticker generate session successful")
	zaplogger.Info("  * user_id    : " + sessionData.UserID)
	zaplogger.Info("  * login_time : " + sessionData.LoginTime)
	zaplogger.Info("")

	// Start the ticker
	err = cs.tickerService.Start(sessionData.UserID, sessionData.Enctoken)
	if err != nil {
		cs.logger.Error("Ticker start failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("Ticker start failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}

	cs.logger.Info("Ticker start successful", nil)
	zaplogger.Info("")
	zaplogger.Info("Ticker start successful")
	zaplogger.Info("")
	zaplogger.Info(config.SingleLine)

}

func (cs *CronService) tickerDataTruncateJob() {
	zaplogger.Info("Starting Ticker Data Truncate Job")
	zaplogger.Info("")

	// Truncate the table
	if err := cs.tickerService.TruncateTickerData(); err != nil {
		cs.logger.Error("TickerData truncate Failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Info("")
		zaplogger.Error("TickerData truncate Failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}
	cs.logger.Info("TickerData truncate successful", nil)
	zaplogger.Info("TickerData truncate successful")
	zaplogger.Info("")

	cs.logger.Info("Ticker Data Truncate Job Completed", nil)
	zaplogger.Info("Ticker Data Truncate Job Completed")
	zaplogger.Info(config.SingleLine)
}

func (cs *CronService) tickerInstrumentsUpdateJob() {
	zaplogger.Info("Starting Ticker Instruments Update Job")
	zaplogger.Info("")

	// Truncate the table
	if err := cs.tickerService.TruncateTickerInstruments(); err != nil {
		cs.logger.Error("TickerInstruments truncate Failed", map[string]interface{}{
			"error": err.Error(),
		})
		zaplogger.Error("TickerInstruments truncate Failed")
		zaplogger.Error("  * error    : " + err.Error())
		zaplogger.Info("")
		return
	}
	cs.logger.Info("TickerInstruments truncate successful", nil)
	zaplogger.Info("TickerInstruments truncate successful")
	zaplogger.Info("")

	// m0NFO, _, _ := cs.tickerService.GetNFOFilterMonths()
	// m0NFOFutFilter := "%" + m0NFO + "FUT"
	// m0NFONiftyOptFilter := "NIFTY" + m0NFO + "%00_E"
	// m0NFOBankNiftyOptFilter := "BANKNIFTY" + m0NFO + "%00_E"

	// _, m1NFO, _ := cs.tickerService.GetNFOFilterMonths()
	// m1NFOFutFilter := "%" + m1NFO + "FUT"
	// m1NFONiftyOptFilter := "NIFTY" + m1NFO + "%00_E"
	// m1NFOBankNiftyOptFilter := "BANKNIFTY" + m1NFO + "%00_E"

	// _, _, m2NFO := cs.tickerService.GetNFOFilterMonths()
	// m2NFOFutFilter := "%" + m2NFO + "FUT"
	// m2NFONiftyOptFilter := "NIFTY" + m2NFO + "%00_E"
	// m2NFOBankNiftyOptFilter := "BANKNIFTY" + m2NFO + "%00_E"

	// Define instrument queries
	queries := []struct {
		exchange      string
		tradingsymbol string
		expiry        string
		strike        string
		description   string
	}{
		{"NSE", "INDIA VIX", "", "", "NSE:INDIA VIX"},                        // NSE:INDIA VIX
		{"NSE", "NIFTY 50", "", "", "NSE:NIFTY 50"},                          // NSE:NIFTY 50 - ~1
		{"NFO", "%FUT", "", "", "NFO All Futures"},                           // NFO All Futures - ~120
		{"NFO", "NIFTY%", "", "", "NFO NIFTY All Futures & Options"},         // NFO NIFTY All Futures & Options - ~2720
		{"NFO", "BANKNIFTY%", "", "", "NFO BANKNIFTY All Futures & Options"}, // NFO BANKNIFTY All Futures & Options - ~1520
		{"NFO", "FINNIFTY%", "", "", "NFO FINNIFTY All Futures & Options"},   // NFO FINNIFTY All Futures & Options - ~1160
		{"MCX", "%FUT", "", "", "MCX All Futures"},                           // MCX All Futures - ~550

		// NIFTY and BANKNIFTY Options for the next 3 months
		// {"NFO", m0NFOFutFilter, "", "", "NFO ALL FUT - m0 [" + m0NFO + "]"},
		// {"NFO", m0NFONiftyOptFilter, "", "", "NFO NIFTY OPT - m0 [" + m0NFO + "]"},
		// {"NFO", m0NFOBankNiftyOptFilter, "", "", "NFO BANKNIFTY OPT - m0 [" + m0NFO + "]"},

		// {"NFO", m1NFOFutFilter, "", "", "NFO ALL FUT - m1 [" + m1NFO + "]"},
		// {"NFO", m1NFONiftyOptFilter, "", "", "NFO NIFTY OPT - m1 [" + m1NFO + "]"},
		// {"NFO", m1NFOBankNiftyOptFilter, "", "", "NFO BANKNIFTY OPT - m1 [" + m1NFO + "]"},

		// {"NFO", m2NFOFutFilter, "", "", "NFO ALL FUT - m2 [" + m2NFO + "]"},
		// {"NFO", m2NFONiftyOptFilter, "", "", "NFO NIFTY OPT - m2 [" + m2NFO + "]"},
		// {"NFO", m2NFOBankNiftyOptFilter, "", "", "NFO BANKNIFTY OPT - m2 [" + m2NFO + "]"},
	}

	// Process each query
	for _, q := range queries {
		result, err := cs.tickerService.UpsertQueriedInstruments(q.exchange, q.tradingsymbol, q.expiry, q.strike)
		if err != nil {
			zaplogger.Error("Failed to upsert queried instruments:")
			zaplogger.Error("  * query      : " + q.description)
			zaplogger.Error("  * error      : " + err.Error())
			zaplogger.Info("")
			continue
		}

		zaplogger.Info("TickerInstruments upsert results for query:")
		zaplogger.Info("  * query      : " + q.description)
		zaplogger.Info("  * queried    : " + strconv.Itoa(result["queried"].(int)))
		zaplogger.Info("  * added      : " + strconv.Itoa(result["added"].(int)))
		zaplogger.Info("  * updated    : " + strconv.Itoa(result["updated"].(int)))
		zaplogger.Info("  * total      : " + strconv.FormatInt(result["total"].(int64), 10))
		zaplogger.Info("")
	}

	// Add provision for upserting selected indices
	indices := []string{"NSE:NIFTY 500", "NSE:NIFTY BANK"}
	for _, indexName := range indices {

		instruments, err := cs.indexService.FetchIndexInstrumentsList(indexName)
		if err != nil {
			zaplogger.Error("Failed to fetch index instruments:")
			zaplogger.Error("  * index : " + indexName)
			zaplogger.Error("  * error : " + err.Error())
			zaplogger.Info("")
			continue
		}

		totalQueried, totalAdded, totalUpdated := 0, 0, 0
		var totalInstruments int64 = 0
		failedInstruments := []string{}

		for _, instr := range instruments {
			parts := strings.SplitN(instr, ":", 2)
			if len(parts) != 2 {
				failedInstruments = append(failedInstruments, instr)
				continue
			}

			result, err := cs.tickerService.UpsertQueriedInstruments(parts[0], parts[1], "", "")
			if err != nil {
				failedInstruments = append(failedInstruments, instr)
				continue
			}

			totalQueried += result["queried"].(int)
			totalAdded += result["added"].(int)
			totalUpdated += result["updated"].(int)
			totalInstruments = result["total"].(int64)
		}

		// Log the accumulated results for the index
		zaplogger.Info("TickerInstruments upsert results for index:")
		zaplogger.Info("  * index       : " + indexName + " [INDEX]")
		zaplogger.Info("  * instruments : " + strconv.Itoa(len(instruments)))
		zaplogger.Info("  * queried     : " + strconv.Itoa(totalQueried))
		zaplogger.Info("  * added       : " + strconv.Itoa(totalAdded))
		zaplogger.Info("  * updated     : " + strconv.Itoa(totalUpdated))
		zaplogger.Info("  * total       : " + strconv.FormatInt(totalInstruments, 10))

		if len(failedInstruments) > 0 {
			zaplogger.Error("  * failed      : " + strconv.Itoa(len(failedInstruments)))
			zaplogger.Error("  * failed instruments:")
			for _, failedInstr := range failedInstruments {
				zaplogger.Error("    - " + failedInstr)
			}
		}
		zaplogger.Info("")
	}

	zaplogger.Info("Ticker Instruments Update Job Completed")
	zaplogger.Info(config.SingleLine)
}
