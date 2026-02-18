package rbac

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// RBAC errors
var (
	ErrRoleNotFound           = errors.New("role not found")
	ErrPermissionNotFound     = errors.New("permission not found")
	ErrUserNotFound           = errors.New("user not found")
	ErrPermissionDenied       = errors.New("permission denied")
	ErrSystemRoleCannotDelete = errors.New("system role cannot be deleted")
	ErrInvalidPermissionName  = errors.New("invalid permission name format")
)

// RBAC manages role-based access control
type RBAC struct {
	db           *gorm.DB
	logger       *zerolog.Logger
	cache        *permissionCache
	mu           sync.RWMutex
	autoRegister bool // Auto-register permissions from routes
}

// Config holds RBAC configuration
type Config struct {
	// EnableCache enables permission caching
	EnableCache bool
	// CacheTTL is the cache time-to-live
	CacheTTL time.Duration
	// AutoRegister enables auto-registration of permissions from routes
	AutoRegister bool
	// Logger is the zerolog logger
	Logger *zerolog.Logger
}

// DefaultConfig returns a default RBAC configuration
func DefaultConfig() *Config {
	return &Config{
		EnableCache:  true,
		CacheTTL:     5 * time.Minute,
		AutoRegister: true,
	}
}

// New creates a new RBAC instance
func New(db *gorm.DB, cfg *Config) (*RBAC, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if db == nil {
		return nil, errors.New("database connection is required")
	}

	logger := cfg.Logger
	if logger == nil {
		// Create default logger
		l := zerolog.Nop()
		logger = &l
	}

	rbac := &RBAC{
		db:           db,
		logger:       logger,
		autoRegister: cfg.AutoRegister,
	}

	// Initialize cache if enabled
	if cfg.EnableCache {
		rbac.cache = newPermissionCache(cfg.CacheTTL)
	}

	return rbac, nil
}

// Initialize runs database migrations and creates default roles
func (r *RBAC) Initialize() error {
	r.logger.Info().Msg("Initializing RBAC...")

	// Auto-migrate models
	if err := r.db.AutoMigrate(&Role{}, &Permission{}, &RolePermission{}, &RoutePermission{}, &User{}); err != nil {
		return fmt.Errorf("failed to migrate RBAC models: %w", err)
	}

	// Create default roles
	if err := r.createDefaultRoles(); err != nil {
		return fmt.Errorf("failed to create default roles: %w", err)
	}

	r.logger.Info().Msg("RBAC initialized successfully")
	return nil
}

// createDefaultRoles creates the default roles (root, user)
func (r *RBAC) createDefaultRoles() error {
	roles := []struct {
		Name      string
		Slug      string
		IsSystem  bool
		IsDefault bool
	}{
		{"Root", "root", true, false},
		{"User", "user", false, true},
	}

	for _, roleData := range roles {
		var role Role
		result := r.db.Where("slug = ?", roleData.Slug).First(&role)

		if result.Error == gorm.ErrRecordNotFound {
			role = Role{
				Name:      roleData.Name,
				Slug:      roleData.Slug,
				IsSystem:  roleData.IsSystem,
				IsDefault: roleData.IsDefault,
			}
			if err := r.db.Create(&role).Error; err != nil {
				return err
			}
			r.logger.Info().Str("role", role.Slug).Msg("Created default role")
		}
	}

	return nil
}

// ============================================================================
// Role Management
// ============================================================================

// CreateRole creates a new role
func (r *RBAC) CreateRole(ctx context.Context, req *CreateRoleRequest) (*Role, error) {
	role := &Role{
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		IsDefault:   req.IsDefault,
	}

	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return nil, err
	}

	r.logger.Info().Str("role", role.Slug).Msg("Role created")
	return role, nil
}

// GetRoleByID gets a role by ID
func (r *RBAC) GetRoleByID(ctx context.Context, id uint) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).
		Preload("Permissions").
		First(&role, id).Error; err != nil {
		return nil, ErrRoleNotFound
	}
	return &role, nil
}

// GetRoleBySlug gets a role by slug
func (r *RBAC) GetRoleBySlug(ctx context.Context, slug string) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).
		Preload("Permissions").
		Where("slug = ?", slug).
		First(&role).Error; err != nil {
		return nil, ErrRoleNotFound
	}
	return &role, nil
}

// ListRoles lists all roles
func (r *RBAC) ListRoles(ctx context.Context) ([]Role, error) {
	var roles []Role
	if err := r.db.WithContext(ctx).
		Preload("Permissions").
		Order("id ASC").
		Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

// UpdateRole updates a role
func (r *RBAC) UpdateRole(ctx context.Context, id uint, req *UpdateRoleRequest) (*Role, error) {
	role, err := r.GetRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		role.Name = req.Name
	}
	if req.Description != "" {
		role.Description = req.Description
	}
	role.IsDefault = req.IsDefault

	if err := r.db.WithContext(ctx).Save(role).Error; err != nil {
		return nil, err
	}

	r.invalidateRoleCache(role.ID)
	r.logger.Info().Uint("role_id", id).Msg("Role updated")
	return role, nil
}

