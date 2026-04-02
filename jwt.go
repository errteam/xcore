// Package xcore provides JWT (JSON Web Token) authentication for the xcore framework.
//
// This package provides JWT token generation, validation, and middleware for HTTP requests.
// It supports both HMAC (HS256, HS384, HS512) and RSA (RS256, RS384, RS512) algorithms.
//
// Key features:
//   - Token generation with custom claims
//   - Token validation with multiple algorithms
//   - Context-based claims extraction
//   - Cookie-based token storage
//   - Path exclusion support
package xcore

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig defines the configuration for JWT authentication.
// It includes secret keys, algorithm selection, expiration times, and cookie settings.
type JWTConfig struct {
	Secret            string
	SignKey           interface{}
	VerifyKey         interface{}
	Algorithm         string
	Expiration        time.Duration
	RefreshExpiration time.Duration
	CookieName        string
	HeaderName        string
	Prefix            string
	Claims            jwt.Claims
	Lookup            string
	ContextKey        string
	TokenLookup       string
	CookieHTTPOnly    bool
	CookieSameSite    http.SameSite
	CookieSecure      bool
	CookiePath        string
	CookieDomain      string
}

// NewJWTConfig creates a JWTConfig with default values.
// Default settings: Algorithm: HS256, Expiration: 24h, HeaderName: Authorization, Prefix: Bearer
func NewJWTConfig(secret string) *JWTConfig {
	return &JWTConfig{
		Secret:         secret,
		Algorithm:      "HS256",
		Expiration:     24 * time.Hour,
		HeaderName:     "Authorization",
		Prefix:         "Bearer",
		CookieName:     "token",
		Lookup:         "header:Authorization",
		ContextKey:     "user",
		TokenLookup:    "header:Authorization",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteLaxMode,
		CookiePath:     "/",
	}
}

// WithAlgorithm sets the JWT signing algorithm.
// Supported values: HS256, HS384, HS512, RS256, RS384, RS512
func (c *JWTConfig) WithAlgorithm(alg string) *JWTConfig {
	c.Algorithm = alg
	return c
}

// WithExpiration sets the token expiration duration.
func (c *JWTConfig) WithExpiration(exp time.Duration) *JWTConfig {
	c.Expiration = exp
	return c
}

// WithCookieName sets the cookie name for token storage.
func (c *JWTConfig) WithCookieName(name string) *JWTConfig {
	c.CookieName = name
	return c
}

// WithCookieHTTPOnly sets the HttpOnly flag for the token cookie.
func (c *JWTConfig) WithCookieHTTPOnly(httpOnly bool) *JWTConfig {
	c.CookieHTTPOnly = httpOnly
	return c
}

// WithCookieSecure sets the Secure flag for the token cookie.
func (c *JWTConfig) WithCookieSecure(secure bool) *JWTConfig {
	c.CookieSecure = secure
	return c
}

// WithCookieSameSite sets the SameSite mode for the token cookie.
func (c *JWTConfig) WithCookieSameSite(sameSite http.SameSite) *JWTConfig {
	c.CookieSameSite = sameSite
	return c
}

// WithContextKey sets the context key for storing claims.
func (c *JWTConfig) WithContextKey(key string) *JWTConfig {
	c.ContextKey = key
	return c
}

// WithRSAPublicKey sets the RSA public key from PEM-encoded data.
// The public key is used for verifying JWT signatures.
func (c *JWTConfig) WithRSAPublicKey(pemData []byte) (*JWTConfig, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid PEM block")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pubKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("invalid key type: expected RSA public key")
	}
	c.VerifyKey = pubKey
	return c, nil
}

// WithRSAPrivateKey sets the RSA private key from PEM-encoded data.
// The private key is used for signing JWT tokens.
func (c *JWTConfig) WithRSAPrivateKey(pemData []byte) (*JWTConfig, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("invalid PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("PEM data is not an RSA private key")
	}

	c.SignKey = rsaKey
	return c, nil
}

// WithRSAPublicKeyFromPrivateKey derives the public key from the private key PEM data.
// This is a convenience method when you have only the private key.
func (c *JWTConfig) WithRSAPublicKeyFromPrivateKey(pemData []byte) (*JWTConfig, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, errors.New("invalid PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("PEM data is not an RSA private key")
	}

	c.SignKey = rsaKey
	c.VerifyKey = &rsaKey.PublicKey
	return c, nil
}

