package simulatetest

import (
	"github.com/tradingcage/tradingcage-go/pkg/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupInMemoryDB sets up and returns an in-memory gorm.DB instance for testing.
func SetupInMemoryDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Migrate the schema
	if err := db.AutoMigrate(&database.User{}, &database.Account{}, &database.Order{}, &database.Position{}); err != nil {
		return nil, err
	}

	// Here, you could insert your test data needed for your test scenarios

	return db, nil
}
