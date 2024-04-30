package database

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	ID             uint `gorm:"primaryKey"`
	AccountID      uint `gorm:"index"`
	SymbolID       uint `gorm:"index"`
	Direction      string
	Price          float64
	FulfilledPrice float64
	Quantity       int
	OrderType      string
	CreatedAt      *time.Time
	ActivatedAt    *time.Time `gorm:"index:idx_order_active"`
	CancelledAt    *time.Time `gorm:"index:idx_order_active"`
	FulfilledAt    *time.Time `gorm:"index:idx_order_active,sort:desc;index:idx_order_fulfilled,sort:desc"`
	EntryOrderID   *uint      // If EntryOrderID is non-null, this is a linked order in a OCO bracket
}

func (order *Order) Create(db *gorm.DB) error {
	err := db.Create(order).Error
	if err != nil {
		return err
	}
	return nil
}

func (order *Order) Update(db *gorm.DB) error {
	err := db.Save(order).Error
	if err != nil {
		return err
	}
	return nil
}

func UpdateMultipleOrders(db *gorm.DB, orders []Order) error {
	accountIDs := make(map[uint]struct{})
	err := Transaction(db, func(tx *gorm.DB) error {
		for _, order := range orders {
			accountIDs[order.AccountID] = struct{}{}
			if err := tx.Model(&Order{}).Where("id = ?", order.ID).Updates(order).Error; err != nil {
				return err // rollback will be triggered
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func GetReadyOrders(db *gorm.DB, accountID uint) ([]Order, error) {
	var orders []Order
	result := db.Where("account_id = ? AND cancelled_at IS NULL AND fulfilled_at IS NULL", accountID).Order("fulfilled_at desc").Find(&orders)
	return orders, result.Error
}

func GetFulfilledOrders(db *gorm.DB, accountID uint) ([]Order, error) {
	var orders []Order
	result := db.Where("account_id = ? AND fulfilled_at IS NOT NULL", accountID).Order("fulfilled_at desc").Limit(50).Find(&orders)
	return orders, result.Error
}

func GetAllFulfilledOrders(db *gorm.DB, accountID uint) ([]Order, error) {
	var orders []Order
	result := db.Where("account_id = ? AND fulfilled_at IS NOT NULL", accountID).Order("fulfilled_at desc").Find(&orders)
	return orders, result.Error
}

func GetOrderByID(db *gorm.DB, orderID uint) (Order, error) {
	var order Order
	err := db.First(&order, orderID).Error
	return order, err
}

func GetLinkedOrdersFromEntryOrder(db *gorm.DB, order Order) ([]Order, error) {
	var linkedOrders []Order
	err := db.Where("entry_order_id = ?", order.ID).Find(&linkedOrders).Error
	return linkedOrders, err
}
