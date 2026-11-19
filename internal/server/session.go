package server

import (
	"sync"
	"time"

	"crypto/rand"

	"proxy-ui/internal/api"
)

// SessionStore 内存会话存储；进程重启后会话失效，需重新登录。
type SessionStore struct {
	mu      sync.Mutex
	entries map[string]time.Time // 令牌 → 过期时间
}

// NewSessionStore 构造会话存储。
func NewSessionStore() *SessionStore {
	return &SessionStore{entries: make(map[string]time.Time)}
}

// Create 创建会话并返回随机令牌。
func (s *SessionStore) Create() string {
	token := rand.Text()
	s.mu.Lock()
	defer s.mu.Unlock()
	// 顺带清理过期会话，单管理员规模下开销可忽略
	now := time.Now()
	for t, expire := range s.entries {
		if expire.Before(now) {
			delete(s.entries, t)
		}
	}
	s.entries[token] = now.Add(api.SessionTTL)
	return token
}

// Valid 校验令牌是否为未过期会话。
func (s *SessionStore) Valid(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	expire, ok := s.entries[token]
	return ok && expire.After(time.Now())
}

// Remove 注销会话。
func (s *SessionStore) Remove(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, token)
}
