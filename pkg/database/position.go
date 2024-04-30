package database

import (
	"gorm.io/gorm"
)

type Position struct {
	AccountID uint `gorm:"index;not null"`
	SymbolID  uint
	Direction string `gorm:"type:text;not null"`
	Price     float64
	Quantity  int
}

// Upsert function is no longer required because GORM handles this natively.

func (position *Position) Create(db *gorm.DB) error {
	err := db.Create(position).Error
	if err != nil {
		return err
	}
	return nil
}

func (position *Position) Update(db *gorm.DB) error {
	err := db.Save(position).Error
	if err != nil {
		return err
	}
	return nil
}

func GetPositionsForAccount(db *gorm.DB, accountID uint) ([]Position, error) {
	var positions []Position
	err := db.Where("account_id = ?", accountID).Find(&positions).Error
	return positions, err
}

func ReplacePositionsForAccount(db *gorm.DB, accountID uint, positions []Position) error {
	// Use transactions for replacing positions
	err := Transaction(db, func(tx *gorm.DB) error {
		// Delete existing positions for the account
		if err := tx.Where("account_id = ?", accountID).Delete(&Position{}).Error; err != nil {
			return err // return will roll back the transaction
		}

		// Insert new positions
		for _, position := range positions {
			position.AccountID = accountID // Ensure the AccountID is set correctly
			if err := tx.Create(&position).Error; err != nil {
				return err // return will roll back the transaction
			}
		}
		// If no errors are returned, the transaction is committed
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
