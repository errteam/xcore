package rbac

import (
	"net/http"
	"xcore-example/pkg/xcore"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// API provides HTTP handlers for RBAC management
type API struct {
	rbac   *RBAC
	logger *zerolog.Logger
}

// NewAPI creates a new RBAC API handler
func NewAPI(rbac *RBAC, logger *zerolog.Logger) *API {
	return &API{
		rbac:   rbac,
		logger: logger,
	}
}

// RegisterRoutes registers RBAC API routes
func (a *API) RegisterRoutes(r *mux.Router) {
	// Role routes
	r.HandleFunc("/roles", a.ListRoles).Methods("GET").Name("admin.rbac.role_list")
	r.HandleFunc("/roles", a.CreateRole).Methods("POST").Name("admin.rbac.role_create")
	r.HandleFunc("/roles/{id}", a.GetRole).Methods("GET").Name("admin.rbac.get_role")
	r.HandleFunc("/roles/{id}", a.UpdateRole).Methods("PUT").Name("admin.rbac.role_update")
	r.HandleFunc("/roles/{id}", a.DeleteRole).Methods("DELETE").Name("admin.rbac.role_delete")
	r.HandleFunc("/roles/{id}/permissions", a.AssignRolePermissions).Methods("PUT").Name("admin.rbac.role_permissions")

	// Permission routes
	r.HandleFunc("/permissions", a.ListPermissions).Methods("GET").Name("admin.rbac.permission_list")
	r.HandleFunc("/permissions", a.CreatePermission).Methods("POST").Name("admin.rbac.permission_create")
	r.HandleFunc("/permissions/{id}", a.GetPermission).Methods("GET").Name("admin.rbac.get_permission")
	r.HandleFunc("/permissions/{id}", a.UpdatePermission).Methods("PUT").Name("admin.rbac.permission_update")
	r.HandleFunc("/permissions/{id}", a.DeletePermission).Methods("DELETE").Name("admin.rbac.permission_delete")

	// User permission routes
	r.HandleFunc("/users/{id}/permissions", a.GetUserPermissions).Methods("GET")
	r.HandleFunc("/users/{id}/role", a.AssignUserRole).Methods("PUT")
	r.HandleFunc("/users/{id}/check", a.CheckPermission).Methods("POST")
}

// ============================================================================
// Role Handlers
// ============================================================================

// ListRoles lists all roles
func (a *API) ListRoles(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	ctx := r.Context()
	roles, err := a.rbac.ListRoles(ctx)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to list roles")
		rb.InternalServerError("Failed to list roles")
		return
	}

	response := make([]RoleResponse, len(roles))
	for i, role := range roles {
		response[i] = a.roleToResponse(&role)
	}

	rb.OK(response)
}

// CreateRole creates a new role
func (a *API) CreateRole(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	var req CreateRoleRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	ctx := r.Context()
	role, err := a.rbac.CreateRole(ctx, &req)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to create role")
		rb.InternalServerError("Failed to create role")
		return
	}

	rb.Created("Role created successfully", a.roleToResponse(role))
}

// GetRole gets a role by ID
func (a *API) GetRole(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	ctx := r.Context()
	role, err := a.rbac.GetRoleByID(ctx, uint(id))
	if err != nil {
		rb.NotFound("Role not found")
		return
	}

	rb.OK(a.roleToResponse(role))
}

// UpdateRole updates a role
func (a *API) UpdateRole(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	var req UpdateRoleRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	ctx := r.Context()
	role, err := a.rbac.UpdateRole(ctx, uint(id), &req)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to update role")
		rb.InternalServerError("Failed to update role")
		return
	}

	rb.OK(a.roleToResponse(role))
}

// DeleteRole deletes a role
func (a *API) DeleteRole(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	ctx := r.Context()
	if err := a.rbac.DeleteRole(ctx, uint(id)); err != nil {
		if err == ErrSystemRoleCannotDelete {
			rb.Forbidden("Cannot delete system role")
			return
		}
		a.rbac.logger.Error().Err(err).Msg("Failed to delete role")
		rb.InternalServerError("Failed to delete role")
		return
	}

	rb.Deleted("Role deleted successfully")
}

// AssignRolePermissions assigns permissions to a role
func (a *API) AssignRolePermissions(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	var req AssignPermissionsRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	ctx := r.Context()
	if err := a.rbac.AssignPermissions(ctx, uint(id), &req); err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to assign permissions")
		rb.InternalServerError("Failed to assign permissions")
		return
	}

	rb.OK("Permissions assigned successfully")
}

// ============================================================================
// Permission Handlers
// ============================================================================

// ListPermissions lists all permissions
func (a *API) ListPermissions(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	ctx := r.Context()
	permissions, err := a.rbac.ListPermissions(ctx)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to list permissions")
		rb.InternalServerError("Failed to list permissions")
		return
	}

	response := make([]PermissionResponse, len(permissions))
	for i, perm := range permissions {
		response[i] = a.permissionToResponse(&perm)
	}

	rb.OK(response)
}

