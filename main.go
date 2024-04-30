package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/analytics"
	"github.com/tradingcage/tradingcage-go/pkg/auth"
	"github.com/tradingcage/tradingcage-go/pkg/bars"
	"github.com/tradingcage/tradingcage-go/pkg/billing"
	"github.com/tradingcage/tradingcage-go/pkg/database"
	"github.com/tradingcage/tradingcage-go/pkg/email"
	"github.com/tradingcage/tradingcage-go/pkg/replay"
	"github.com/tradingcage/tradingcage-go/pkg/simulate"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/posthog/posthog-go"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var db *gorm.DB

var barsData bars.BarData

var timescaleURL = os.Getenv("TIMESCALE_URL")

var wsupgrader = &websocket.Upgrader{}

// This is the struct that gets sent back from the websocket
type replayData struct {
	Bars            map[uint][]bars.Bar `json:"bars"`
	Account         *database.Account   `json:"account"`
	ActiveOrders    []database.Order    `json:"activeOrders"`
	FulfilledOrders []database.Order    `json:"fulfilledOrders"`
	Positions       []database.Position `json:"positions"`
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (r replayData) Send(c *websocket.Conn) {
	stateJSON, err := json.Marshal(r)
	if err != nil {
		log.Print("error marshaling response: ", err)
		return
	}
	err = c.WriteMessage(websocket.TextMessage, stateJSON)
	if err != nil {
		log.Print("error writing message: ", err)
	}
}

func checkJSONError(c *gin.Context, err error) bool {
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return true
	}
	return false
}

func simulateFn(c *gin.Context) {
	authInfo := auth.GetAuthInfoFromContext(c)

	idStr := c.Query("accountID")
	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No accountID specified"})
		return
	}
	accountIDInt, err := strconv.Atoi(idStr)
	if err != nil || accountIDInt <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountID parameter"})
		return
	}
	accountID := uint(accountIDInt)

	var symbolID uint
	symbolIDStr := c.Query("symbolID")
	if symbolIDStr != "" {
		symbolID64, err := strconv.ParseUint(symbolIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid symbolID parameter"})
			return
		}
		symbolID = uint(symbolID64)
	}

	account, err := database.GetAccountByID(db, accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if authInfo.UserID != account.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission"})
		return
	}
	startMillis := account.Date.UnixMilli()

	uad, err := database.NewUpdatedAccountData(db, accountID)
	if err != nil {
		log.Print("failed to get uad: ", err)
	}

	// Determine which symbols we need by looking at the orders
	symbolIDsMap := make(map[uint]struct{})
	for _, order := range uad.GetOrders() {
		symbolIDsMap[order.SymbolID] = struct{}{}
	}
	symbolIDsMap[symbolID] = struct{}{}
	symbolIDs := make([]uint, 0, len(symbolIDsMap))
	for symbolID := range symbolIDsMap {
		symbolIDs = append(symbolIDs, symbolID)
	}
	// Start replaying and simulating and send updates through websocket
	barCh := make(chan map[uint][]bars.Bar)
	defer close(barCh)
	replayer := replay.NewReplayer(symbolIDs, startMillis, barsData, barCh)
	defer replayer.Close()

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(c)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case barMap := <-barCh:
				var ret replayData
				ret.Bars = barMap
				// Simulate orders
				didExecute, ord, pos, pnl, err := simulate.SimulateBars(barMap, uad.GetOrders(), uad.GetPositions())
				if err != nil {
					log.Print("error simulating bars: ", err)
					continue
				}
				if !didExecute {
					ret.Send(conn)
					if symbolBars, ok := barMap[symbolID]; ok && len(symbolBars) > 0 {
						firstBar := symbolBars[0]
						if firstBar.Date > account.Date.UnixMilli() {
							account.Date = time.UnixMilli(firstBar.Date)
							if err = account.Update(db); err != nil {
								log.Printf("account.Update error: %s", err.Error())
							}
						}
					}
					continue
				}
				// Update orders and positions
				var activeOrders []database.Order
				var fulfilledOrders []database.Order
				err = database.Transaction(db, func(db *gorm.DB) error {
					if err := database.UpdateMultipleOrders(db, ord); err != nil {
						return fmt.Errorf("database.UpdateMultipleOrders: %w", err)
					}
					if err = database.ReplacePositionsForAccount(db, accountID, pos); err != nil {
						return fmt.Errorf("database.ReplacePositionsForAccount: %w", err)
					}
					account, err = database.GetAccountByID(db, accountID)
					if err != nil {
						return fmt.Errorf("database.GetAccountByID: %w", err)
					}
					account.RealizedPnL += pnl
					if err = account.Update(db); err != nil {
						return fmt.Errorf("account.Update: %w", err)
					}
					activeOrders, err = database.GetReadyOrders(db, accountID)
					if err != nil {
						return err
					}
					fulfilledOrders, err = database.GetFulfilledOrders(db, accountID)
					if err != nil {
						return err
					}
					return nil
				})
				if err != nil {
					log.Printf("error updating orders and positions after execution: %s", err.Error())
					continue
				}

				uad.SetAccount(account)
				uad.SetOrders(activeOrders)
				uad.SetPositions(pos)

				// Send updated orders and positions back to the client
				ret.Account = &account
				ret.ActiveOrders = activeOrders
				ret.FulfilledOrders = fulfilledOrders
				ret.Positions = pos
				ret.Send(conn)
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		command, err := replay.ParseCommand(msg)
		if err != nil {
			log.Println("error parsing command: ", err)
			continue
		}

		replayer.SendCommand(command)
	}
}

