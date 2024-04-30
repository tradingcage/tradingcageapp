package database

import (
	"gorm.io/gorm"
)

type ForgotPasswordEntry struct {
	gorm.Model
	UserID uint
	Token  string `gorm:"uniqueIndex;not null"`
}

func (fpe *ForgotPasswordEntry) Create(db *gorm.DB) error {
	return db.Create(fpe).Error
}
