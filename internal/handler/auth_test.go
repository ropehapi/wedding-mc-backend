package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ropehapi/wedding-mc/internal/domain"
	"github.com/ropehapi/wedding-mc/internal/middleware"
	"github.com/ropehapi/wedding-mc/internal/service"
)

// mockAuthService implements authServicer for handler tests.
type mockAuthService struct {
	registerUser *domain.User
	registerErr  error

	loginResult *service.LoginResult
	loginErr    error

	refreshResult *service.RefreshResult
	refreshErr    error

	logoutErr error
}

func (m *mockAuthService) Register(_ context.Context, _, _, _, _, _ string) (*domain.User, error) {
	return m.registerUser, m.registerErr
}
func (m *mockAuthService) Login(_ context.Context, _, _ string) (*service.LoginResult, error) {
	return m.loginResult, m.loginErr
}
func (m *mockAuthService) RefreshToken(_ context.Context, _ string) (*service.RefreshResult, error) {
	return m.refreshResult, m.refreshErr
}
func (m *mockAuthService) Logout(_ context.Context, _ string) error {
	return m.logoutErr
}

// helpers

func newAuthHandler(svc *mockAuthService) *AuthHandler {
	return NewAuthHandler(svc)
}

func postJSON(t *testing.T, h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

// ---- Register ----

func TestRegister_201(t *testing.T) {
	now := time.Now()
	svc := &mockAuthService{registerUser: &domain.User{
		ID: "uuid-1", Name: "Ana e João", Email: "ana@email.com", CreatedAt: now,
	}}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Register, `{"name":"Ana e João","email":"ana@email.com","password":"senha123","bride_name":"Ana","groom_name":"João"}`)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rec.Code)
	}

	var body struct {
		Data struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	decodeBody(t, rec, &body)
	if body.Data.ID != "uuid-1" {
		t.Errorf("id: got %q", body.Data.ID)
	}

	// password_hash must never appear in the response
	if strings.Contains(rec.Body.String(), "password") {
		t.Error("password field leaked in response")
	}
}

func TestRegister_409_DuplicateEmail(t *testing.T) {
	svc := &mockAuthService{registerErr: domain.ErrConflict}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Register, `{"name":"Ana","email":"ana@email.com","password":"senha123","bride_name":"Ana","groom_name":"João"}`)

	if rec.Code != http.StatusConflict {
		t.Errorf("status: got %d, want 409", rec.Code)
	}
	var body map[string]string
	decodeBody(t, rec, &body)
	if body["error"] != "conflict" {
		t.Errorf("error: got %q", body["error"])
	}
}

func TestRegister_422_InvalidEmail(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})
	rec := postJSON(t, h.Register, `{"name":"Ana","email":"not-an-email","password":"senha123"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want 422", rec.Code)
	}
	var body struct {
		Error   string `json:"error"`
		Details []any  `json:"details"`
	}
	decodeBody(t, rec, &body)
	if body.Error != "validation_error" {
		t.Errorf("error: got %q", body.Error)
	}
	if len(body.Details) == 0 {
		t.Error("expected validation details")
	}
}

func TestRegister_422_ShortPassword(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})
	rec := postJSON(t, h.Register, `{"name":"Ana","email":"ana@email.com","password":"short"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status: got %d, want 422", rec.Code)
	}
}

func TestRegister_400_MalformedJSON(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})
	rec := postJSON(t, h.Register, `{broken`)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// ---- Login ----

func TestLogin_200(t *testing.T) {
	svc := &mockAuthService{loginResult: &service.LoginResult{
		AccessToken:  "access.tok",
		RefreshToken: "refresh.tok",
		ExpiresAt:    time.Now().Add(time.Hour),
	}}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Login, `{"email":"ana@email.com","password":"senha123"}`)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var body struct {
		Data struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresAt    string `json:"expires_at"`
		} `json:"data"`
	}
	decodeBody(t, rec, &body)
	if body.Data.AccessToken != "access.tok" {
		t.Errorf("access_token: got %q", body.Data.AccessToken)
	}
	if body.Data.RefreshToken != "refresh.tok" {
		t.Errorf("refresh_token: got %q", body.Data.RefreshToken)
	}
	if body.Data.ExpiresAt == "" {
		t.Error("expires_at is empty")
	}
}

func TestLogin_401_WrongCredentials(t *testing.T) {
	svc := &mockAuthService{loginErr: domain.ErrUnauthorized}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Login, `{"email":"ana@email.com","password":"wrong"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

func TestLogin_400_MalformedJSON(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})
	rec := postJSON(t, h.Login, `{broken`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
}

// ---- Refresh ----

func TestRefresh_200(t *testing.T) {
	svc := &mockAuthService{refreshResult: &service.RefreshResult{
		AccessToken: "new.access.tok",
		ExpiresAt:   time.Now().Add(time.Hour),
	}}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Refresh, `{"refresh_token":"some-refresh-tok"}`)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var body struct {
		Data struct {
			AccessToken string `json:"access_token"`
			ExpiresAt   string `json:"expires_at"`
		} `json:"data"`
	}
	decodeBody(t, rec, &body)
	if body.Data.AccessToken != "new.access.tok" {
		t.Errorf("access_token: got %q", body.Data.AccessToken)
	}
}

func TestRefresh_401_InvalidToken(t *testing.T) {
	svc := &mockAuthService{refreshErr: domain.ErrUnauthorized}
	h := newAuthHandler(svc)

	rec := postJSON(t, h.Refresh, `{"refresh_token":"bad-tok"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// ---- Logout ----

func TestLogout_204(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})

	// Inject userID into context as middleware.Auth would do
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req = req.WithContext(middleware.WithUserID(req.Context(), "user-1"))
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", rec.Code)
	}
}

func TestLogout_401_NoAuth(t *testing.T) {
	h := newAuthHandler(&mockAuthService{})
	rec := postJSON(t, h.Logout, ``)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