// JWTMiddleware provides JWT authentication middleware for HTTP requests.
// It validates tokens from Authorization header or cookies and extracts claims into the request context.
// Use Exclude() to specify paths that should bypass JWT validation.
type JWTMiddleware struct {
	config    *JWTConfig
	verifyKey interface{}
	signKey   interface{}
	excluded  []string
}

// NewJWTMiddleware creates a new JWT middleware with the given configuration.
// If config is nil, default configuration is used.
func NewJWTMiddleware(config *JWTConfig) *JWTMiddleware {
	if config == nil {
		config = NewJWTConfig("")
	}

	mw := &JWTMiddleware{
		config:   config,
		excluded: []string{},
	}

	if config.SignKey != nil {
		if keyBytes, ok := config.SignKey.([]byte); ok && len(keyBytes) > 0 {
			mw.signKey = keyBytes
		} else {
			mw.signKey = config.SignKey
		}
	} else if config.Secret != "" {
		mw.signKey = []byte(config.Secret)
	}

	if config.VerifyKey != nil {
		mw.verifyKey = config.VerifyKey
	}

	if config.Algorithm == "" {
		config.Algorithm = "HS256"
	}

	return mw
}

// Exclude adds paths that should bypass JWT authentication.
// Paths are matched using strings.HasPrefix, so "/path" matches "/path" and "/path/sub".
// Returns the middleware for method chaining.
func (m *JWTMiddleware) Exclude(paths ...string) *JWTMiddleware {
	m.excluded = append(m.excluded, paths...)
	return m
}

// Middleware returns an http.Handler that validates JWT tokens.
// It extracts tokens from the Authorization header (with Bearer prefix) or cookies.
// If the token is valid, claims are stored in the request context.
func (m *JWTMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, excluded := range m.excluded {
			if strings.HasPrefix(r.URL.Path, excluded) {
				next.ServeHTTP(w, r)
				return
			}
		}

		token, err := m.extractToken(r)
		if err != nil {
			m.unauthorized(w)
			return
		}

		claims, err := m.parseToken(token)
		if err != nil {
			m.unauthorized(w)
			return
		}

		ctx := r.Context()
		if m.config.ContextKey != "" {
			ctx = context.WithValue(ctx, m.config.ContextKey, claims)
		}
		if jwtClaims, ok := claims.(*JWTClaims); ok {
			ctx = context.WithValue(ctx, UserIDKey, jwtClaims.UserID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractToken extracts the JWT token from the request.
// It first checks the Authorization header, then falls back to cookies.
func (m *JWTMiddleware) extractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get(m.config.HeaderName)
	if authHeader != "" && m.config.Prefix != "" {
		if strings.HasPrefix(authHeader, m.config.Prefix+" ") {
			return strings.TrimPrefix(authHeader, m.config.Prefix+" "), nil
		}
		if strings.HasPrefix(authHeader, m.config.Prefix) {
			return strings.TrimPrefix(authHeader, m.config.Prefix), nil
		}
		return authHeader, nil
	}

	cookie, err := r.Cookie(m.config.CookieName)
	if err == nil {
		return cookie.Value, nil
	}

	return "", errors.New("no token found")
}

// parseToken parses and validates a JWT token string.
// It handles both HMAC and RSA signing methods.
// Returns the claims if valid, or an error if validation fails.
func (m *JWTMiddleware) parseToken(tokenString string) (jwt.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); ok {
			if m.verifyKey != nil {
				return m.verifyKey, nil
			}
			return m.signKey, nil
		}

		if _, ok := token.Method.(*jwt.SigningMethodRSA); ok {
			if m.verifyKey != nil {
				return m.verifyKey, nil
			}
			if m.signKey != nil {
				if rsaKey, ok := m.signKey.(*rsa.PrivateKey); ok {
					return &rsaKey.PublicKey, nil
				}
			}
			return nil, errors.New("RSA signing key not configured")
		}

		if _, ok := token.Method.(*jwt.SigningMethodECDSA); ok {
			if m.verifyKey != nil {
				return m.verifyKey, nil
			}
			return nil, errors.New("ECDSA not supported")
		}

		return nil, errors.New("unexpected signing method")
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return token.Claims, nil
}

// ParseToken is a public method to parse a token string without going through middleware.
// Useful for manual token validation in handlers.
func (m *JWTMiddleware) ParseToken(tokenString string) (jwt.Claims, error) {
	return m.parseToken(tokenString)
}

// GenerateToken creates a new JWT token with the given claims.
// Sets Expiration and IssuedAt based on config.Expiration.
// Uses the algorithm specified in the configuration.
func (m *JWTMiddleware) GenerateToken(claims *JWTClaims) (string, error) {
	if m.config.Expiration > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(m.config.Expiration))
		claims.IssuedAt = jwt.NewNumericDate(time.Now())
	}

	var signingMethod jwt.SigningMethod
	switch m.config.Algorithm {
	case "HS256":
		signingMethod = jwt.SigningMethodHS256
	case "HS384":
		signingMethod = jwt.SigningMethodHS384
	case "HS512":
		signingMethod = jwt.SigningMethodHS512
	case "RS256":
		signingMethod = jwt.SigningMethodRS256
	case "RS384":
		signingMethod = jwt.SigningMethodRS384
	case "RS512":
		signingMethod = jwt.SigningMethodRS512
	default:
		signingMethod = jwt.SigningMethodHS256
	}

	return jwt.NewWithClaims(signingMethod, claims).SignedString(m.signKey)
}

