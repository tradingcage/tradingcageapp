package simulate

import (
	"fmt"
	"sync"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/bars"
	"github.com/tradingcage/tradingcage-go/pkg/database"

	"golang.org/x/sync/errgroup"
)

var (
	TickerMultiplier = map[uint]float64{
		1:  50,
		2:  20,
		3:  5,
		14: 100000,
		15: 62500,
		16: 1000,
		18: 125000,
		19: 50,
		20: 100000,
		21: 12500000,
		23: 10000,
		25: 100000,
		26: 100000,
		17: 25,
		22: 100,
		24: 250,
	}
)

// Given a set of bars, existing active orders, and current positions,
// simulate the active orders against the bars and update the positions.
// Returns a list of orders that were fulfilled and an updated list of
// positions to replace the prior one.
func SimulateBars(
	barsBySymbol map[uint][]bars.Bar,
	orders []database.Order,
	positions []database.Position,
) (
	bool,
	[]database.Order,
	[]database.Position,
	float64,
	error,
) {

	// Keep track of the symbols for active positions so we replace them correctly
	symbolsWithPositions := make(map[uint][]database.Position)
	for _, pos := range positions {
		symbolsWithPositions[pos.SymbolID] = append(symbolsWithPositions[pos.SymbolID], pos)
	}

	var retLock sync.Mutex
	var retOrders []database.Order
	var retPositions []database.Position
	var retCash float64
	var eg errgroup.Group

	didExecute := false

	// Simulate bars for each relevant symbol in parallel

	for symbolID, bars := range barsBySymbol {

		delete(symbolsWithPositions, symbolID)

		// Filter relevant orders and positions for this symbol
		var symOrders []database.Order
		var symPositions []database.Position
		for _, order := range orders {
			if order.SymbolID == symbolID {
				symOrders = append(symOrders, order)
			}
		}
		for _, position := range positions {
			if position.SymbolID == symbolID {
				symPositions = append(symPositions, position)
			}
		}

		// Now do the simulation
		symbolID := symbolID
		bars := bars
		eg.Go(func() error {
			symOrders := symOrders
			symPositions := symPositions
			executedOrder, newOrders, newPositions, newCash, err := simulateBars(
				symbolID,
				bars,
				symOrders,
				symPositions,
			)
			if err != nil {
				return err
			}

			// Combine the results
			retLock.Lock()
			if executedOrder {
				didExecute = true
			}
			retOrders = append(retOrders, newOrders...)
			retPositions = append(retPositions, newPositions...)
			retCash += newCash
			retLock.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return false, nil, nil, 0, err
	}

	// Add back in the positions that weren't affected by the simulation

	for _, positions := range symbolsWithPositions {
		retPositions = append(retPositions, positions...)
	}

	return didExecute, retOrders, retPositions, retCash, nil
}

func simulateBars(
	symbolID uint,
	bars []bars.Bar,
	orders []database.Order,
	positions []database.Position,
) (
	bool,
	[]database.Order,
	[]database.Position,
	float64,
	error,
) {

	ordersToUpdate := []database.Order{}

	if len(bars) == 0 {
		return false, ordersToUpdate, positions, 0, nil
	}

	// For each bar, go through the remaining limit and stop orders,
	// check if they are applicable, and if so, execute them
	didExecute := false
	totalPnl := float64(0)
	var pnl float64
	orderIndexesToUpdate := make(map[int]struct{})

	for _, bar := range bars {
		for j, order := range orders {
			if orders[j].ActivatedAt == nil ||
				orders[j].CancelledAt != nil ||
				orders[j].FulfilledAt != nil {
				continue
			}
			if price := getOrderPrice(bar, order); price != -1 {
				orderIndexesToUpdate[j] = struct{}{}
				t := time.Unix(0, bar.Date*int64(time.Millisecond))
				orders[j].FulfilledAt = &t
				orders[j].FulfilledPrice = price
				positions, pnl = executeOrder(symbolID, order, positions, price)

				// Check if this is an entry order that will activate other pending orders
				if order.EntryOrderID == nil {
					for j2, _ := range orders {
						if orders[j2].EntryOrderID != nil &&
							*orders[j2].EntryOrderID == order.ID &&
							orders[j2].ActivatedAt == nil {
							// Activate the order and make sure it gets updated
							orders[j2].ActivatedAt = &t
							orderIndexesToUpdate[j2] = struct{}{}
							fmt.Printf("activating order %d\n", orders[j2].ID)
						}
					}
				} else {
					// Check if this is part of an active OCO bracket that will cancel other orders
					for j2, _ := range orders {
						if (orders[j2].ID == *order.EntryOrderID ||
							(orders[j2].EntryOrderID != nil && *orders[j2].EntryOrderID == *order.EntryOrderID)) &&
							orders[j2].ID != order.ID &&
							orders[j2].CancelledAt == nil &&
							orders[j2].ActivatedAt != nil &&
							orders[j2].FulfilledAt == nil {
							fmt.Printf("cancelling order %d\n", orders[j2].ID)
							orders[j2].CancelledAt = &t
							orderIndexesToUpdate[j2] = struct{}{}
						}
					}
				}

				totalPnl += pnl
				didExecute = true
			}
		}
	}

	for i, _ := range orderIndexesToUpdate {
		fmt.Printf("updating order %d\n", orders[i].ID)
		ordersToUpdate = append(ordersToUpdate, orders[i])
	}

	return didExecute, ordersToUpdate, positions, totalPnl * TickerMultiplier[symbolID], nil
}

func getOrderPrice(bar bars.Bar, order database.Order) float64 {
	if order.FulfilledAt != nil {
		return -1
	}
	if order.OrderType == "market" {
		return bar.Open
	}
	if bar.Low <= order.Price && order.Price <= bar.High {
		return order.Price
	}
	return -1
}

func calculatePnl(
	direction string,
	entryPrice,
	exitPrice float64,
	quantity int,
) float64 {
	if direction == "buy" {
		return (exitPrice - entryPrice) * float64(quantity)
	}
	return (entryPrice - exitPrice) * float64(quantity)
}

func executeOrder(
	symbolID uint,
	order database.Order,
	positions []database.Position,
	price float64,
) ([]database.Position, float64) {
	if len(positions) == 0 || order.Direction == positions[0].Direction {
		positions = append(positions, database.Position{
			SymbolID:  symbolID,
			AccountID: order.AccountID,
			Direction: order.Direction,
			Price:     price,
			Quantity:  order.Quantity,
		})
		return positions, 0
	}

	quantityRemaining := order.Quantity
	newPositions := []database.Position{}
	pnl := float64(0)
	for _, position := range positions {
		if quantityRemaining == 0 {
			newPositions = append(newPositions, position)
			continue
		}
		if position.Quantity > quantityRemaining {
			pnl += calculatePnl(position.Direction, position.Price, price, quantityRemaining)
			newPositions = append(newPositions, database.Position{
				SymbolID:  symbolID,
				AccountID: position.AccountID,
				Direction: position.Direction,
				Price:     price,
				Quantity:  position.Quantity - quantityRemaining,
			})
			quantityRemaining = 0
		} else if position.Quantity == quantityRemaining {
			pnl += calculatePnl(position.Direction, position.Price, price, quantityRemaining)
			quantityRemaining = 0
		} else { // position.Quantity < quantityRemaining
			pnl += calculatePnl(position.Direction, position.Price, price, position.Quantity)
			quantityRemaining -= position.Quantity
		}
	}

	if quantityRemaining > 0 {
		newPositions = append(newPositions, database.Position{
			SymbolID:  symbolID,
			AccountID: order.AccountID,
			Direction: order.Direction,
			Price:     price,
			Quantity:  quantityRemaining,
		})
	}

	return newPositions, pnl
}
