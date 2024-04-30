package analytics

import (
	"fmt"
	"sort"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/database"
	"github.com/tradingcage/tradingcage-go/pkg/simulate"
	"gorm.io/gorm"
)

var (
	timeFormat = "2006-01-02 15:04:05"

	locationChicago, _ = time.LoadLocation("America/Chicago")
)

type Trade struct {
	AccountID    uint    `json:"accountID"`
	SymbolID     uint    `json:"symbolID"`
	Quantity     int     `json:"quantity"`
	EntryPrice   float64 `json:"entryPrice"`
	ExitPrice    float64 `json:"exitPrice"`
	EnteredAt    string  `json:"enteredAt"`
	ExitedAt     string  `json:"exitedAt"`
	ProfitOrLoss float64 `json:"profitOrLoss"`
}

// GetTrades retrieves trades for a given account ID and converts matched
// buy and sell orders into trades. It also accounts for symbol matching and short entries.
func GetTrades(db *gorm.DB, accountID uint) ([]Trade, error) {
	fulfilledOrders, err := database.GetAllFulfilledOrders(db, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to get fulfilled orders: %w", err)
	}

	var trades []Trade
	orderQueue := make(map[uint][]database.Order) // Keyed by SymbolID

	// Sort fulfilled orders by their fulfilled timestamp
	sort.Slice(fulfilledOrders, func(i, j int) bool {
		return fulfilledOrders[i].FulfilledAt.Before(*fulfilledOrders[j].FulfilledAt)
	})

	// Group orders by SymbolID
	for _, order := range fulfilledOrders {
		orderQueue[order.SymbolID] = append(orderQueue[order.SymbolID], order)
	}

	// Generate trades per symbol
	for symbolID, orders := range orderQueue {
		var buyQueue []database.Order
		var sellQueue []database.Order

		// Separate orders by buy and sell
		for _, order := range orders {
			switch order.Direction {
			case "buy":
				buyQueue = append(buyQueue, order)
			case "sell":
				sellQueue = append(sellQueue, order)
			}
		}

		for len(buyQueue) > 0 && len(sellQueue) > 0 {
			buyOrder := &buyQueue[0]
			sellOrder := &sellQueue[0]

			// Determine trade quantity based on the smaller of the two order quantities
			tradeQuantity := buyOrder.Quantity
			if sellOrder.Quantity < buyOrder.Quantity {
				tradeQuantity = sellOrder.Quantity
			}

			var trade Trade
			var profitOrLoss float64

			var entryOrder *database.Order
			var exitOrder *database.Order

			if buyOrder.FulfilledAt.Before(*sellOrder.FulfilledAt) {
				entryOrder = buyOrder
				exitOrder = sellOrder
				profitOrLoss = float64(tradeQuantity) * (exitOrder.FulfilledPrice - entryOrder.FulfilledPrice) * simulate.TickerMultiplier[symbolID]
			} else {
				entryOrder = sellOrder
				exitOrder = buyOrder
				profitOrLoss = float64(tradeQuantity) * (entryOrder.FulfilledPrice - exitOrder.FulfilledPrice) * simulate.TickerMultiplier[symbolID]
			}

			trade = Trade{
				AccountID:    accountID,
				SymbolID:     symbolID,
				Quantity:     tradeQuantity,
				EntryPrice:   entryOrder.FulfilledPrice,
				ExitPrice:    exitOrder.FulfilledPrice,
				EnteredAt:    entryOrder.FulfilledAt.In(locationChicago).Format(timeFormat),
				ExitedAt:     exitOrder.FulfilledAt.In(locationChicago).Format(timeFormat),
				ProfitOrLoss: profitOrLoss,
			}

			trades = append(trades, trade)

			// Decrement order quantities by the trade quantity
			buyOrder.Quantity -= tradeQuantity
			sellOrder.Quantity -= tradeQuantity

			// Remove the order from the queue if its quantity is now zero
			if buyOrder.Quantity == 0 {
				buyQueue = buyQueue[1:]
			}
			if sellOrder.Quantity == 0 {
				sellQueue = sellQueue[1:]
			}
		}
	}

	return trades, nil
}
