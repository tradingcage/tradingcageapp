package simulatetest

import (
  "testing"

  "github.com/tradingcage/tradingcage-go/pkg/auth"
  "github.com/tradingcage/tradingcage-go/pkg/simulate"
)

func TestIncDate_SimpleBuySell(t *testing.T) {
  db, err := SetupInMemoryDB()
  if err != nil {
    t.Fatalf("Failed to set up database: %v", err)
  }
  // Assume you have a function to populate test users, accounts, and orders in your test DB
  populateTestData(db)

  barData := NewInMemoryBarData()
  // Load your bar data for the test

  authInfo := &auth.AuthContext{UserID: 1, Username: "test@example.com"} // Example test user
  accountID := uint(1) // Assuming you have a test account with this ID
  inc := "1d"          // Example increment

  account, activeOrders, _, _, err := simulate.IncDate(db, authInfo, barData, accountID, inc)
  if err != nil {
    t.Errorf("IncDate returned unexpected error: %v", err)
  }

  // Assertions follow, e.g.:
  if account.ID != accountID {
    t.Errorf("Expected account ID to be %d, got %d", accountID, account.ID)
  }

  // Additional assertions for activeOrders, filledOrders, and positions, depending on your scenario.
}
