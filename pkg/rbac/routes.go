package rbac

import (
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// RouteRegistrar handles route walking and permission registration
type RouteRegistrar struct {
	rbac   *RBAC
	logger *zerolog.Logger
	router *mux.Router
}

// NewRouteRegistrar creates a new route registrar
func NewRouteRegistrar(rbac *RBAC, logger *zerolog.Logger, router *mux.Router) *RouteRegistrar {
	return &RouteRegistrar{
		rbac:   rbac,
		logger: logger,
		router: router,
	}
}

// RegisterRoutes walks all routes and registers permissions
func (rr *RouteRegistrar) RegisterRoutes(ctx context.Context) error {
	rr.logger.Info().Msg("Registering routes for RBAC...")

	var registered int
	var skipped int

	err := rr.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		// Get route name
		routeName := route.GetName()
		if routeName == "" {
			skipped++
			return nil // Skip routes without names
		}

		// Get route path
		path, err := route.GetPathTemplate()
		if err != nil {
			rr.logger.Debug().Str("route", routeName).Err(err).Msg("Failed to get path template")
			return nil
		}

		// Get route methods
		methods, err := route.GetMethods()
		if err != nil {
			methods = []string{"*"}
		}

		// Register permission for each method
		for _, method := range methods {
			// Check if permission already exists
			_, err := rr.rbac.GetPermissionByName(ctx, routeName)
			if err == ErrPermissionNotFound {
				// Create permission
				_, err := rr.rbac.CreatePermission(ctx, &CreatePermissionRequest{
					Name:        routeName,
					Description: fmt.Sprintf("Permission for %s %s", method, path),
				})
				if err != nil {
					rr.logger.Warn().Str("route", routeName).Err(err).Msg("Failed to create permission")
					continue
				}
				registered++
				rr.logger.Debug().Str("route", routeName).Str("method", method).Msg("Permission registered")
			}

			// Register route permission mapping
			routePerm := RoutePermission{
				RoutePath:    path,
				RouteName:    routeName,
				Permission:   routeName,
				Method:       method,
				AutoRegister: true,
			}

			// Check if mapping exists
			var existing RoutePermission
			result := rr.rbac.db.Where("route_name = ? AND method = ?", routeName, method).First(&existing)
			if result.Error == gorm.ErrRecordNotFound {
				if err := rr.rbac.db.Create(&routePerm).Error; err != nil {
					rr.logger.Warn().Str("route", routeName).Err(err).Msg("Failed to create route permission mapping")
				}
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk routes: %w", err)
	}

	rr.logger.Info().
		Int("registered", registered).
		Int("skipped", skipped).
		Msg("Route registration completed")

	return nil
}

// GetRoutePermissions gets all route permissions
func (rr *RouteRegistrar) GetRoutePermissions(ctx context.Context) ([]RoutePermission, error) {
	var routePermissions []RoutePermission
	if err := rr.rbac.db.WithContext(ctx).Find(&routePermissions).Error; err != nil {
		return nil, err
	}
	return routePermissions, nil
}

// SyncRoutePermissions syncs route permissions with current routes
func (rr *RouteRegistrar) SyncRoutePermissions(ctx context.Context) error {
	rr.logger.Info().Msg("Syncing route permissions...")

	// Get current route names from router
	currentRoutes := make(map[string]bool)
	err := rr.router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		if name := route.GetName(); name != "" {
			currentRoutes[name] = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Get all permissions
	permissions, err := rr.rbac.ListPermissions(ctx)
	if err != nil {
		return err
	}

	// Remove permissions for routes that no longer exist
	for _, perm := range permissions {
		if !currentRoutes[perm.Name] {
			// Check if it's an auto-registered permission
			var routePerm RoutePermission
			result := rr.rbac.db.Where("permission = ?", perm.Name).First(&routePerm)
			if result.Error == nil && routePerm.AutoRegister {
				if err := rr.rbac.DeletePermission(ctx, perm.ID); err != nil {
					rr.logger.Warn().Str("permission", perm.Name).Err(err).Msg("Failed to delete orphaned permission")
				} else {
					rr.logger.Debug().Str("permission", perm.Name).Msg("Deleted orphaned permission")
				}
			}
		}
	}

	return nil
}
