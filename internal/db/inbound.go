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
	ListenPort     uint           `gorm:"not null"`
	ConfigJSON     datatypes.JSON `gorm:"type:text"` // tls, transport, users, up_mbps, down_mbps, obfs per protocol
	TrafficUplink  int64          `gorm:"default:0"` // bytes
	TrafficDownlink int64         `gorm:"default:0"` // bytes
	CreatedAt      time.Time     `gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `gorm:"autoUpdateTime"`
}

func (Inbound) TableName() string {
	return "inbounds"
}

// ListInbounds returns inbounds. sort: "traffic_asc", "traffic_desc", or empty (default created_at desc).
func ListInbounds(sort string) ([]Inbound, error) {
	var inbounds []Inbound
	q := DB
	switch sort {
	case "traffic_asc":
		q = q.Order("(traffic_uplink + traffic_downlink) ASC")
	case "traffic_desc":
		q = q.Order("(traffic_uplink + traffic_downlink) DESC")
	default:
		q = q.Order("created_at DESC")
	}
	err := q.Find(&inbounds).Error
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

// GetInboundsByIDs returns inbounds by IDs for user-inbound association.
func GetInboundsByIDs(ids []uint) ([]Inbound, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var inbounds []Inbound
	err := DB.Where("id IN ?", ids).Find(&inbounds).Error
	return inbounds, err
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

// GetInboundByTag returns an inbound by tag, or nil if not found.
func GetInboundByTag(tag string) (*Inbound, error) {
	var in Inbound
	err := DB.Where("tag = ?", tag).First(&in).Error
	if err != nil {
		return nil, err
	}
	return &in, nil
}

// InboundExistsByTag returns true if an inbound with the given tag exists.
func InboundExistsByTag(tag string) (bool, error) {
	var count int64
	err := DB.Model(&Inbound{}).Where("tag = ?", tag).Count(&count).Error
	return count > 0, err
}