func replayFn(c *gin.Context) {
	var startMillis int64
	var err error
	startStr := c.Query("startingDateMillis")
	if startStr != "" {
		startMillis, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid startingDateMillis parameter"})
			return
		}
	}

	var symbolID uint
	symbolIDStr := c.Query("symbolID")
	if symbolIDStr != "" {
		symbolID64, err := strconv.ParseUint(symbolIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid symbolID parameter"})
			return
		}
		symbolID = uint(symbolID64)
	}

	barCh := make(chan map[uint][]bars.Bar)
	defer close(barCh)
	replayer := replay.NewReplayer([]uint{symbolID}, startMillis, barsData, barCh)
	defer replayer.Close()

	conn, err := wsupgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer conn.Close()

	go func() {
		for recvBarsM := range barCh {
			for _, recvBars := range recvBarsM {
				ret := replayData{
					Bars: map[uint][]bars.Bar{
						symbolID: recvBars,
					},
				}
				ret.Send(conn)
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		command, err := replay.ParseCommand(msg)
		if err != nil {
			log.Printf("error parsing command: %v", err)
			continue
		}

		replayer.SendCommand(command)
	}
}

func GetCurrentGitCommitHash() (string, error) {
	// Create the command to get the current git commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	// Run the command and capture the output
	out, err := cmd.Output()
	if err != nil {
		return "", err // Return the error if the command execution fails
	}
	// Convert the output to a string and trim any whitespace
	commitHash := string(out)
	commitHash = strings.TrimSpace(commitHash)
	return commitHash, nil
}

func PosthogMiddleware(client posthog.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		if strings.HasPrefix(c.Request.URL.Path, "/static/") {
			return
		}

		if c.Writer.Status() != http.StatusOK {
			return
		}

		authInfo, _ := auth.TryGetAuthInfo(c)
		if authInfo == nil {
			return
		}
		properties := posthog.NewProperties().
			Set("path", c.Request.URL.Path).
			Set("duration", time.Since(start).Milliseconds())
		distinctID := fmt.Sprintf("%d", authInfo.UserID)
		// Collect data about the request
		evt := posthog.Capture{
			DistinctId: distinctID,
			Event:      "http_request",
			Properties: properties,
		}

		// Send the event to posthog
		client.Enqueue(evt)
	}
}

func SentryErrorReportingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		blw := &bodyLogWriter{body: new(bytes.Buffer), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next() // process request

		// Check if the response status code is in the 5xx range
		if c.Writer.Status() >= http.StatusInternalServerError {
			if hub := sentrygin.GetHubFromContext(c); hub != nil {
				// You can add more information about the error here if needed
				hub.CaptureMessage(fmt.Sprintf("5xx error occurred: %s", blw.body.String()))
			}
		}
	}
}

