package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/ropehapi/wedding-mc/internal/domain"
	"github.com/ropehapi/wedding-mc/internal/middleware"
	"github.com/ropehapi/wedding-mc/internal/service"
)

// authServicer is the subset of service.AuthService used by AuthHandler.
type authServicer interface {
	Register(ctx context.Context, name, email, password, brideName, groomName string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*service.LoginResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*service.RefreshResult, error)
	Logout(ctx context.Context, userID string) error
	ForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error
}

// AuthHandler handles HTTP requests for authentication endpoints.
type AuthHandler struct {
	svc      authServicer
	validate *validator.Validate
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc authServicer) *AuthHandler {
	return &AuthHandler{svc: svc, validate: validator.New()}
}

// --- Request / Response types ---

type registerRequest struct {
	Name      string `json:"name"       validate:"required"`
	Email     string `json:"email"      validate:"required,email"`
	Password  string `json:"password"   validate:"required,min=8"`
	BrideName string `json:"bride_name" validate:"required"`
	GroomName string `json:"groom_name" validate:"required"`
}

type userResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type loginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type refreshResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   string `json:"expires_at"`
}

type forgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"        validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password"     validate:"required,min=8"`
}

// --- Handlers ---

// Register godoc
// @Summary Registrar casal
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Dados do casal"
// @Success 201 {object} userResponse
// @Failure 409 {object} errorEnvelope
// @Failure 422 {object} validationEnvelope
// @Router /v1/auth/register [post]
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	u, err := h.svc.Register(r.Context(), req.Name, req.Email, req.Password, req.BrideName, req.GroomName)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusCreated, userResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Login godoc
// @Summary Login do casal
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Credenciais"
// @Success 200 {object} loginResponse
// @Failure 401 {object} errorEnvelope
// @Router /v1/auth/login [post]
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	result, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, loginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Refresh godoc
// @Summary Renovar access token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body refreshRequest true "Refresh token"
// @Success 200 {object} refreshResponse
// @Failure 401 {object} errorEnvelope
// @Router /v1/auth/refresh [post]
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	result, err := h.svc.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, refreshResponse{
		AccessToken: result.AccessToken,
		ExpiresAt:   result.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// Logout godoc
// @Summary Logout
// @Tags auth
// @Security BearerAuth
// @Success 204
// @Failure 401 {object} errorEnvelope
// @Router /v1/auth/logout [post]
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.svc.Logout(r.Context(), userID); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

// ForgotPassword godoc
// @Summary Solicitar recuperação de senha
// @Tags auth
// @Accept json
// @Produce json
// @Param body body forgotPasswordRequest true "E-mail do usuário"
// @Success 204
// @Failure 422 {object} validationEnvelope
// @Router /v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	if err := h.svc.ForgotPassword(r.Context(), req.Email); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

// ResetPassword godoc
// @Summary Redefinir senha com token de recuperação
// @Tags auth
// @Accept json
// @Produce json
// @Param body body resetPasswordRequest true "Token e nova senha"
// @Success 204
// @Failure 401 {object} errorEnvelope
// @Failure 422 {object} validationEnvelope
// @Router /v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	if err := h.svc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

// ChangePassword godoc
// @Summary Alterar senha (autenticado)
// @Tags auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body changePasswordRequest true "Senha atual e nova senha"
// @Success 204
// @Failure 401 {object} errorEnvelope
// @Failure 422 {object} validationEnvelope
// @Router /v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if errs := h.validate.Struct(req); errs != nil {
		var ve validator.ValidationErrors
		if errors.As(errs, &ve) {
			ValidationError(w, ve)
			return
		}
	}

	if err := h.svc.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

func (h *AuthHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrConflict):
		Error(w, http.StatusConflict, "conflict", "email already registered")
	case errors.Is(err, domain.ErrUnauthorized):
		Error(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
	case errors.Is(err, domain.ErrValidation):
		Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	default:
		log.Error().Err(err).Str("request_id", middleware.RequestIDFromContext(r.Context())).Msg("unhandled auth error")
		Error(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
