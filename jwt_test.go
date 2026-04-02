package xcore

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewJWTConfig(t *testing.T) {
	cfg := NewJWTConfig("secret")
	if cfg == nil {
		t.Error("NewJWTConfig returned nil")
	}
	if cfg.Secret != "secret" {
		t.Errorf("expected secret 'secret', got '%s'", cfg.Secret)
	}
	if cfg.Algorithm != "HS256" {
		t.Errorf("expected algorithm HS256, got %s", cfg.Algorithm)
	}
	if cfg.Expiration != 24*time.Hour {
		t.Errorf("expected expiration 24h, got %v", cfg.Expiration)
	}
	if cfg.HeaderName != "Authorization" {
		t.Errorf("expected header Authorization, got %s", cfg.HeaderName)
	}
	if cfg.CookieName != "token" {
		t.Errorf("expected cookie name token, got %s", cfg.CookieName)
	}
}

func TestJWTConfig_WithAlgorithm(t *testing.T) {
	cfg := NewJWTConfig("secret").WithAlgorithm("HS384")
	if cfg.Algorithm != "HS384" {
		t.Errorf("expected algorithm HS384, got %s", cfg.Algorithm)
	}
}

func TestJWTConfig_WithExpiration(t *testing.T) {
	cfg := NewJWTConfig("secret").WithExpiration(1 * time.Hour)
	if cfg.Expiration != 1*time.Hour {
		t.Errorf("expected expiration 1h, got %v", cfg.Expiration)
	}
}

func TestJWTConfig_WithCookieName(t *testing.T) {
	cfg := NewJWTConfig("secret").WithCookieName("my_token")
	if cfg.CookieName != "my_token" {
		t.Errorf("expected cookie name my_token, got %s", cfg.CookieName)
	}
}

func TestJWTConfig_WithCookieHTTPOnly(t *testing.T) {
	cfg := NewJWTConfig("secret").WithCookieHTTPOnly(false)
	if cfg.CookieHTTPOnly != false {
		t.Errorf("expected httpOnly false, got %v", cfg.CookieHTTPOnly)
	}
}

func TestJWTConfig_WithCookieSecure(t *testing.T) {
	cfg := NewJWTConfig("secret").WithCookieSecure(true)
	if cfg.CookieSecure != true {
		t.Errorf("expected secure true, got %v", cfg.CookieSecure)
	}
}

func TestJWTConfig_WithCookieSameSite(t *testing.T) {
	cfg := NewJWTConfig("secret").WithCookieSameSite(http.SameSiteStrictMode)
	if cfg.CookieSameSite != http.SameSiteStrictMode {
		t.Errorf("expected sameSite StrictMode, got %v", cfg.CookieSameSite)
	}
}

func TestJWTConfig_WithContextKey(t *testing.T) {
	cfg := NewJWTConfig("secret").WithContextKey("custom_user")
	if cfg.ContextKey != "custom_user" {
		t.Errorf("expected context key custom_user, got %s", cfg.ContextKey)
	}
}

