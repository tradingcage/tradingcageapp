package database

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dsn string

func Init() *gorm.DB {
	dsn = os.Getenv("TIMESCALE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Get generic database object sql.DB to use its functions
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("failed to get underlying database object:", err)
	}

	// Set the maximum number of idle connections in the connection pool.
	sqlDB.SetMaxIdleConns(5)

	// Set the maximum number of open (in use + idle) connections to the database.
	sqlDB.SetMaxOpenConns(5)

	// Set the maximum amount of time a connection may be reused.
	sqlDB.SetConnMaxLifetime(time.Hour)

	err = db.AutoMigrate(&Account{}, &Order{}, &Position{}, &User{}, &ForgotPasswordEntry{})
	if err != nil {
		log.Fatal("failed to migrate database:", err)
	}

	return db
}
