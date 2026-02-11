package db

import (
	"time"

	"gorm.io/gorm"
)

type Admin struct {
	ID           uint           `gorm:"primaryKey"`
	Username     string         `gorm:"uniqueIndex;size:50;not null"`
	PasswordHash string         `gorm:"column:password_hash;size:255;not null"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (Admin) TableName() string {
	return "admins"
}

func HasAdmin() (bool, error) {
	var count int64
	err := DB.Model(&Admin{}).Count(&count).Error
	return count > 0, err
}

func CreateAdmin(username, passwordHash string) error {
	return DB.Create(&Admin{
		Username:     username,
		PasswordHash: passwordHash,
	}).Error
}

func GetAdminByUsername(username string) (*Admin, error) {
	var admin Admin
	err := DB.Where("username = ?", username).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}