func TestJWTConfig_WithRSAPublicKey_InvalidPEM(t *testing.T) {
	cfg := NewJWTConfig("secret")
	_, err := cfg.WithRSAPublicKey([]byte("invalid pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestJWTConfig_WithRSAPrivateKey_InvalidPEM(t *testing.T) {
	cfg := NewJWTConfig("secret")
	_, err := cfg.WithRSAPrivateKey([]byte("invalid pem"))
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

func TestNewJWTMiddleware_NilConfig(t *testing.T) {
	mw := NewJWTMiddleware(nil)
	if mw == nil {
		t.Error("NewJWTMiddleware returned nil")
	}
}

func TestNewJWTMiddleware_WithSecret(t *testing.T) {
	cfg := NewJWTConfig("mysecret")
	mw := NewJWTMiddleware(cfg)
	if mw.signKey == nil {
		t.Error("signKey should not be nil")
	}
}

func TestJWTMiddleware_Exclude(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)
	result := mw.Exclude("/public", "/health")
	if result != mw {
		t.Error("Exclude should return self")
	}
}

func TestJWTMiddleware_GenerateToken(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, err := mw.GenerateToken(claims)
	if err != nil {
		t.Errorf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Error("GenerateToken returned empty token")
	}
}

func TestJWTMiddleware_GenerateToken_CustomExpiration(t *testing.T) {
	cfg := NewJWTConfig("secret").WithExpiration(1 * time.Hour)
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, err := mw.GenerateToken(claims)
	if err != nil {
		t.Errorf("GenerateToken failed: %v", err)
	}

	parsedToken, _ := jwt.ParseWithClaims(token, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	parsedClaims := parsedToken.Claims.(*JWTClaims)

	expiresAt := parsedClaims.ExpiresAt.Time
	if expiresAt.Sub(time.Now()) > 1*time.Hour+time.Minute {
		t.Error("token expiration too far in future")
	}
}

func TestJWTMiddleware_ParseToken(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, _ := mw.GenerateToken(claims)

	parsedClaims, err := mw.ParseToken(token)
	if err != nil {
		t.Errorf("ParseToken failed: %v", err)
	}
	if parsedClaims == nil {
		t.Error("ParseToken returned nil claims")
	}
}

func TestJWTMiddleware_ParseToken_Invalid(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	_, err := mw.ParseToken("invalid-token")
	if err == nil {
		t.Error("ParseToken should fail for invalid token")
	}
}

func TestJWTMiddleware_SetTokenCookie(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	w := httptest.NewRecorder()
	mw.SetTokenCookie(w, "test-token")

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Errorf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].Name != "token" {
		t.Errorf("expected cookie name 'token', got '%s'", cookies[0].Name)
	}
	if cookies[0].Value != "test-token" {
		t.Errorf("expected cookie value 'test-token', got '%s'", cookies[0].Value)
	}
}

func TestJWTMiddleware_ClearTokenCookie(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	w := httptest.NewRecorder()
	mw.ClearTokenCookie(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Errorf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookies[0].MaxAge)
	}
}

func TestJWTMiddleware_ExtractToken_FromHeader(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer my-token")

	token, err := mw.extractToken(req)
	if err != nil {
		t.Errorf("extractToken failed: %v", err)
	}
	if token != "my-token" {
		t.Errorf("expected 'my-token', got '%s'", token)
	}
}

func TestJWTMiddleware_ExtractToken_FromCookie(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "cookie-token"})

	token, err := mw.extractToken(req)
	if err != nil {
		t.Errorf("extractToken failed: %v", err)
	}
	if token != "cookie-token" {
		t.Errorf("expected 'cookie-token', got '%s'", token)
	}
}

func TestJWTMiddleware_ExtractToken_NoToken(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	req := httptest.NewRequest("GET", "/", nil)

	_, err := mw.extractToken(req)
	if err == nil {
		t.Error("extractToken should fail when no token present")
	}
}

func TestJWTMiddleware_Middleware_ExcludePath(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg).Exclude("/public")

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/public", nil)

	mw.Middleware(handler).ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for excluded path")
	}
}

func TestJWTMiddleware_Middleware_NoToken(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/private", nil)

	mw.Middleware(handler).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestJWClaims_NewJWTClaims(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	if claims.UserID != "user1" {
		t.Errorf("expected userID user1, got %s", claims.UserID)
	}
	if claims.Username != "john" {
		t.Errorf("expected username john, got %s", claims.Username)
	}
	if claims.Email != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role admin, got %s", claims.Role)
	}
}

func TestJWTClaims_GetUserID(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	if claims.GetUserID() != "user1" {
		t.Error("GetUserID returned wrong value")
	}
}

func TestJWTClaims_GetUsername(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	if claims.GetUsername() != "john" {
		t.Error("GetUsername returned wrong value")
	}
}

