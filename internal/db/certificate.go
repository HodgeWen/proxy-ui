package db

import (
	"encoding/json"
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

// InboundsReferencingCert returns tags of inbounds whose config_json references this cert via tls.certificate_id.
func InboundsReferencingCert(certID uint) ([]string, error) {
	inbounds, err := ListInbounds()
	if err != nil {
		return nil, err
	}
	var tags []string
	for i := range inbounds {
		if len(inbounds[i].ConfigJSON) == 0 {
			continue
		}
		var cfg map[string]any
		if err := json.Unmarshal(inbounds[i].ConfigJSON, &cfg); err != nil {
			continue
		}
		tls, ok := cfg["tls"].(map[string]any)
		if !ok || tls == nil {
			continue
		}
		cid, ok := tls["certificate_id"]
		if !ok || cid == nil {
			continue
		}
		// JSON numbers decode as float64; handle int/float64/json.Number
		var id uint
		switch x := cid.(type) {
		case float64:
			if x >= 0 && x == float64(uint(x)) {
				id = uint(x)
			} else {
				continue
			}
		case int:
			if x >= 0 {
				id = uint(x)
			} else {
				continue
			}
		case int64:
			if x >= 0 {
				id = uint(x)
			} else {
				continue
			}
		default:
			continue
		}
		if id == certID {
			tags = append(tags, inbounds[i].Tag)
		}
	}
	return tags, nil
}
