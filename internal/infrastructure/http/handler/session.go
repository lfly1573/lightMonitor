package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"lightmonitor/internal/application/core"
)

const (
	SessionCookieName = "lightmonitor_session"
	userContextKey    = "current_user"
)

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]core.User
}

func NewSessionManager() *SessionManager {
	return &SessionManager{sessions: make(map[string]core.User)}
}

func (m *SessionManager) Create(c *gin.Context, user core.User) {
	token := newToken()
	m.mu.Lock()
	m.sessions[token] = user
	m.mu.Unlock()

	c.SetCookie(SessionCookieName, token, 86400, "/", "", false, true)
	c.Header("X-Session-Token", token)
}

func (m *SessionManager) Destroy(c *gin.Context) {
	token := m.tokenFromRequest(c)
	if token != "" {
		m.mu.Lock()
		delete(m.sessions, token)
		m.mu.Unlock()
	}
	c.SetCookie(SessionCookieName, "", -1, "/", "", false, true)
}

func (m *SessionManager) Require() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.tokenFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Fail(401, "unauthorized"))
			return
		}

		m.mu.RLock()
		user, ok := m.sessions[token]
		m.mu.RUnlock()
		if !ok || !user.Enabled {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Fail(401, "unauthorized"))
			return
		}

		c.Set(userContextKey, user)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok || user.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, Fail(403, "forbidden"))
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (core.User, bool) {
	value, ok := c.Get(userContextKey)
	if !ok {
		return core.User{}, false
	}
	user, ok := value.(core.User)
	return user, ok
}

func (m *SessionManager) tokenFromRequest(c *gin.Context) string {
	if token, err := c.Cookie(SessionCookieName); err == nil && token != "" {
		return token
	}
	header := c.GetHeader("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}
	return c.GetHeader("X-Session-Token")
}

func newToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}
