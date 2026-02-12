package db

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Init(path string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return err
	}
	if err := DB.AutoMigrate(&Admin{}, &Inbound{}, &Certificate{}, &User{}); err != nil {
		return err
	}
	return backfillSubscriptionTokens()
}

// backfillSubscriptionTokens sets SubscriptionToken for existing users with empty token.
func backfillSubscriptionTokens() error {
	var users []User
	if err := DB.Where("COALESCE(subscription_token, '') = ?", "").Find(&users).Error; err != nil {
		return err
	}
	for i := range users {
		users[i].SubscriptionToken = GenerateSubscriptionToken()
		if err := DB.Model(&users[i]).Update("subscription_token", users[i].SubscriptionToken).Error; err != nil {
			return err
		}
	}
	return nil
}
