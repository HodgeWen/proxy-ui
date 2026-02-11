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
