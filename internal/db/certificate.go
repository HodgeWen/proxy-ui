package db

import (
	"time"
)

type Certificate struct {
	ID            uint      `gorm:"primaryKey"`
	Name          string    `gorm:"not null"` // display label
	FullchainPath string    `gorm:"column:fullchain_path;not null"`
	PrivkeyPath   string    `gorm:"column:privkey_path;not null"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

func (Certificate) TableName() string {
	return "certificates"
}

func ListCertificates() ([]Certificate, error) {
	var certs []Certificate
	err := DB.Order("created_at DESC").Find(&certs).Error
	return certs, err
}

func GetCertificateByID(id uint) (*Certificate, error) {
	var c Certificate
	err := DB.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func CreateCertificate(c *Certificate) error {
	return DB.Create(c).Error
}

func UpdateCertificate(c *Certificate) error {
	return DB.Save(c).Error
}

func DeleteCertificate(id uint) error {
	return DB.Delete(&Certificate{}, id).Error
}
