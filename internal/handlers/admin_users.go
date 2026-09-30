package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/abuamar142/auth-service/internal/middleware"
	"github.com/abuamar142/auth-service/internal/response"
	"github.com/abuamar142/auth-service/internal/services"
)

// AdminUserHandler manages accounts on behalf of an admin.
//
// Separate from UserHandler (the API-key, service-to-service lookup) on
// purpose: that one is reachable by any service holding the shared key, and
// this one changes data. Keeping them in different types makes it obvious at
// the route table which is which.
type AdminUserHandler struct {
	AuthService *services.AuthService
}

func NewAdminUserHandler(svc *services.AuthService) *AdminUserHandler {
	return &AdminUserHandler{AuthService: svc}
}

// writeServiceError maps a service error to an HTTP response.
//
// One place, so every endpoint answers the same way for the same condition —
// a delete that fails because it is the last admin should not say 500 on one
// route and 409 on another.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrUserNotFound):
		response.Error(w, http.StatusNotFound, "NOT_FOUND", "user not found", "")
	case errors.Is(err, services.ErrEmailExists), errors.Is(err, services.ErrUsernameExists):
		response.Error(w, http.StatusConflict, "IDENTIFIER_TAKEN", "email or username already taken", "")
	case errors.Is(err, services.ErrIdentifierRequired):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "email or username is required", "")
	case errors.Is(err, services.ErrPasswordTooShort):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "password too short", "must be at least 8 characters")
	case errors.Is(err, services.ErrCannotDeleteSelf):
		response.Error(w, http.StatusConflict, "CANNOT_DELETE_SELF", "you cannot delete your own account", "")
	case errors.Is(err, services.ErrCannotDemoteSelf):
		response.Error(w, http.StatusConflict, "CANNOT_DEMOTE_SELF", "you cannot remove your own admin rights", "")
	case errors.Is(err, services.ErrLastAdmin):
		response.Error(w, http.StatusConflict, "LAST_ADMIN", "at least one admin must remain", "")
	case errors.Is(err, services.ErrNothingToUpdate):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "no fields to update", "")
	case errors.Is(err, services.ErrUserIDRequired):
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "user id is required", "")
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "request failed", "")
	}
}

// List godoc
// @Summary      List all users
// @Description  Returns every account. Admin only.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.SwaggerResponse
// @Failure      401 {object} models.SwaggerResponse
// @Failure      403 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users [get]
func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.AuthService.ListUsers(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "users retrieved", map[string]any{
		"items": users,
		"total": len(users),
	})
}

// Get godoc
// @Summary      Get one user
// @Description  Returns a single account by id. Admin only.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User id"
// @Success      200 {object} models.SwaggerUserResponse
// @Failure      404 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users/{id} [get]
func (h *AdminUserHandler) Get(w http.ResponseWriter, r *http.Request) {
	user, err := h.AuthService.GetUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "user retrieved", user)
}

// Create godoc
// @Summary      Create a user
// @Description  Creates an account. Optionally grants admin rights. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body models.CreateUserRequest true "New account"
// @Success      201 {object} models.SwaggerUserResponse
// @Failure      400 {object} models.SwaggerResponse
// @Failure      409 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users [post]
func (h *AdminUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       string `json:"email"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if req.Password == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "password is required", "")
		return
	}
	if len(req.Password) < 8 {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "password too short", "must be at least 8 characters")
		return
	}

	user, err := h.AuthService.Register(r.Context(), req.Email, req.Username, req.Password, req.DisplayName)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Register always creates a non-admin. Granting the flag is a second,
	// explicit step so that the one place that can hand out admin rights stays
	// visible — it cannot happen as a side effect of a payload field.
	if req.IsAdmin {
		admin := true
		updated, err := h.AuthService.UpdateUser(r.Context(), "", user.ID, services.UpdateUserInput{IsAdmin: &admin})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		user = updated
	}

	response.JSON(w, http.StatusCreated, "user created", user)
}

// Update godoc
// @Summary      Update a user
// @Description  Partial update. Omitted fields are left unchanged. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User id"
// @Param        body body models.UpdateUserRequest true "Fields to change"
// @Success      200 {object} models.SwaggerUserResponse
// @Failure      400 {object} models.SwaggerResponse
// @Failure      404 {object} models.SwaggerResponse
// @Failure      409 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users/{id} [patch]
func (h *AdminUserHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email       *string `json:"email"`
		Username    *string `json:"username"`
		DisplayName *string `json:"display_name"`
		IsAdmin     *bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}

	actor := middleware.GetUser(r.Context())
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}

	user, err := h.AuthService.UpdateUser(r.Context(), actorID, chi.URLParam(r, "id"), services.UpdateUserInput{
		Email:       req.Email,
		Username:    req.Username,
		DisplayName: req.DisplayName,
		IsAdmin:     req.IsAdmin,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "user updated", user)
}

// Delete godoc
// @Summary      Delete a user
// @Description  Removes an account and its sessions. Admin only.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User id"
// @Success      200 {object} models.SwaggerResponse
// @Failure      404 {object} models.SwaggerResponse
// @Failure      409 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users/{id} [delete]
func (h *AdminUserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor := middleware.GetUser(r.Context())
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}

	if err := h.AuthService.DeleteUser(r.Context(), actorID, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "user deleted", nil)
}

// SetPassword godoc
// @Summary      Set a user's password
// @Description  Replaces the password and revokes the account's refresh tokens so its session cannot be extended. Admin only.
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "User id"
// @Param        body body models.SetPasswordRequest true "New password"
// @Success      200 {object} models.SwaggerResponse
// @Failure      400 {object} models.SwaggerResponse
// @Failure      404 {object} models.SwaggerResponse
// @Router       /api/v1/admin/users/{id}/password [post]
func (h *AdminUserHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body", "")
		return
	}
	if strings.TrimSpace(req.Password) == "" {
		response.Error(w, http.StatusBadRequest, "VALIDATION_ERROR", "password is required", "")
		return
	}

	if err := h.AuthService.SetPassword(r.Context(), chi.URLParam(r, "id"), req.Password); err != nil {
		writeServiceError(w, err)
		return
	}
	response.JSON(w, http.StatusOK, "password updated", nil)
}
