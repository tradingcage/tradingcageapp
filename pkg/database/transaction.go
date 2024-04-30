package database

import (
	"gorm.io/gorm"
)

func Transaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	return db.Transaction(fn)
}
