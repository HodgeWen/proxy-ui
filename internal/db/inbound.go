package db

import (
	"time"

	"gorm.io/datatypes"
)

type Inbound struct {
	ID         uint           `gorm:"primaryKey"`
	Tag        string         `gorm:"uniqueIndex;not null"`
	Protocol   string         `gorm:"not null"` // "vless" or "hysteria2"
	Listen     string         `gorm:"default:'::'"` // listen address
	ListenPort uint           `gorm:"not null"`
	ConfigJSON datatypes.JSON `gorm:"type:text"` // tls, transport, users, up_mbps, down_mbps, obfs per protocol
	CreatedAt  time.Time      `gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime"`
}

func (Inbound) TableName() string {
	return "inbounds"
}

func ListInbounds() ([]Inbound, error) {
	var inbounds []Inbound
	err := DB.Order("created_at DESC").Find(&inbounds).Error
	return inbounds, err
}

func GetInboundByID(id uint) (*Inbound, error) {
	var in Inbound
	err := DB.First(&in, id).Error
	if err != nil {
		return nil, err
	}
	return &in, nil
}

func CreateInbound(in *Inbound) error {
	return DB.Create(in).Error
}

func UpdateInbound(in *Inbound) error {
	return DB.Save(in).Error
}

func DeleteInbound(id uint) error {
	return DB.Delete(&Inbound{}, id).Error
}
