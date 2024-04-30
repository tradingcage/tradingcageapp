package database

import (
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username              string `gorm:"uniqueIndex;not null"`
	Password              string
	HasAccessAnyway       bool
	StripeCustomerID      *string `gorm:"uniqueIndex"`
	SubscriptionStartedAt *time.Time
	SubscriptionEndedAt   *time.Time
}

func (u *User) Insert(db *gorm.DB) error {
	if err := u.hashPassword(); err != nil {
		return err
	}

	result := db.Create(u)
	return result.Error
}

func (u *User) IsValid(db *gorm.DB) (bool, error) {
	if u.Username == "" || u.Password == "" {
		return false, errors.New("username or password not provided")
	}

	var user User
	result := db.Where("username = ?", u.Username).First(&user)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, result.Error
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(u.Password)); err != nil {
		return false, nil
	}

	u.ID = user.ID

	return true, nil
}

func (u *User) Fill(db *gorm.DB) error {
	result := db.Where("username = ?", u.Username).First(u)
	return result.Error
}

func (u *User) hashPassword() error {
	if len(u.Password) == 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hash)
	return nil
}

func (u *User) Upsert(db *gorm.DB) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err = db.Model(u).Updates(User{Password: string(hashedPassword)}).Error; err != nil {
		return err
	}
	return nil
}

func GetUserByUsername(db *gorm.DB, username string) (User, error) {
	var user User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return User{}, result.Error
	}
	return user, nil
}

func GetUserByID(db *gorm.DB, userID uint) (User, error) {
	var user User
	if err := db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return User{}, fmt.Errorf("user with id %d not found", userID)
		}
		return User{}, err
	}
	return user, nil
}

func (u *User) HasActiveSubscription() bool {
	now := time.Now()
	return u.HasAccessAnyway ||
		((u.SubscriptionStartedAt != nil && now.After(*u.SubscriptionStartedAt)) &&
			(u.SubscriptionEndedAt == nil || now.Before(*u.SubscriptionEndedAt)))
}
