package simulate

import (
	"sync"

	"github.com/tradingcage/tradingcage-go/pkg/auth"
	"github.com/tradingcage/tradingcage-go/pkg/bars"
	"github.com/tradingcage/tradingcage-go/pkg/database"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

func IncDate(
	db *gorm.DB,
	authInfo *auth.AuthContext,
	barsData bars.BarData,
	accountID uint,
	inc string,
) (database.Account, []database.Order, []database.Order, []database.Position, error) {
	var account database.Account
	var activeOrders []database.Order
	var fulfilledOrders []database.Order
	var newPositions []database.Position
	err := database.Transaction(db, func(db *gorm.DB) error {

		var err error
		account, err = database.GetAccountByID(db, accountID)
		if err != nil {
			return err
		}
		if account.UserID != authInfo.UserID {
			return auth.ErrNotAuthorized
		}

		prevDate := account.Date
		account.IncDate(inc)

		orders, err := database.GetReadyOrders(db, accountID)
		if err != nil {
			return err
		}

		// Bail early if there are no orders
		if len(orders) == 0 {
			if err = account.Update(db); err != nil {
				return err
			}
			activeOrders, err = database.GetReadyOrders(db, accountID)
			if err != nil {
				return err
			}
			fulfilledOrders, err = database.GetFulfilledOrders(db, accountID)
			if err != nil {
				return err
			}
			newPositions, err = database.GetPositionsForAccount(db, accountID)
			if err != nil {
				return err
			}
			return nil
		}

		positions, err := database.GetPositionsForAccount(db, accountID)
		if err != nil {
			return err
		}

		// Get all the bars for each symbol ID that we care about
		barsBetween := struct {
			sync.Mutex
			m map[uint][]bars.Bar
		}{
			m: make(map[uint][]bars.Bar),
		}
		symbolIDs := make(map[uint]struct{})
		for _, order := range orders {
			symbolIDs[order.SymbolID] = struct{}{}
		}
		var eg errgroup.Group
		for symbolID := range symbolIDs {
			symbolID := symbolID
			eg.Go(func() error {
				bars, err := barsData.GetBarsBetween(bars.GetBarsBetweenRequest{
					StartDate: prevDate.UnixMilli(),
					EndDate:   account.Date.UnixMilli(),
					Timeframe: "1m",
					RTH:       false,
					SymbolID:  symbolID,
				})
				if err != nil {
					return err
				}
				barsBetween.Lock()
				barsBetween.m[symbolID] = bars
				barsBetween.Unlock()
				return nil
			})
		}
		if err := eg.Wait(); err != nil {
			return err
		}

		// Simulate the bars
		didExecute, ord, pos, pnl, err := SimulateBars(barsBetween.m, orders, positions)
		if err != nil {
			return err
		}
		if didExecute {
			newPositions = pos
			if err = database.UpdateMultipleOrders(db, ord); err != nil {
				return err
			}
			activeOrders, err = database.GetReadyOrders(db, accountID)
			if err != nil {
				return err
			}
			fulfilledOrders, err = database.GetFulfilledOrders(db, accountID)
			if err != nil {
				return err
			}
			if err = database.ReplacePositionsForAccount(db, accountID, pos); err != nil {
				return err
			}
			account.RealizedPnL += pnl
		}
		if err = account.Update(db); err != nil {
			return err
		}

		return nil
	})

	return account, activeOrders, fulfilledOrders, newPositions, err
}