// CreatePermission creates a new permission
func (a *API) CreatePermission(w http.ResponseWriter, r *http.Request) {
	var req CreatePermissionRequest
	rb := xcore.NewResponseBuilder(w, r)
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	ctx := r.Context()
	permission, err := a.rbac.CreatePermission(ctx, &req)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to create permission")
		rb.InternalServerError("Failed to create permission")
		return
	}

	rb.Created("Permission created successfully", a.permissionToResponse(permission))
}

// GetPermission gets a permission by ID
func (a *API) GetPermission(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid permission ID")
		return
	}

	ctx := r.Context()
	permission, err := a.rbac.GetPermissionByID(ctx, uint(id))
	if err != nil {
		rb.NotFound("Permission not found")
		return
	}

	rb.OK(a.permissionToResponse(permission))
}

// UpdatePermission updates a permission
func (a *API) UpdatePermission(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	var req UpdatePermissionRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.BadRequest("Invalid request body")
		return
	}

	ctx := r.Context()
	permission, err := a.rbac.UpdatePermission(ctx, uint(id), &req)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to update permission")
		rb.InternalServerError("Failed to update permission")
		return
	}

	rb.OK(a.permissionToResponse(permission))
}

// DeletePermission deletes a permission
func (a *API) DeletePermission(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid role ID")
		return
	}

	ctx := r.Context()
	if err := a.rbac.DeletePermission(ctx, uint(id)); err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to delete permission")
		rb.InternalServerError("Failed to delete permission")
		return
	}

	rb.Deleted("Permission deleted successfully")
}

// ============================================================================
// User Permission Handlers
// ============================================================================

// GetUserPermissions gets permissions for a user
func (a *API) GetUserPermissions(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid permission ID")
		return
	}

	ctx := r.Context()
	allowed, denied, err := a.rbac.GetUserPermissions(ctx, uint(id))
	if err != nil {
		if err == ErrUserNotFound {
			rb.NotFound("User not found")
			return
		}
		a.rbac.logger.Error().Err(err).Msg("Failed to get user permissions")
		rb.InternalServerError("Failed to get user permissions")
		return
	}

	_ = denied // May be used in future for displaying denied permissions

	role, _ := a.rbac.GetUserRole(ctx, uint(id))
	var roleName *string
	if role != nil {
		name := role.Name
		roleName = &name
	}

	var roleID *uint
	if role != nil && role.ID != 0 {
		roleID = &role.ID
	}

	rb.OK(UserPermissionResponse{
		UserID:      uint(id),
		RoleID:      roleID,
		RoleName:    roleName,
		Permissions: allowed,
	})
}

// AssignUserRole assigns a role to a user
func (a *API) AssignUserRole(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid user ID")
		return
	}

	var req AssignRoleRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.HandleError(err)
		return
	}

	ctx := r.Context()
	if err := a.rbac.AssignRoleToUser(ctx, uint(id), req.RoleID); err != nil {
		a.rbac.logger.Error().Err(err).Msg("Failed to assign role")
		rb.InternalServerError("Failed to assign role")
		return
	}

	rb.OK("Role assigned successfully")
}

// CheckPermission checks if a user has a specific permission
func (a *API) CheckPermission(w http.ResponseWriter, r *http.Request) {
	rb := xcore.NewResponseBuilder(w, r)
	id := rb.GetQueryInt("id", 0)
	if id == 0 {
		rb.BadRequest("Invalid user ID")
		return
	}

	var req CheckPermissionRequest
	if err := rb.ParseJSON(&req); err != nil {
		rb.BadRequest("Invalid request body")
		return
	}

	ctx := r.Context()
	hasPermission, err := a.rbac.HasPermission(ctx, uint(id), req.Permission)
	if err != nil {
		a.rbac.logger.Error().Err(err).Msg("Permission check failed")
		rb.InternalServerError("Failed to check permission")
		return
	}

	response := CheckPermissionResponse{
		Allowed:    hasPermission,
		Permission: req.Permission,
	}

	if hasPermission {
		response.Reason = "User has permission"
	} else {
		response.Reason = "Permission denied"
	}

	rb.OK(response)
}

// ============================================================================
// Helper Functions
// ============================================================================

func (a *API) roleToResponse(role *Role) RoleResponse {
	permNames := make([]string, len(role.Permissions))
	denyIDs := make([]uint, 0)

	// Get permission names
	var rolePermissions []RolePermission
	a.rbac.db.Where("role_id = ?", role.ID).Find(&rolePermissions)

	for i, rp := range rolePermissions {
		var perm Permission
		if err := a.rbac.db.First(&perm, rp.PermissionID).Error; err == nil {
			permNames[i] = perm.Name
			if rp.IsDeny {
				denyIDs = append(denyIDs, rp.PermissionID)
			}
		}
	}

	return RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Slug:        role.Slug,
		Description: role.Description,
		IsDefault:   role.IsDefault,
		IsSystem:    role.IsSystem,
		Permissions: permNames,
		DenyIDs:     denyIDs,
		CreatedAt:   role.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   role.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (a *API) permissionToResponse(perm *Permission) PermissionResponse {
	return PermissionResponse{
		ID:          perm.ID,
		Name:        perm.Name,
		Namespace:   perm.Namespace,
		Resource:    perm.Resource,
		Action:      perm.Action,
		Levels:      perm.Levels,
		Description: perm.Description,
		CreatedAt:   perm.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   perm.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