// SetTokenCookie sets a cookie containing the JWT token.
// The cookie is configured according to the JWTConfig settings (name, path, expiry, etc.).
func (m *JWTMiddleware) SetTokenCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     m.config.CookieName,
		Value:    token,
		Path:     m.config.CookiePath,
		HttpOnly: m.config.CookieHTTPOnly,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
		Expires:  time.Now().Add(m.config.Expiration),
	}

	if m.config.CookieDomain != "" {
		cookie.Domain = m.config.CookieDomain
	}

	http.SetCookie(w, cookie)
}

// ClearTokenCookie removes the JWT token cookie by setting MaxAge to -1.
// This effectively deletes the cookie from the client.
func (m *JWTMiddleware) ClearTokenCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     m.config.CookieName,
		Value:    "",
		Path:     m.config.CookiePath,
		HttpOnly: m.config.CookieHTTPOnly,
		Secure:   m.config.CookieSecure,
		SameSite: m.config.CookieSameSite,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}

	if m.config.CookieDomain != "" {
		cookie.Domain = m.config.CookieDomain
	}

	http.SetCookie(w, cookie)
}

// unauthorized sends a 401 Unauthorized response with a default message.
func (m *JWTMiddleware) unauthorized(w http.ResponseWriter) {
	resp := Unauthorized("Invalid or missing token")
	_ = resp.Write(w)
}

// JWTClaims represents the standard claims for JWT tokens.
// It extends jwt.RegisteredClaims with additional user fields.
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// NewJWTClaims creates a new JWTClaims instance with the provided user information.
// Sets default expiration to 24 hours from now.
func NewJWTClaims(userID, username, email, role string) *JWTClaims {
	return &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
	}
}

// GetUserID returns the user ID from the claims.
func (c *JWTClaims) GetUserID() string {
	return c.UserID
}

// GetUsername returns the username from the claims.
func (c *JWTClaims) GetUsername() string {
	return c.Username
}

// GetEmail returns the email from the claims.
func (c *JWTClaims) GetEmail() string {
	return c.Email
}

// GetRole returns the role from the claims.
func (c *JWTClaims) GetRole() string {
	return c.Role
}

// GetJWTClaims retrieves JWT claims from the context.
// Uses the default context key "user".
func GetJWTClaims(ctx context.Context) *JWTClaims {
	if c, ok := ctx.Value("user").(*JWTClaims); ok {
		return c
	}
	return nil
}

// GetUserIDFromContext extracts the user ID from JWT claims in the context.
// Returns empty string if no claims found.
func GetUserIDFromContext(ctx context.Context) string {
	if claims := GetJWTClaims(ctx); claims != nil {
		return claims.UserID
	}
	return ""
}

// GetUserFromContext extracts user information (ID, username, email) from JWT claims.
// Returns empty strings if no claims found.
func GetUserFromContext(ctx context.Context) (string, string, string) {
	if claims := GetJWTClaims(ctx); claims != nil {
		return claims.UserID, claims.Username, claims.Email
	}
	return "", "", ""
}

// ExtractClaims extracts and parses JWT claims from an HTTP request.
// It first extracts the token, then validates it.
func (m *JWTMiddleware) ExtractClaims(r *http.Request) (jwt.Claims, error) {
	token, err := m.extractToken(r)
	if err != nil {
		return nil, err
	}
	return m.parseToken(token)
}

// GenerateTokenWithClaims generates a token from a generic jwt.Claims.
// Returns an error if the claims cannot be cast to JWTClaims.
func (m *JWTMiddleware) GenerateTokenWithClaims(claims jwt.Claims) (string, error) {
	if c, ok := claims.(*JWTClaims); ok {
		return m.GenerateToken(c)
	}
	return "", errors.New("invalid claims type")
}
