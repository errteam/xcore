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

func (c *JWTConfig) WithAlgorithm(alg string) *JWTConfig {
	c.Algorithm = alg
	return c
}

func (c *JWTConfig) WithExpiration(exp time.Duration) *JWTConfig {
	c.Expiration = exp
	return c
}

func (c *JWTConfig) WithCookieName(name string) *JWTConfig {
	c.CookieName = name
	return c
}

func (c *JWTConfig) WithCookieHTTPOnly(httpOnly bool) *JWTConfig {
	c.CookieHTTPOnly = httpOnly
	return c
}

func (c *JWTConfig) WithCookieSecure(secure bool) *JWTConfig {
	c.CookieSecure = secure
	return c
}

func (c *JWTConfig) WithCookieSameSite(sameSite http.SameSite) *JWTConfig {
	c.CookieSameSite = sameSite
	return c
}

func (c *JWTConfig) WithContextKey(key string) *JWTConfig {
	c.ContextKey = key
	return c
}

func (c *JWTConfig) WithRSAPublicKey(pemData []byte) (*JWTConfig, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("invalid PEM block")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	c.VerifyKey = key.(*rsa.PublicKey)
	return c, nil
}

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

type JWTMiddleware struct {
	config    *JWTConfig
	verifyKey interface{}
	signKey   interface{}
	excluded  []string
}

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

func (m *JWTMiddleware) Exclude(paths ...string) *JWTMiddleware {
	m.excluded = append(m.excluded, paths...)
	return m
}

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

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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

func (m *JWTMiddleware) ParseToken(tokenString string) (jwt.Claims, error) {
	return m.parseToken(tokenString)
}

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

func (m *JWTMiddleware) unauthorized(w http.ResponseWriter) {
	resp := Unauthorized("Invalid or missing token")
	resp.Write(w)
}

type JWTClaims struct {
	jwt.RegisteredClaims
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

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

func (c *JWTClaims) GetUserID() string {
	return c.UserID
}

func (c *JWTClaims) GetUsername() string {
	return c.Username
}

func (c *JWTClaims) GetEmail() string {
	return c.Email
}

func (c *JWTClaims) GetRole() string {
	return c.Role
}

func GetJWTClaims(ctx context.Context) *JWTClaims {
	if c, ok := ctx.Value("user").(*JWTClaims); ok {
		return c
	}
	return nil
}

func GetUserIDFromContext(ctx context.Context) string {
	if claims := GetJWTClaims(ctx); claims != nil {
		return claims.UserID
	}
	return ""
}

func GetUserFromContext(ctx context.Context) (string, string, string) {
	if claims := GetJWTClaims(ctx); claims != nil {
		return claims.UserID, claims.Username, claims.Email
	}
	return "", "", ""
}

func (m *JWTMiddleware) ExtractClaims(r *http.Request) (jwt.Claims, error) {
	token, err := m.extractToken(r)
	if err != nil {
		return nil, err
	}
	return m.parseToken(token)
}

func (m *JWTMiddleware) GenerateTokenWithClaims(claims jwt.Claims) (string, error) {
	if c, ok := claims.(*JWTClaims); ok {
		return m.GenerateToken(c)
	}
	return "", errors.New("invalid claims type")
}
