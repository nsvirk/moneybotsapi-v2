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
	"github.com/nsvirk/moneybotsapi/shared/applogger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

type CronService struct {
	e                 *echo.Echo
	cfg               *config.Config
	db                *gorm.DB
	redisClient       *redis.Client
	c                 *cron.Cron
	sessionService    *session.Service
	instrumentService *instrument.InstrumentService
	tickerService     *ticker.Service
	indexService      *instrument.IndexService
}

func NewCronService(e *echo.Echo, cfg *config.Config, db *gorm.DB, redisClient *redis.Client) *CronService {
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
		c:                 cron.New(),
		sessionService:    sessionService,
		instrumentService: instrumentService,
		tickerService:     tickerService,
		indexService:      indexService,
	}
}

func (cs *CronService) Start() {
	applogger.Info(config.SingleLine)
	applogger.Info("Initializing CronService")

	// Add your scheduled jobs here
	cs.addScheduledJob("API Instruments Update Job", cs.apiInstrumentsUpdateJob, "5 8 * * 1-5")        // Once at 08:05am, Mon-Fri
	cs.addScheduledJob("Ticker Instruments Update Job", cs.tickerInstrumentsUpdateJob, "10 8 * * 1-5") // Once at 08:10am, Mon-Fri
	// cs.addScheduledJob("Ticker Restart Job", cs.tickerRestartJob, "30 8-23 * * 1-5")                   // Every half hour from 8am to 11pm, Mon-Fri

	// Add your startup jobs here
	cs.addStartupJob("API Instruments Update Job", cs.apiInstrumentsUpdateJob, 1*time.Second)
	cs.addStartupJob("Ticker Instruments Update Job", cs.tickerInstrumentsUpdateJob, 5*time.Second)
	cs.addStartupJob("Ticker Data Truncate Job", cs.tickerDataTruncateJob, 10*time.Second)
	cs.addStartupJob("Ticker Start Job", cs.tickerStartJob, 25*time.Second)

	cs.c.Start()
}

func (cs *CronService) addScheduledJob(name string, job func(), schedule string) {
	_, err := cs.c.AddFunc(schedule, func() {
		applogger.Info("")
		applogger.Info("Executing scheduled job: ")
		applogger.Info("  >> job  : " + name)
		applogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		applogger.Info("")
		job()
	})
	if err != nil {
		applogger.Info("")
		applogger.Error("Failed to schedule job")
		applogger.Error("  >> job  : " + name)
		applogger.Error("  >> error: " + err.Error())
		applogger.Info("")
		return
	}
	applogger.Info("  * Scheduled job added: " + name)
}

func (cs *CronService) addStartupJob(name string, job func(), delay time.Duration) {
	go func() {
		time.Sleep(delay)
		applogger.Info("")
		applogger.Info("Executing startup job: ")
		applogger.Info("  >> job  : " + name)
		applogger.Info("  >> time : " + time.Now().Format("15:04:05"))
		applogger.Info("")
		job()
	}()
	applogger.Info("  * Startup job queued : " + name)
}

func (cs *CronService) apiInstrumentsUpdateJob() {

	totalInserted, err := cs.instrumentService.UpdateInstruments()
	if err != nil {
		applogger.Error("Failed to update instruments")
		applogger.Error("  * error    : " + err.Error())
		applogger.Info("")
		return
	}

	applogger.Info("Instruments update successful")
	applogger.Info("  * total_inserted    : " + strconv.Itoa(totalInserted))
	applogger.Info("")
	applogger.Info(config.SingleLine)
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
		applogger.Error("Ticker generate session failed")
		applogger.Error("  * error    : " + err.Error())
		applogger.Info("")

		return
	}
	applogger.Info("Ticker generate session successful")
	applogger.Info("  * user_id    : " + sessionData.UserID)
	applogger.Info("  * login_time : " + sessionData.LoginTime)
	applogger.Info("")

	// Start the ticker
	err = cs.tickerService.Start(sessionData.UserID, sessionData.Enctoken)
	if err != nil {
		applogger.Error("Ticker start failed")
		applogger.Error("  * error    : " + err.Error())
		applogger.Info("")
		return
	}

	applogger.Info("Ticker start successful")
	applogger.Info("")
	applogger.Info(config.SingleLine)

}

func (cs *CronService) tickerDataTruncateJob() {
	applogger.Info("Starting Ticker Data Truncate Job")
	applogger.Info("")

	// Truncate the table
	if err := cs.tickerService.TruncateTickerData(); err != nil {
		applogger.Error("TickerData truncate Failed")
		applogger.Error("  * error    : " + err.Error())
		applogger.Info("")
		return
	}
	applogger.Info("TickerData truncate successful")
	applogger.Info("")

	applogger.Info("Ticker Data Truncate Job Completed")
	applogger.Info(config.SingleLine)
}

func (cs *CronService) tickerInstrumentsUpdateJob() {
	applogger.Info("Starting Ticker Instruments Update Job")
	applogger.Info("")

	// Truncate the table
	if err := cs.tickerService.TruncateTickerInstruments(); err != nil {
		applogger.Error("TickerInstruments truncate Failed")
		applogger.Error("  * error    : " + err.Error())
		applogger.Info("")
		return
	}
	applogger.Info("TickerInstruments truncate successful")
	applogger.Info("")

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
			applogger.Error("Failed to upsert queried instruments:")
			applogger.Error("  * query      : " + q.description)
			applogger.Error("  * error      : " + err.Error())
			applogger.Info("")
			continue
		}

		applogger.Info("TickerInstruments upsert results for query:")
		applogger.Info("  * query      : " + q.description)
		applogger.Info("  * queried    : " + strconv.Itoa(result["queried"].(int)))
		applogger.Info("  * added      : " + strconv.Itoa(result["added"].(int)))
		applogger.Info("  * updated    : " + strconv.Itoa(result["updated"].(int)))
		applogger.Info("  * total      : " + strconv.FormatInt(result["total"].(int64), 10))
		applogger.Info("")
	}

	// Add provision for upserting selected indices
	indices := []string{"NSE:NIFTY 500", "NSE:NIFTY BANK"}
	for _, indexName := range indices {

		instruments, err := cs.indexService.FetchIndexInstrumentsList(indexName)
		if err != nil {
			applogger.Error("Failed to fetch index instruments:")
			applogger.Error("  * index : " + indexName)
			applogger.Error("  * error : " + err.Error())
			applogger.Info("")
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
		applogger.Info("TickerInstruments upsert results for index:")
		applogger.Info("  * index       : " + indexName + " [INDEX]")
		applogger.Info("  * instruments : " + strconv.Itoa(len(instruments)))
		applogger.Info("  * queried     : " + strconv.Itoa(totalQueried))
		applogger.Info("  * added       : " + strconv.Itoa(totalAdded))
		applogger.Info("  * updated     : " + strconv.Itoa(totalUpdated))
		applogger.Info("  * total       : " + strconv.FormatInt(totalInstruments, 10))

		if len(failedInstruments) > 0 {
			applogger.Error("  * failed      : " + strconv.Itoa(len(failedInstruments)))
			applogger.Error("  * failed instruments:")
			for _, failedInstr := range failedInstruments {
				applogger.Error("    - " + failedInstr)
			}
		}
		applogger.Info("")
	}

	applogger.Info("Ticker Instruments Update Job Completed")
	applogger.Info(config.SingleLine)
}
