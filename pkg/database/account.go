package database

import (
	"sync"
	"time"

	"gorm.io/gorm"
)

var loc, _ = time.LoadLocation("America/New_York")

type Account struct {
	gorm.Model
	Name        string
	UserID      uint
	Date        time.Time
	RealizedPnL float64
}

func (a *Account) Create(db *gorm.DB) error {
	return db.Create(a).Error
}

func (a *Account) Update(db *gorm.DB) error {
	err := db.Save(a).Error
	if err != nil {
		return err
	}
	return nil
}

func GetAccountByID(db *gorm.DB, accountID uint) (Account, error) {
	var account Account
	result := db.First(&account, accountID)
	return account, result.Error
}

func GetAccountByUsername(db *gorm.DB, username string) (Account, error) {
	var account Account
	result := db.Joins("JOIN users u ON u.id = accounts.user_id").
		Where("u.username = ?", username).
		First(&account)
	return account, result.Error
}

func (a *Account) IncDate(inc string) {
	switch inc {
	case "1m":
		a.Date = a.Date.Add(time.Minute).Truncate(time.Minute)
	case "5m":
		a.Date = a.Date.Add(5 * time.Minute).Truncate(5 * time.Minute)
	case "15m":
		a.Date = a.Date.Add(15 * time.Minute).Truncate(15 * time.Minute)
	case "30m":
		a.Date = a.Date.Add(30 * time.Minute).Truncate(30 * time.Minute)
	case "1h":
		a.Date = a.Date.Add(60 * time.Minute).Truncate(time.Hour)
	case "2h":
		a.Date = a.Date.Add(120 * time.Minute).Truncate(2 * time.Hour)
	case "4h":
		a.Date = a.Date.Add(240 * time.Minute).Truncate(4 * time.Hour)
	case "next":
		a.Date = a.Date.AddDate(0, 0, 1)
		day := a.Date.Weekday()
		if day == time.Saturday {
			a.Date = a.Date.AddDate(0, 0, 2) // move two days ahead to reach Monday
		} else if day == time.Sunday {
			a.Date = a.Date.AddDate(0, 0, 1) // move one day ahead to reach Monday
		}
		a.Date = time.Date(
			a.Date.Year(),
			a.Date.Month(),
			a.Date.Day(),
			9, 30, 0, 0, loc,
		)
	default:
	}
}

type UpdatedAccountData struct {
	sync.Mutex
	db        *gorm.DB
	account   Account
	orders    []Order
	positions []Position
}

func NewUpdatedAccountData(db *gorm.DB, accountID uint) (*UpdatedAccountData, error) {
	uad := UpdatedAccountData{
		db: db,
	}
	err := uad.ReloadFromDatabase(accountID)
	if err != nil {
		return &UpdatedAccountData{}, err
	}
	return &uad, nil
}

func (uad *UpdatedAccountData) GetAccount() Account {
	uad.Lock()
	defer uad.Unlock()
	return uad.account
}

func (uad *UpdatedAccountData) GetOrders() []Order {
	uad.Lock()
	defer uad.Unlock()
	return uad.orders
}

func (uad *UpdatedAccountData) GetPositions() []Position {
	uad.Lock()
	defer uad.Unlock()
	return uad.positions
}

func (uad *UpdatedAccountData) SetAccount(account Account) {
	uad.Lock()
	defer uad.Unlock()
	uad.account = account
}

func (uad *UpdatedAccountData) SetOrders(orders []Order) {
	uad.Lock()
	defer uad.Unlock()
	uad.orders = orders
}

func (uad *UpdatedAccountData) SetPositions(positions []Position) {
	uad.Lock()
	defer uad.Unlock()
	uad.positions = positions
}

func (uad *UpdatedAccountData) ReloadFromDatabase(accountID uint) error {
	uad.Lock()
	defer uad.Unlock()

	var account Account
	var orders []Order
	var positions []Position
	var err error
	err = Transaction(uad.db, func(db *gorm.DB) error {
		account, err = GetAccountByID(db, accountID)
		if err != nil {
			return err
		}
		orders, err = GetReadyOrders(db, accountID)
		if err != nil {
			return err
		}
		positions, err = GetPositionsForAccount(db, accountID)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	uad.account = account
	uad.orders = orders
	uad.positions = positions

	return nil
}
