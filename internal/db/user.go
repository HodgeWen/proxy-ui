package db

import (
	"crypto/rand"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a proxy user (VLESS/Hysteria2 client).
type User struct {
	ID                 uint      `gorm:"primaryKey"`
	Name               string    `gorm:"size:100;not null"`
	Remark             string    `gorm:"size:255"`
	UUID               string    `gorm:"size:36;uniqueIndex"` // VLESS; auto-generated
	Password           string    `gorm:"size:255"`            // Hysteria2; auto-generated
	SubscriptionToken  string    `gorm:"size:32;uniqueIndex"`  // short token for /sub/{token}
	TrafficLimit       int64     `gorm:"default:0"`            // bytes; 0 = unlimited
	TrafficUsed        int64     `gorm:"default:0"`            // bytes
	ExpireAt           *time.Time                            // nil = no expiry
	Enabled            bool      `gorm:"default:true"`
	CreatedAt          time.Time `gorm:"autoCreateTime"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime"`
	Inbounds           []Inbound `gorm:"many2many:user_inbounds;"`
}

// generateSubscriptionToken returns a 16-char URL-safe token (a-z0-9).
func generateSubscriptionToken() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

func (User) TableName() string {
	return "users"
}

// ListUsers returns users ordered by created_at DESC.
// Optional keyword filters by name or remark (case-insensitive contains).
func ListUsers(keyword string) ([]User, error) {
	var users []User
	q := DB.Order("created_at DESC")
	if keyword != "" {
		q = q.Where("name LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	err := q.Preload("Inbounds").Find(&users).Error
	return users, err
}

// GetUserByID returns a user by ID or gorm.ErrRecordNotFound.
func GetUserByID(id uint) (*User, error) {
	var u User
	err := DB.Preload("Inbounds").First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserBySubscriptionToken returns a user by subscription token, or nil if not found.
func GetUserBySubscriptionToken(token string) (*User, error) {
	if token == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var u User
	err := DB.Preload("Inbounds").Where("subscription_token = ?", token).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateUser creates a user. Auto-generates UUID, Password, and SubscriptionToken if empty.
func CreateUser(u *User) error {
	if u.UUID == "" {
		u.UUID = uuid.NewString()
	}
	if u.Password == "" {
		u.Password = uuid.NewString()
	}
	if u.SubscriptionToken == "" {
		u.SubscriptionToken = generateSubscriptionToken()
	}
	return DB.Create(u).Error
}

// UpdateUser updates a user.
func UpdateUser(u *User) error {
	return DB.Save(u).Error
}

// ReplaceUserInbounds replaces user's inbound associations.
func ReplaceUserInbounds(userID uint, inboundIDs []uint) error {
	user, err := GetUserByID(userID)
	if err != nil {
		return err
	}
	var inbounds []Inbound
	if len(inboundIDs) > 0 {
		inbounds, err = GetInboundsByIDs(inboundIDs)
		if err != nil {
			return err
		}
	}
	return DB.Model(user).Association("Inbounds").Replace(inbounds)
}

// DeleteUser deletes a user.
func DeleteUser(id uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM user_inbounds WHERE user_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&User{}, id).Error
	})
}

// GetUsersForInbound returns users assigned to inbound ib.ID who are valid:
// Enabled == true, TrafficLimit == 0 OR TrafficUsed < TrafficLimit,
// ExpireAt == nil OR ExpireAt.After(time.Now().UTC()).
func GetUsersForInbound(inboundID uint) ([]User, error) {
	var users []User
	err := DB.Preload("Inbounds").Joins(
		"JOIN user_inbounds ON user_inbounds.user_id = users.id AND user_inbounds.inbound_id = ?",
		inboundID,
	).Find(&users).Error
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	valid := make([]User, 0, len(users))
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		if u.TrafficLimit > 0 && u.TrafficUsed >= u.TrafficLimit {
			continue
		}
		if u.ExpireAt != nil && !u.ExpireAt.After(now) {
			continue
		}
		valid = append(valid, u)
	}
	return valid, nil
}