// DeleteRole deletes a role (cannot delete system roles)
func (r *RBAC) DeleteRole(ctx context.Context, id uint) error {
	role, err := r.GetRoleByID(ctx, id)
	if err != nil {
		return err
	}

	if role.IsSystem {
		return ErrSystemRoleCannotDelete
	}

	// Reassign users with this role to default role
	defaultRole, _ := r.GetDefaultRole(ctx)
	if defaultRole != nil {
		r.db.WithContext(ctx).Model(&User{}).Where("role_id = ?", id).Update("role_id", defaultRole.ID)
	}

	if err := r.db.WithContext(ctx).Delete(role).Error; err != nil {
		return err
	}

	r.invalidateRoleCache(role.ID)
	r.logger.Info().Uint("role_id", id).Msg("Role deleted")
	return nil
}

// GetDefaultRole gets the default role for new users
func (r *RBAC) GetDefaultRole(ctx context.Context) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Where("is_default = ?", true).First(&role).Error; err != nil {
		return nil, ErrRoleNotFound
	}
	return &role, nil
}

// AssignPermissions assigns permissions to a role
func (r *RBAC) AssignPermissions(ctx context.Context, roleID uint, req *AssignPermissionsRequest) error {
	// Verify role exists
	_, err := r.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing permissions
		if err := tx.Where("role_id = ?", roleID).Delete(&RolePermission{}).Error; err != nil {
			return err
		}

		// Add new permissions
		for _, pid := range req.PermissionIDs {
			rp := RolePermission{
				RoleID:       roleID,
				PermissionID: pid,
				IsDeny:       false,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}

		// Add denied permissions
		for _, pid := range req.DenyIDs {
			rp := RolePermission{
				RoleID:       roleID,
				PermissionID: pid,
				IsDeny:       true,
			}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// ============================================================================
// Permission Management
// ============================================================================

// CreatePermission creates a new permission
func (r *RBAC) CreatePermission(ctx context.Context, req *CreatePermissionRequest) (*Permission, error) {
	permission := &Permission{
		Namespace:   req.Namespace,
		Resource:    req.Resource,
		Action:      req.Action,
		Description: req.Description,
	}

	// Generate name if not provided
	if req.Name != "" {
		permission.Name = req.Name
	} else {
		if req.Resource != "" {
			permission.Name = fmt.Sprintf("%s.%s.%s", req.Namespace, req.Resource, req.Action)
			permission.Levels = 3
		} else {
			permission.Name = fmt.Sprintf("%s.%s", req.Namespace, req.Action)
			permission.Levels = 2
		}
	}

	// Parse and validate name
	if err := r.parsePermissionName(permission); err != nil {
		return nil, err
	}

	if err := r.db.WithContext(ctx).Create(permission).Error; err != nil {
		return nil, err
	}

	r.logger.Info().Str("permission", permission.Name).Msg("Permission created")
	return permission, nil
}

// parsePermissionName parses a permission name into namespace, resource, action
func (r *RBAC) parsePermissionName(p *Permission) error {
	parts := strings.Split(p.Name, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return ErrInvalidPermissionName
	}

	p.Namespace = parts[0]
	p.Levels = len(parts)

	if len(parts) == 2 {
		p.Action = parts[1]
		p.Resource = ""
	} else {
		p.Resource = parts[1]
		p.Action = parts[2]
	}

	return nil
}

// GetPermissionByID gets a permission by ID
func (r *RBAC) GetPermissionByID(ctx context.Context, id uint) (*Permission, error) {
	var permission Permission
	if err := r.db.WithContext(ctx).First(&permission, id).Error; err != nil {
		return nil, ErrPermissionNotFound
	}
	return &permission, nil
}

// GetPermissionByName gets a permission by name
func (r *RBAC) GetPermissionByName(ctx context.Context, name string) (*Permission, error) {
	var permission Permission
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&permission).Error; err != nil {
		return nil, ErrPermissionNotFound
	}
	return &permission, nil
}

// ListPermissions lists all permissions
func (r *RBAC) ListPermissions(ctx context.Context) ([]Permission, error) {
	var permissions []Permission
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

// ListPermissionsByNamespace lists permissions by namespace
func (r *RBAC) ListPermissionsByNamespace(ctx context.Context, namespace string) ([]Permission, error) {
	var permissions []Permission
	if err := r.db.WithContext(ctx).Where("namespace = ?", namespace).Order("name ASC").Find(&permissions).Error; err != nil {
		return nil, err
	}
	return permissions, nil
}

// UpdatePermission updates a permission
func (r *RBAC) UpdatePermission(ctx context.Context, id uint, req *UpdatePermissionRequest) (*Permission, error) {
	permission, err := r.GetPermissionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != "" {
		permission.Description = req.Description
	}

	if err := r.db.WithContext(ctx).Save(permission).Error; err != nil {
		return nil, err
	}

	r.logger.Info().Str("permission", permission.Name).Msg("Permission updated")
	return permission, nil
}

// DeletePermission deletes a permission
func (r *RBAC) DeletePermission(ctx context.Context, id uint) error {
	permission, err := r.GetPermissionByID(ctx, id)
	if err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Delete(permission).Error; err != nil {
		return err
	}

	r.logger.Info().Str("permission", permission.Name).Msg("Permission deleted")
	return nil
}

// ============================================================================
// User Permission Management
// ============================================================================

// AssignRoleToUser assigns a role to a user
func (r *RBAC) AssignRoleToUser(ctx context.Context, userID, roleID uint) error {
	// Verify role exists
	_, err := r.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", userID).Update("role_id", roleID).Error; err != nil {
		return err
	}

	r.logger.Info().Uint("user_id", userID).Uint("role_id", roleID).Msg("Role assigned to user")
	return nil
}

// GetUserPermissions gets all permissions for a user
func (r *RBAC) GetUserPermissions(ctx context.Context, userID uint) ([]string, []uint, error) {
	// Check cache first
	if r.cache != nil {
		if cached, found := r.cache.GetUserPermissions(userID); found {
			return cached.Allowed, cached.Denied, nil
		}
	}

	var user User
	if err := r.db.WithContext(ctx).
		Preload("Role.Permissions").
		First(&user, userID).Error; err != nil {
		return nil, nil, ErrUserNotFound
	}

	if user.Role == nil {
		return []string{}, []uint{}, nil
	}

	// Get role permissions
	var rolePermissions []RolePermission
	if err := r.db.WithContext(ctx).
		Where("role_id = ?", user.Role.ID).
		Find(&rolePermissions).Error; err != nil {
		return nil, nil, err
	}

	allowed := make([]string, 0)
	denied := make([]uint, 0)

	for _, rp := range rolePermissions {
		var perm Permission
		if err := r.db.WithContext(ctx).First(&perm, rp.PermissionID).Error; err != nil {
			continue
		}

		if rp.IsDeny {
			denied = append(denied, rp.PermissionID)
		} else {
			allowed = append(allowed, perm.Name)
		}
	}

	// Cache the result
	if r.cache != nil {
		r.cache.SetUserPermissions(userID, allowed, denied)
	}

	return allowed, denied, nil
}

// HasPermission checks if a user has a specific permission
func (r *RBAC) HasPermission(ctx context.Context, userID uint, permission string) (bool, error) {
	allowed, denied, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	// Check explicit denies first
	for _, pid := range denied {
		perm, err := r.GetPermissionByID(ctx, pid)
		if err != nil {
			continue
		}
		if r.matchPermission(permission, perm.Name) {
			return false, nil
		}
	}

	// Check if root role (has all permissions)
	userRole, _ := r.GetUserRole(ctx, userID)
	if userRole != nil && userRole.Slug == "root" {
		return true, nil
	}

	// Check allowed permissions with wildcard matching
	for _, p := range allowed {
		if r.matchPermission(permission, p) {
			return true, nil
		}
	}

	return false, nil
}

// matchPermission checks if a permission matches a pattern (supports wildcards)
func (r *RBAC) matchPermission(requested, pattern string) bool {
	// Exact match
	if requested == pattern {
		return true
	}

	// Wildcard matching
	reqParts := strings.Split(requested, ".")
	patParts := strings.Split(pattern, ".")

	// Root has all permissions
	if pattern == "*" || pattern == "*.*" || pattern == "*.*.*" {
		return true
	}

	// Check each level with wildcard support
	for i := 0; i < len(reqParts) && i < len(patParts); i++ {
		if patParts[i] == "*" {
			continue // Wildcard matches anything
		}
		if reqParts[i] != patParts[i] {
			return false
		}
	}

	// If pattern is shorter, it's a namespace match (e.g., "admin.*" matches "admin.user.list")
	return len(patParts) <= len(reqParts)
}

// GetUserRole gets the role of a user
func (r *RBAC) GetUserRole(ctx context.Context, userID uint) (*Role, error) {
	var user User
	if err := r.db.WithContext(ctx).Preload("Role").First(&user, userID).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return user.Role, nil
}

// ============================================================================
// Cache Invalidation
// ============================================================================

func (r *RBAC) invalidateRoleCache(roleID uint) {
	if r.cache != nil {
		// Find all users with this role and invalidate their cache
		var users []User
		if err := r.db.Where("role_id = ?", roleID).Find(&users).Error; err != nil {
			return
		}
		for _, user := range users {
			r.cache.InvalidateUser(user.ID)
		}
	}
}

// ClearCache clears all cached permissions
func (r *RBAC) ClearCache() {
	if r.cache != nil {
		r.cache.Clear()
		r.logger.Debug().Msg("RBAC cache cleared")
	}
}
