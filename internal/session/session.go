package session

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/sqlite3store"
	"gorm.io/gorm"
)

const SessionKeyUserID = "user_id"

func NewManager(db *gorm.DB) (*scs.SessionManager, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	_, err = sqlDB.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		data BLOB,
		expiry REAL
	)`)
	if err != nil {
		return nil, err
	}
	store := sqlite3store.New(sqlDB)

	sm := scs.New()
	sm.Store = store
	sm.Lifetime = 7 * 24 * time.Hour
	sm.Cookie.HttpOnly = true
	sm.Cookie.Secure = false // set true when behind HTTPS
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Persist = true

	return sm, nil
}
