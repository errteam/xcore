package rbac

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// contextKey is a type for context keys
type contextKey string

const (
	// ContextKeyUserID is the context key for user ID
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyUserRole is the context key for user role
	ContextKeyUserRole contextKey = "user_role"
	// ContextKeyUserPermissions is the context key for user permissions
	ContextKeyUserPermissions contextKey = "user_permissions"
)

// UserProvider is a function that extracts user ID from request
type UserProvider func(r *http.Request) (uint, error)

// PermissionMiddleware creates a middleware that checks permissions
func PermissionMiddleware(rbac *RBAC, logger *zerolog.Logger, userProvider UserProvider) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user ID from request
			userID, err := userProvider(r)
			if err != nil {
				logger.Debug().Err(err).Msg("Failed to get user from request")
				// Continue without user context - public route
				next.ServeHTTP(w, r)
				return
			}

			// Get route name (permission) from route
			routeName := mux.CurrentRoute(r).GetName()

			// If no route name, continue (no permission check needed)
			if routeName == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check permission
			hasPermission, err := rbac.HasPermission(r.Context(), userID, routeName)
			if err != nil {
				logger.Error().Err(err).Uint("user_id", userID).Str("permission", routeName).Msg("Permission check failed")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				logger.Warn().Uint("user_id", userID).Str("permission", routeName).Msg("Permission denied")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Add user info to context
			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)

			// Get user role and add to context
			role, _ := rbac.GetUserRole(r.Context(), userID)
			if role != nil {
				ctx = context.WithValue(ctx, ContextKeyUserRole, role)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequirePermission creates a middleware that requires a specific permission
func RequirePermission(rbac *RBAC, logger *zerolog.Logger, userProvider UserProvider, permission string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := userProvider(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			hasPermission, err := rbac.HasPermission(r.Context(), userID, permission)
			if err != nil {
				logger.Error().Err(err).Msg("Permission check failed")
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !hasPermission {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole creates a middleware that requires a specific role
func RequireRole(rbac *RBAC, logger *zerolog.Logger, userProvider UserProvider, roleSlug string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := userProvider(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			role, err := rbac.GetUserRole(r.Context(), userID)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			if role.Slug != roleSlug {
				logger.Warn().Uint("user_id", userID).Str("role", role.Slug).Str("required", roleSlug).Msg("Role check failed")
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
			ctx = context.WithValue(ctx, ContextKeyUserRole, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAnyRole creates a middleware that requires any of the specified roles
func RequireAnyRole(rbac *RBAC, logger *zerolog.Logger, userProvider UserProvider, roleSlugs ...string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := userProvider(r)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			role, err := rbac.GetUserRole(r.Context(), userID)
			if err != nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			for _, slug := range roleSlugs {
				if role.Slug == slug {
					ctx := context.WithValue(r.Context(), ContextKeyUserID, userID)
					ctx = context.WithValue(ctx, ContextKeyUserRole, role)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			logger.Warn().Uint("user_id", userID).Str("role", role.Slug).Strs("required", roleSlugs).Msg("Role check failed")
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

// GetUserIDFromContext gets the user ID from context
func GetUserIDFromContext(ctx context.Context) (uint, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(uint)
	return userID, ok
}

// GetUserRoleFromContext gets the user role from context
func GetUserRoleFromContext(ctx context.Context) (*Role, bool) {
	role, ok := ctx.Value(ContextKeyUserRole).(*Role)
	return role, ok
}

// DefaultUserProvider is a default user provider that gets user ID from JWT claims or session
// You should replace this with your actual user extraction logic
func DefaultUserProvider() UserProvider {
	return func(r *http.Request) (uint, error) {
		// This is a placeholder - implement your own user extraction
		// Examples:
		// - Get from JWT token claims
		// - Get from session
		// - Get from API key lookup

		// For demo purposes, get from header
		userIDStr := r.Header.Get("X-User-ID")
		if userIDStr == "" {
			return 0, errors.New("user ID not found")
		}

		// Parse user ID (implement proper parsing)
		var userID uint
		_, err := fmt.Sscanf(userIDStr, "%d", &userID)
		if err != nil {
			return 0, err
		}

		return userID, nil
	}
}
