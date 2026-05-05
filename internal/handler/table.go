package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/ropehapi/wedding-mc/internal/domain"
	"github.com/ropehapi/wedding-mc/internal/middleware"
	"github.com/ropehapi/wedding-mc/internal/service"
)

type TableHandler struct {
	svc      service.TableService
	validate *validator.Validate
}

func NewTableHandler(svc service.TableService) *TableHandler {
	return &TableHandler{svc: svc, validate: validator.New()}
}

// --- Request / Response types ---

type createTableRequest struct {
	Name     string `json:"name"     validate:"required"`
	Capacity int    `json:"capacity" validate:"required,min=1"`
}

type updateTableRequest struct {
	Name     *string `json:"name"`
	Capacity *int    `json:"capacity" validate:"omitempty,min=1"`
}

type tableGuestResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type tableResponse struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	Capacity  int                  `json:"capacity"`
	Occupied  int                  `json:"occupied"`
	Guests    []tableGuestResponse `json:"guests"`
	CreatedAt string               `json:"created_at"`
	UpdatedAt string               `json:"updated_at"`
}

type listTablesResponse struct {
	Tables     []tableResponse      `json:"tables"`
	Unassigned []tableGuestResponse `json:"unassigned"`
}

// --- Handlers ---

// Create godoc
// @Summary Criar mesa
// @Tags tables
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body createTableRequest true "Dados da mesa"
// @Success 201 {object} tableResponse
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Failure 422 {object} validationEnvelope
// @Router /v1/tables [post]
func (h *TableHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req createTableRequest
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

	t, err := h.svc.CreateTable(r.Context(), userID, req.Name, req.Capacity)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusCreated, toTableResponse(t))
}

// List godoc
// @Summary Listar mesas
// @Tags tables
// @Security BearerAuth
// @Produce json
// @Success 200 {object} listTablesResponse
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Router /v1/tables [get]
func (h *TableHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tables, unassigned, err := h.svc.ListTables(r.Context(), userID)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	resp := listTablesResponse{
		Tables:     make([]tableResponse, len(tables)),
		Unassigned: make([]tableGuestResponse, len(unassigned)),
	}
	for i := range tables {
		resp.Tables[i] = toTableResponse(&tables[i])
	}
	for i := range unassigned {
		resp.Unassigned[i] = toTableGuestResponse(&unassigned[i])
	}

	JSON(w, http.StatusOK, resp)
}

// Update godoc
// @Summary Atualizar mesa
// @Tags tables
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param tableID path string true "ID da mesa"
// @Param body body updateTableRequest true "Campos a atualizar"
// @Success 200 {object} tableResponse
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Failure 422 {object} validationEnvelope
// @Router /v1/tables/{tableID} [patch]
func (h *TableHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tableID := chi.URLParam(r, "tableID")

	var req updateTableRequest
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

	t, err := h.svc.UpdateTable(r.Context(), userID, tableID, req.Name, req.Capacity)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, toTableResponse(t))
}

// Delete godoc
// @Summary Remover mesa
// @Tags tables
// @Security BearerAuth
// @Param tableID path string true "ID da mesa"
// @Success 204
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Router /v1/tables/{tableID} [delete]
func (h *TableHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tableID := chi.URLParam(r, "tableID")

	if err := h.svc.DeleteTable(r.Context(), userID, tableID); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

// AssignGuest godoc
// @Summary Alocar convidado na mesa
// @Tags tables
// @Security BearerAuth
// @Param tableID path string true "ID da mesa"
// @Param guestID path string true "ID do convidado"
// @Success 200
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Failure 409 {object} errorEnvelope
// @Router /v1/tables/{tableID}/guests/{guestID} [put]
func (h *TableHandler) AssignGuest(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tableID := chi.URLParam(r, "tableID")
	guestID := chi.URLParam(r, "guestID")

	if err := h.svc.AssignGuest(r.Context(), userID, tableID, guestID); err != nil {
		h.handleError(w, r, err)
		return
	}

	JSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// UnassignGuest godoc
// @Summary Remover convidado da mesa
// @Tags tables
// @Security BearerAuth
// @Param tableID path string true "ID da mesa"
// @Param guestID path string true "ID do convidado"
// @Success 204
// @Failure 401 {object} errorEnvelope
// @Failure 404 {object} errorEnvelope
// @Router /v1/tables/{tableID}/guests/{guestID} [delete]
func (h *TableHandler) UnassignGuest(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tableID := chi.URLParam(r, "tableID")
	guestID := chi.URLParam(r, "guestID")

	if err := h.svc.UnassignGuest(r.Context(), userID, tableID, guestID); err != nil {
		h.handleError(w, r, err)
		return
	}

	NoContent(w)
}

func (h *TableHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		Error(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrValidation):
		Error(w, http.StatusUnprocessableEntity, "validation_error", err.Error())
	case errors.Is(err, domain.ErrForbidden):
		Error(w, http.StatusForbidden, "forbidden", "access denied")
	case errors.Is(err, domain.ErrNotAssigned):
		Error(w, http.StatusConflict, "not_assigned", err.Error())
	default:
		log.Error().Err(err).Str("request_id", middleware.RequestIDFromContext(r.Context())).Msg("unhandled table error")
		Error(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func toTableResponse(t *domain.Table) tableResponse {
	guests := make([]tableGuestResponse, len(t.Guests))
	for i := range t.Guests {
		guests[i] = toTableGuestResponse(&t.Guests[i])
	}
	return tableResponse{
		ID:        t.ID,
		Name:      t.Name,
		Capacity:  t.Capacity,
		Occupied:  len(guests),
		Guests:    guests,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

func toTableGuestResponse(g *domain.Guest) tableGuestResponse {
	return tableGuestResponse{
		ID:     g.ID,
		Name:   g.Name,
		Status: string(g.Status),
	}
}

