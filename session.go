package xcore

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type SessionStore interface {
	Get(ctx context.Context, id string) (*Session, error)
	Set(ctx context.Context, session *Session) error
	Delete(ctx context.Context, id string) error
}

type Session struct {
	ID        string
	Values    map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewSession(id string) *Session {
	return &Session{
		ID:        id,
		Values:    make(map[string]interface{}),
		CreatedAt: time.Now(),
	}
}

func (s *Session) Get(key string) interface{} {
	return s.Values[key]
}

func (s *Session) Set(key string, value interface{}) {
	s.Values[key] = value
}

func (s *Session) Delete(key string) {
	delete(s.Values, key)
}

func (s *Session) Clear() {
	s.Values = make(map[string]interface{})
}

func (s *Session) Len() int {
	return len(s.Values)
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

type MemorySessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	cleanup  time.Duration
	stop     chan struct{}
}

func NewMemorySessionStore(cleanupInterval time.Duration) *MemorySessionStore {
	if cleanupInterval <= 0 {
		cleanupInterval = 10 * time.Minute
	}

	store := &MemorySessionStore{
		sessions: make(map[string]*Session),
		cleanup:  cleanupInterval,
		stop:     make(chan struct{}),
	}

	go store.cleanupExpired()

	return store
}

func (s *MemorySessionStore) Get(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, ok := s.sessions[id]; ok {
		if time.Now().Before(session.ExpiresAt) {
			return session, nil
		}
	}

	return nil, fmt.Errorf("session not found or expired: %s", id)
}

func (s *MemorySessionStore) Set(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session
	return nil
}

func (s *MemorySessionStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, id)
	return nil
}

func (s *MemorySessionStore) cleanupExpired() {
	ticker := time.NewTicker(s.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			now := time.Now()
			for id, session := range s.sessions {
				if now.After(session.ExpiresAt) {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		case <-s.stop:
			return
		}
	}
}

func (s *MemorySessionStore) Stop() {
	close(s.stop)
}

func GenerateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

type SessionManager struct {
	store      SessionStore
	cookieName string
	cookiePath string
	maxAge     int
	secure     bool
	httpOnly   bool
	sameSite   http.SameSite
}

func NewSessionManager(store SessionStore) *SessionManager {
	return &SessionManager{
		store:      store,
		cookieName: "session_id",
		cookiePath: "/",
		maxAge:     86400,
		secure:     false,
		httpOnly:   true,
		sameSite:   http.SameSiteLaxMode,
	}
}

func (m *SessionManager) WithCookieName(name string) *SessionManager {
	m.cookieName = name
	return m
}

func (m *SessionManager) WithCookiePath(path string) *SessionManager {
	m.cookiePath = path
	return m
}

func (m *SessionManager) WithMaxAge(maxAge int) *SessionManager {
	m.maxAge = maxAge
	return m
}

func (m *SessionManager) WithSecure(secure bool) *SessionManager {
	m.secure = secure
	return m
}

func (m *SessionManager) WithHTTPOnly(httpOnly bool) *SessionManager {
	m.httpOnly = httpOnly
	return m
}

func (m *SessionManager) WithSameSite(sameSite http.SameSite) *SessionManager {
	m.sameSite = sameSite
	return m
}

type SessionMiddleware struct {
	manager *SessionManager
}

func NewSessionMiddleware(manager *SessionManager) *SessionMiddleware {
	return &SessionMiddleware{manager: manager}
}

func (m *SessionMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.manager.cookieName)
		sessionID := ""

		if err == nil && cookie != nil {
			sessionID = cookie.Value
		}

		if sessionID == "" {
			newID, err := GenerateSessionID()
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			session := NewSession(newID)
			session.ExpiresAt = time.Now().Add(time.Duration(m.manager.maxAge) * time.Second)

			if err := m.manager.store.Set(r.Context(), session); err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			sessionID = newID

			http.SetCookie(w, &http.Cookie{
				Name:     m.manager.cookieName,
				Value:    sessionID,
				Path:     m.manager.cookiePath,
				MaxAge:   m.manager.maxAge,
				Secure:   m.manager.secure,
				HttpOnly: m.manager.httpOnly,
				SameSite: m.manager.sameSite,
			})
		}

		session, err := m.manager.store.Get(r.Context(), sessionID)
		if err != nil || session == nil {
			newID, genErr := GenerateSessionID()
			if genErr != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			session = NewSession(newID)
			session.ExpiresAt = time.Now().Add(time.Duration(m.manager.maxAge) * time.Second)

			if setErr := m.manager.store.Set(r.Context(), session); setErr != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     m.manager.cookieName,
				Value:    newID,
				Path:     m.manager.cookiePath,
				MaxAge:   m.manager.maxAge,
				Secure:   m.manager.secure,
				HttpOnly: m.manager.httpOnly,
				SameSite: m.manager.sameSite,
			})
		}

		ctx := context.WithValue(r.Context(), "session", session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetSession(ctx context.Context) *Session {
	if session, ok := ctx.Value("session").(*Session); ok {
		return session
	}
	return nil
}

func (m *SessionManager) Middleware(next http.Handler) http.Handler {
	return NewSessionMiddleware(m).Middleware(next)
}
