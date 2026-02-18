package rbac

import (
	"time"

	"gorm.io/gorm"
)

// Role represents a user role in the system
type Role struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:50;uniqueIndex;not null" json:"name"` // e.g., "root", "user", "admin"
	Slug        string         `gorm:"size:50;uniqueIndex;not null" json:"slug"` // URL-friendly version
	Description string         `gorm:"size:255" json:"description"`
	IsDefault   bool           `gorm:"default:false" json:"is_default"` // Default role for new users
	IsSystem    bool           `gorm:"default:false" json:"is_system"`  // System role (cannot be deleted)
	Permissions []Permission   `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
	Users       []User         `gorm:"foreignKey:RoleID" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Permission represents a permission in the system
// Name format: "namespace.resource.action" (e.g., "user.wallet.withdraw")
type Permission struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;uniqueIndex;not null" json:"name"` // Full name
	Namespace   string         `gorm:"size:50;index" json:"namespace"`            // First level (e.g., "user", "admin")
	Resource    string         `gorm:"size:50;index" json:"resource"`             // Second level (e.g., "wallet")
	Action      string         `gorm:"size:50;index" json:"action"`               // Third level (e.g., "withdraw")
	Levels      int            `gorm:"default:3" json:"levels"`                   // Number of levels (2 or 3)
	Description string         `gorm:"size:255" json:"description"`
	Roles       []Role         `gorm:"many2many:role_permissions;" json:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// RolePermission represents the many-to-many relationship with deny support
type RolePermission struct {
	RoleID       uint `gorm:"primaryKey;autoIncrement:false" json:"role_id"`
	PermissionID uint `gorm:"primaryKey;autoIncrement:false" json:"permission_id"`
	IsDeny       bool `gorm:"default:false" json:"is_deny"` // If true, this permission is explicitly denied
}

// User represents a user with a role
// This is a minimal interface - your actual User model may have more fields
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	RoleID    *uint          `gorm:"index" json:"role_id"` // Nullable - users without role have no permissions
	Role      *Role          `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Username  string         `gorm:"size:100" json:"username"`
	Email     string         `gorm:"size:100" json:"email"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// RoutePermission maps routes to permissions
type RoutePermission struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RoutePath    string    `gorm:"size:255;index" json:"route_path"`  // e.g., "/api/users"
	RouteName    string    `gorm:"size:100;index" json:"route_name"`  // e.g., "admin.user.list"
	Permission   string    `gorm:"size:100" json:"permission"`        // e.g., "admin.user.list"
	Method       string    `gorm:"size:10" json:"method"`             // HTTP method
	AutoRegister bool      `gorm:"default:true" json:"auto_register"` // Auto-registered from route walking
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName overrides the table name for RolePermission
func (RolePermission) TableName() string {
	return "role_permissions"
}

// ============================================================================
// Request/Response Models
// ============================================================================

// CreateRoleRequest is the request to create a role
type CreateRoleRequest struct {
	Name        string `json:"name" validate:"required"`
	Slug        string `json:"slug" validate:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// UpdateRoleRequest is the request to update a role
type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// AssignPermissionsRequest is the request to assign permissions to a role
type AssignPermissionsRequest struct {
	PermissionIDs []uint `json:"permission_ids" validate:"required"`
	DenyIDs       []uint `json:"deny_ids"` // Permissions to explicitly deny
}

// CreatePermissionRequest is the request to create a permission
type CreatePermissionRequest struct {
	Name        string `json:"name" validate:"required"` // Full name or auto-generated
	Namespace   string `json:"namespace" validate:"required"`
	Resource    string `json:"resource"`
	Action      string `json:"action" validate:"required"`
	Description string `json:"description"`
}

// UpdatePermissionRequest is the request to update a permission
type UpdatePermissionRequest struct {
	Description string `json:"description"`
}

// AssignRoleRequest is the request to assign a role to a user
type AssignRoleRequest struct {
	RoleID uint `json:"role_id" validate:"required"`
}

// RoleResponse is the response for role operations
type RoleResponse struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	IsDefault   bool     `json:"is_default"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"` // Permission names
	DenyIDs     []uint   `json:"deny_ids"`    // Denied permission IDs
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// PermissionResponse is the response for permission operations
type PermissionResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Levels      int    `json:"levels"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// UserPermissionResponse is the response for user permissions
type UserPermissionResponse struct {
	UserID      uint     `json:"user_id"`
	RoleID      *uint    `json:"role_id"`
	RoleName    *string  `json:"role_name"`
	Permissions []string `json:"permissions"`
}

// CheckPermissionRequest is the request to check permission
type CheckPermissionRequest struct {
	UserID     uint   `json:"user_id"`
	Permission string `json:"permission" validate:"required"`
}

// CheckPermissionResponse is the response for permission check
type CheckPermissionResponse struct {
	Allowed    bool   `json:"allowed"`
	Permission string `json:"permission"`
	Reason     string `json:"reason,omitempty"` // Why allowed/denied
}