func main() {

	buildHash, err := GetCurrentGitCommitHash()
	if err != nil {
		buildHash = "unknown"
	}

	client, _ := posthog.NewWithConfig(
		"phc_PhaIRFLc0Q2oswM89MOE5HAlFWyY6APOU49xsDb5Gg5",
		posthog.Config{Endpoint: "https://us.posthog.com"},
	)
	defer client.Close()

	log.Println("Initializing data...")

	db = database.Init()

	barsData, err = bars.NewTimescaleData(timescaleURL)
	if err != nil {
		log.Fatalf("NewTimescaleData: %s\n", err)
	}

	log.Println("Done initializing data. Initializing application...")

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:           "https://a0549be3002d34cfa64cce7604f65f43@o4506507863851008.ingest.sentry.io/4506508057378816",
		EnableTracing: true,
		// Set TracesSampleRate to 1.0 to capture 100%
		// of transactions for performance monitoring.
		// We recommend adjusting this value in production,
		TracesSampleRate: 1.0,
	}); err != nil {
		log.Printf("Sentry initialization failed: %v", err)
	}

	sendGridAPIKey := os.Getenv("SENDGRID_API_KEY")
	emailService := email.NewSendGridEmailService(sendGridAPIKey)

	r := gin.Default()

	r.Use(auth.AuthInfoMiddleware)
	r.Use(sentrygin.New(sentrygin.Options{}))
	r.Use(PosthogMiddleware(client))
	r.Use(SentryErrorReportingMiddleware())

	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// If the user is not authed, redirect them to the login page
	r.GET("/", func(c *gin.Context) {
		if _, err := auth.CheckAuthenticated(c); err == nil {
			c.Redirect(http.StatusFound, "/dashboard")
		} else {
			c.HTML(http.StatusOK, "index.tmpl", gin.H{})
		}
	})

	r.POST("/register", auth.RegisterEndpoint(db))
	r.POST("/login", auth.LoginEndpoint(db))
	r.POST("/forgot-password", auth.ForgotPasswordEndpoint(emailService, db))
	r.POST("/reset-password", auth.ResetPasswordEndpoint(db))

	// Serve HTML templates for authentication-related endpoints
	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.tmpl", gin.H{})
	})
	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.tmpl", gin.H{})
	})
	r.GET("/logout", func(c *gin.Context) {
		auth.Logout(c)
		c.Redirect(http.StatusFound, "/")
	})
	r.GET("/forgot-password", func(c *gin.Context) {
		c.HTML(http.StatusOK, "forgot-password.tmpl", gin.H{})
	})
	r.GET("/reset-password", func(c *gin.Context) {
		token := c.Query("token")
		c.HTML(http.StatusOK, "reset-password.tmpl", gin.H{"Token": token})
	})
	r.GET("/checkout", func(c *gin.Context) {
		var username string
		var userID uint
		var err error
		if username, userID, err = auth.GetCurrentUsername(c); err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}

		u, err := database.GetUserByID(db, userID)
		if u.HasActiveSubscription() {
			c.Redirect(http.StatusFound, "/")
			return
		}
		if err != nil {
			c.Redirect(http.StatusFound, "/")
			return
		}
		clientSecret, err := billing.CreateCheckoutSession(c, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		c.HTML(http.StatusOK, "checkout.tmpl", gin.H{
			"UserID":         userID,
			"Username":       username,
			"ClientSecret":   clientSecret,
			"PublishableKey": os.Getenv("STRIPE_PUBLISHABLE_KEY"),
		})
	})
	r.GET("/checkout-result", func(c *gin.Context) {
		sessionID := c.Query("sessionID")
		sessionStatus := billing.GetCheckoutSessionStatus(sessionID)
		c.HTML(http.StatusOK, "checkout-result.tmpl", gin.H{
			"SessionStatus": sessionStatus,
		})
	})
	r.POST("/stripe-webhook", billing.HandleStripeWebhook(db))

	r.Use(auth.AuthMiddleware)
	r.Use(billing.BillingMiddleware(db))
	{
		r.GET("/bar-ranges", func(c *gin.Context) {
			dateRanges, err := barsData.GetSymbolDateRanges()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			c.JSON(http.StatusOK, dateRanges)
		})

		r.GET("/simulate", simulateFn)
		r.GET("/replay", replayFn)

		r.GET("/dashboard", func(c *gin.Context) {
			authInfo := auth.GetAuthInfoFromContext(c)
			var accounts []database.Account
			err := db.Where("user_id = ?", authInfo.UserID).Order("created_at asc").Find(&accounts).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load accounts"})
				return
			}

			manageSubscriptionLink, err := billing.GetManageSubscriptionLink(c, db, authInfo.Username)
			if err != nil {
				log.Printf("Error getting manage subscription link: %s\n", err.Error())
			}

			c.HTML(http.StatusOK, "dashboard.tmpl", gin.H{
				"title":                  "Trading Cage - Dashboard",
				"Accounts":               accounts,
				"ManageSubscriptionLink": manageSubscriptionLink,
			})
		})

		r.GET("/analytics/:accountID", func(c *gin.Context) {
			authInfo := auth.GetAuthInfoFromContext(c)
			accountIDParam := c.Param("accountID")
			accountID, err := strconv.ParseUint(accountIDParam, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountID parameter"})
				return
			}
			var account database.Account
			err = db.Where("user_id = ? AND id = ?", authInfo.UserID, accountID).First(&account).Error
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				}
				return
			}
			trades, err := analytics.GetTrades(db, uint(accountID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			tradeMetrics := analytics.CalculateTradeMetrics(trades)
			c.HTML(http.StatusOK, "analytics.tmpl", gin.H{
				"title":        "Trading Cage - Analytics",
				"account":      account,
				"trades":       trades,
				"tradeMetrics": tradeMetrics,
			})
		})

		r.GET("/chart-finder", func(c *gin.Context) {
			c.HTML(http.StatusOK, "chart-finder.tmpl", gin.H{
				"title":     "Trading Cage - Chart Finder",
				"buildHash": buildHash,
			})
		})

		r.GET("/chart", func(c *gin.Context) {
			c.HTML(http.StatusOK, "chart.tmpl", gin.H{
				"title":     "Trading Cage - Time Traveling Chart",
				"buildHash": buildHash,
			})
		})

		r.GET("/simulator/:accountID", func(c *gin.Context) {
			authInfo := auth.GetAuthInfoFromContext(c)

			idStr := c.Param("accountID")
			accountIDInt, err := strconv.Atoi(idStr)
			if err != nil || accountIDInt <= 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountID parameter"})
				return
			}
			accountID := uint(accountIDInt)

			var account database.Account
			var activeOrders []database.Order
			var positions []database.Position
			var fulfilledOrders []database.Order
			err = database.Transaction(db, func(db *gorm.DB) error {
				account, err = database.GetAccountByID(db, accountID)
				if err != nil {
					return err
				}
				if account.UserID != authInfo.UserID {
					return auth.ErrNotAuthorized
				}
				activeOrders, err = database.GetReadyOrders(db, accountID)
				if err != nil {
					return err
				}
				fulfilledOrders, err = database.GetFulfilledOrders(db, accountID)
				if err != nil {
					return err
				}
				positions, err = database.GetPositionsForAccount(db, accountID)
				if err != nil {
					return err
				}
				return nil
			})
			if checkJSONError(c, err) {
				return
			}
			c.HTML(http.StatusOK, "simulator.tmpl", gin.H{
				"title":           "Trading Cage - Simulator",
				"date":            account.Date.UnixMilli(),
				"activeOrders":    activeOrders,
				"positions":       positions,
				"realizedPnl":     account.RealizedPnL,
				"fulfilledOrders": fulfilledOrders,
				"accountID":       accountID,
				"buildHash":       buildHash,
			})
		})

		r.POST("/bars", func(c *gin.Context) {
			var getBarsRequest bars.GetBarsRequest
			err := c.ShouldBindJSON(&getBarsRequest)
			if checkJSONError(c, err) {
				return
			}

			var resultBars []bars.Bar
			var lastPrices map[uint]float64
			var eg errgroup.Group

			eg.Go(func() error {
				var err error
				resultBars, err = barsData.GetBars(getBarsRequest)
				if err != nil {
					return err
				}
				return nil
			})

			eg.Go(func() error {
				var err error
				lastPrices, err = barsData.GetLastPrices(getBarsRequest.EndDate, getBarsRequest.SymbolID)
				if err != nil {
					return err
				}
				return nil
			})

			if err := eg.Wait(); checkJSONError(c, err) {
				return
			}
			payload := struct {
				Bars       []bars.Bar       `json:"bars"`
				LastPrices map[uint]float64 `json:"lastPrices"`
			}{
				Bars:       resultBars,
				LastPrices: lastPrices,
			}
			resultJSON, err := json.Marshal(payload)
			if checkJSONError(c, err) {
				return
			}
			c.Data(http.StatusOK, "application/json", resultJSON)
		})
		r.POST("/inc-date", func(c *gin.Context) {
			var req struct {
				Inc       string `json:"inc"`
				AccountID uint   `json:"accountID"`
			}
			err := c.ShouldBindJSON(&req)
			if checkJSONError(c, err) {
				return
			}
			authInfo := auth.GetAuthInfoFromContext(c)

			account, activeOrders, fulfilledOrders, newPositions, err := simulate.IncDate(
				db, authInfo, barsData, req.AccountID, req.Inc,
			)

			if checkJSONError(c, err) {
				return
			}

			c.JSON(http.StatusOK, replayData{
				Account:         &account,
				ActiveOrders:    activeOrders,
				FulfilledOrders: fulfilledOrders,
				Positions:       newPositions,
			})
		})
		r.POST("/submit-order", func(c *gin.Context) {
			var req struct {
				AccountID  uint `json:"accountID"`
				SymbolID   uint `json:"symbolID"`
				EntryOrder struct {
					OrderType string  `json:"orderType"`
					Direction string  `json:"direction"`
					Price     float64 `json:"price"`
					Quantity  int     `json:"quantity"`
				} `json:"entryOrder"`
				LinkedOrders []struct {
					OrderType      string  `json:"orderType"`
					Direction      string  `json:"direction"`
					Price          float64 `json:"price"`
					Quantity       int     `json:"quantity"`
					ActivateOnFill bool    `json:"activateOnFill"`
				} `json:"linkedOrders"`
			}
			err := c.ShouldBindJSON(&req)
			if checkJSONError(c, err) {
				return
			}
			if req.SymbolID < 1 || req.SymbolID > 3 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid symbol id"})
				return
			}
			if req.EntryOrder.Quantity == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "quantity cannot be 0"})
				return
			}
			authInfo := auth.GetAuthInfoFromContext(c)
			var activeOrders []database.Order
			err = database.Transaction(db, func(db *gorm.DB) error {

				account, err := database.GetAccountByID(db, req.AccountID)
				if err != nil {
					return err
				}
				if account.UserID != authInfo.UserID {
					return auth.ErrNotAuthorized
				}

				entryOrder := database.Order{
					AccountID:   req.AccountID,
					SymbolID:    req.SymbolID,
					OrderType:   req.EntryOrder.OrderType,
					Direction:   req.EntryOrder.Direction,
					Price:       req.EntryOrder.Price,
					Quantity:    req.EntryOrder.Quantity,
					CreatedAt:   &account.Date,
					ActivatedAt: &account.Date,
				}
				err = entryOrder.Create(db)
				if err != nil {
					return err
				}

				for _, linkedOrder := range req.LinkedOrders {
					newOrder := database.Order{
						AccountID:    req.AccountID,
						SymbolID:     req.SymbolID,
						OrderType:    linkedOrder.OrderType,
						Direction:    linkedOrder.Direction,
						Price:        linkedOrder.Price,
						Quantity:     linkedOrder.Quantity,
						CreatedAt:    &account.Date,
						EntryOrderID: &entryOrder.ID,
					}
					if linkedOrder.ActivateOnFill {
						// Leave ActivatedAt as nil to indicate that this order is activated after the entry order is filled.
						newOrder.ActivatedAt = nil
					} else {
						newOrder.ActivatedAt = &account.Date
					}
					if err = newOrder.Create(db); err != nil {
						return err
					}
				}

				activeOrders, err = database.GetReadyOrders(db, account.ID)
				if err != nil {
					return err
				}

				return nil
			})
			if checkJSONError(c, err) {
				return
			}
			c.JSON(http.StatusOK, activeOrders)
		})
		r.POST("/cancel-order", func(c *gin.Context) {
			var req struct {
				AccountID uint `json:"accountID"`
				OrderID   uint `json:"orderID"`
			}
			err := c.ShouldBindJSON(&req)
			if checkJSONError(c, err) {
				return
			}
			authInfo := auth.GetAuthInfoFromContext(c)
			var orders []database.Order
			err = database.Transaction(db, func(db *gorm.DB) error {

				account, err := database.GetAccountByID(db, req.AccountID)
				if err != nil {
					return err
				}
				if account.UserID != authInfo.UserID {
					return auth.ErrNotAuthorized
				}

				order, err := database.GetOrderByID(db, req.OrderID)
				if err != nil {
					return err
				}
				if order.AccountID != account.ID {
					c.JSON(http.StatusBadRequest, "You can't cancel this order")
					return err
				}
				order.CancelledAt = &account.Date
				if err := order.Update(db); err != nil {
					return err
				}
				if order.EntryOrderID == nil {
					linkedOrders, err := database.GetLinkedOrdersFromEntryOrder(db, order)
					if err != nil {
						return err
					}
					for _, linkedOrder := range linkedOrders {
						if linkedOrder.ActivatedAt == nil {
							linkedOrder.CancelledAt = &account.Date
							if err := linkedOrder.Update(db); err != nil {
								return err
							}
						}
					}
				}
				orders, err = database.GetReadyOrders(db, account.ID)
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return
			}
			c.JSON(http.StatusOK, orders)
		})
		r.POST("/create-account", func(c *gin.Context) {
			type CreateAccountRequest struct {
				Name            string `form:"account-name" binding:"required"`
				StartDate       string `form:"start-date" binding:"required"`
				StartingCapital string `form:"starting-capital" binding:"required"`
			}

			var req CreateAccountRequest
			if err := c.ShouldBind(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			startDate, err := time.Parse("2006-01-02", req.StartDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format. Please use YYYY-MM-DD."})
				return
			}

			startingCapital, err := strconv.ParseFloat(req.StartingCapital, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid starting capital format."})
				return
			}

			authInfo := auth.GetAuthInfoFromContext(c)
			account := database.Account{
				Name:        req.Name,
				UserID:      authInfo.UserID,
				Date:        startDate,
				RealizedPnL: startingCapital,
			}

			if err := account.Create(db); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create account"})
				return
			}

			// Redirect to the simulator page for the newly created account
			c.Redirect(http.StatusFound, "/simulator/"+strconv.Itoa(int(account.ID)))
		})

		// Add the following function within the `main` function in `main.go`

		r.DELETE("/account/:accountID", func(c *gin.Context) {
			accountID := c.Param("accountID")
			accountIDUint, err := strconv.ParseUint(accountID, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountID parameter"})
				return
			}

			authInfo := auth.GetAuthInfoFromContext(c)
			err = database.Transaction(db, func(db *gorm.DB) error {
				var account database.Account
				if err := db.First(&account, accountIDUint).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return nil // Return nil to not rollback the transaction when the account is not found
					}
					return err
				}
				if account.UserID != authInfo.UserID {
					return auth.ErrNotAuthorized
				}

				return db.Delete(&account).Error
			})

			if err != nil {
				if err == auth.ErrNotAuthorized {
					c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				}
				return
			}

			c.Status(http.StatusOK)
		})

		r.POST("/update-account-name", func(c *gin.Context) {
			var req struct {
				AccountID uint   `json:"accountID"`
				NewName   string `json:"newName"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			authInfo := auth.GetAuthInfoFromContext(c)
			err := database.Transaction(db, func(db *gorm.DB) error {
				var account database.Account
				if err := db.First(&account, req.AccountID).Error; err != nil {
					return err
				}
				if account.UserID != authInfo.UserID {
					return auth.ErrNotAuthorized
				}
				account.Name = req.NewName
				return db.Save(&account).Error
			})
			if err != nil {
				if errors.Is(err, auth.ErrNotAuthorized) {
					c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				}
				return
			}
			c.Status(http.StatusOK)
		})

		r.GET("/download-trades", func(c *gin.Context) {
			accountIDParam := c.Query("accountID")
			accountID, err := strconv.ParseUint(accountIDParam, 10, 32)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accountID parameter"})
				return
			}
			authInfo := auth.GetAuthInfoFromContext(c)

			account, err := database.GetAccountByID(db, uint(accountID))
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
				return
			}
			if account.UserID != authInfo.UserID {
				c.JSON(http.StatusForbidden, gin.H{"error": "you do not have permission"})
				return
			}

			trades, err := analytics.GetTrades(db, uint(accountID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			file, err := analytics.GenerateTradesExcel(trades)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate excel file"})
				return
			}

			// Set the header information for the excel file download
			c.Header("Content-Description", "File Transfer")
			c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=trades-%d.xlsx", accountID))
			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			c.File(file.Name())
		})
	}

	go analytics.CleanupTempDir(context.Background())

	log.Println("Listening on port 8080...")
	r.Run("0.0.0.0:8080")
}