func TestJWTClaims_GetEmail(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	if claims.GetEmail() != "john@example.com" {
		t.Error("GetEmail returned wrong value")
	}
}

func TestJWTClaims_GetRole(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	if claims.GetRole() != "admin" {
		t.Error("GetRole returned wrong value")
	}
}

func TestGetJWTClaims_Exists(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	ctx := SetUserForTest(claims)

	retrieved := GetJWTClaims(ctx)
	if retrieved == nil {
		t.Error("GetJWTClaims returned nil")
	}
	if retrieved.UserID != "user1" {
		t.Errorf("expected userID user1, got %s", retrieved.UserID)
	}
}

func TestGetJWTClaims_NotExists(t *testing.T) {
	ctx := context.Background()
	retrieved := GetJWTClaims(ctx)
	if retrieved != nil {
		t.Error("GetJWTClaims should return nil when no user in context")
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	ctx := SetUserForTest(claims)

	userID := GetUserIDFromContext(ctx)
	if userID != "user1" {
		t.Errorf("expected userID user1, got %s", userID)
	}
}

func TestGetUserFromContext(t *testing.T) {
	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	ctx := SetUserForTest(claims)

	userID, username, email := GetUserFromContext(ctx)
	if userID != "user1" || username != "john" || email != "john@example.com" {
		t.Errorf("expected user1/john/john@example.com, got %s/%s/%s", userID, username, email)
	}
}

func TestJWTMiddleware_ExtractClaims(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, _ := mw.GenerateToken(claims)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	extracted, err := mw.ExtractClaims(req)
	if err != nil {
		t.Errorf("ExtractClaims failed: %v", err)
	}
	if extracted == nil {
		t.Error("ExtractClaims returned nil")
	}
}

func TestJWTMiddleware_GenerateTokenWithClaims_Valid(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, err := mw.GenerateTokenWithClaims(claims)
	if err != nil {
		t.Errorf("GenerateTokenWithClaims failed: %v", err)
	}
	if token == "" {
		t.Error("GenerateTokenWithClaims returned empty token")
	}
}

func TestJWTMiddleware_GenerateTokenWithClaims_InvalidType(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	_, err := mw.GenerateTokenWithClaims(&jwt.RegisteredClaims{})
	if err == nil {
		t.Error("GenerateTokenWithClaims should fail for invalid claims type")
	}
}

func TestJWTMiddleware_DifferentAlgorithms(t *testing.T) {
	algorithms := []string{"HS256", "HS384", "HS512"}

	for _, alg := range algorithms {
		cfg := NewJWTConfig("secret").WithAlgorithm(alg)
		mw := NewJWTMiddleware(cfg)

		claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
		token, err := mw.GenerateToken(claims)
		if err != nil {
			t.Errorf("GenerateToken failed for %s: %v", alg, err)
		}

		parsed, err := mw.ParseToken(token)
		if err != nil {
			t.Errorf("ParseToken failed for %s: %v", alg, err)
		}
		if parsed == nil {
			t.Errorf("ParseToken returned nil for %s", alg)
		}
	}
}

func TestJWTMiddleware_Middleware_ValidToken(t *testing.T) {
	cfg := NewJWTConfig("secret")
	mw := NewJWTMiddleware(cfg)

	claims := NewJWTClaims("user1", "john", "john@example.com", "admin")
	token, _ := mw.GenerateToken(claims)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		userID := GetUserIDFromContext(r.Context())
		if userID != "user1" {
			t.Errorf("expected userID user1 in context, got %s", userID)
		}
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	mw.Middleware(handler).ServeHTTP(w, req)

	if !handlerCalled {
		t.Error("handler should be called for valid token")
	}
}

// Helper to set user in context for testing
func SetUserForTest(claims *JWTClaims) context.Context {
	return context.WithValue(context.Background(), "user", claims)
}
