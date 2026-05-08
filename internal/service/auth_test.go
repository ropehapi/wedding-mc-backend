package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ropehapi/wedding-mc/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

const (
	testSecret        = "test-jwt-secret"
	testJWTExpiry     = time.Hour
	testRefreshExpiry = 168 * time.Hour
)

func newTestAuthService(users *mockUserRepo, tokens *mockTokenRepo) AuthService {
	return NewAuthService(users, tokens, &mockWeddingService{}, testSecret, testJWTExpiry, testRefreshExpiry)
}

func hashedPassword(t *testing.T, pw string) string {
	t.Helper()
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(b)
}

// ---- Register ----

func TestRegister_Success(t *testing.T) {
	users := &mockUserRepo{}
	svc := newTestAuthService(users, &mockTokenRepo{})

	u, err := svc.Register(context.Background(), "Ana e João", "ana@email.com", "senha123", "Ana", "João")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.ID == "" {
		t.Error("ID should be set after creation")
	}
	if u.PasswordHash != "" && u.PasswordHash == "senha123" {
		t.Error("password stored in plain text")
	}
	if users.created == nil {
		t.Error("Create was not called")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	users := &mockUserRepo{createErr: domain.ErrConflict}
	svc := newTestAuthService(users, &mockTokenRepo{})

	_, err := svc.Register(context.Background(), "Ana", "ana@email.com", "senha123", "Ana", "João")
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	users := &mockUserRepo{}
	svc := newTestAuthService(users, &mockTokenRepo{})

	_, err := svc.Register(context.Background(), "Ana", "ana@email.com", "short", "Ana", "João")
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
	if users.created != nil {
		t.Error("Create should not be called on validation failure")
	}
}

// ---- Login ----

func TestLogin_Success(t *testing.T) {
	pw := "senha123"
	user := &domain.User{ID: "user-1", Email: "ana@email.com", PasswordHash: hashedPassword(t, pw)}
	svc := newTestAuthService(&mockUserRepo{findByEmail: user}, &mockTokenRepo{})

	result, err := svc.Login(context.Background(), "ana@email.com", pw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if result.RefreshToken == "" {
		t.Error("RefreshToken is empty")
	}
	if result.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt is in the past")
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{findEmailErr: domain.ErrNotFound}, &mockTokenRepo{})

	_, err := svc.Login(context.Background(), "no@one.com", "senha123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	user := &domain.User{ID: "user-1", PasswordHash: hashedPassword(t, "correta123")}
	svc := newTestAuthService(&mockUserRepo{findByEmail: user}, &mockTokenRepo{})

	_, err := svc.Login(context.Background(), "ana@email.com", "errada123")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestLogin_WrongPassword_SameErrorAsNotFound(t *testing.T) {
	// Security: both "not found" and "wrong password" must produce ErrUnauthorized
	// so attackers cannot enumerate valid emails.
	user := &domain.User{ID: "user-1", PasswordHash: hashedPassword(t, "correta123")}
	svc := newTestAuthService(&mockUserRepo{findByEmail: user}, &mockTokenRepo{})

	_, errWrongPw := svc.Login(context.Background(), "ana@email.com", "errada123")

	svc2 := newTestAuthService(&mockUserRepo{findEmailErr: domain.ErrNotFound}, &mockTokenRepo{})
	_, errNotFound := svc2.Login(context.Background(), "no@one.com", "errada123")

	if !errors.Is(errWrongPw, domain.ErrUnauthorized) || !errors.Is(errNotFound, domain.ErrUnauthorized) {
		t.Errorf("both errors must be ErrUnauthorized: %v, %v", errWrongPw, errNotFound)
	}
}

func TestLogin_AccessTokenHasCorrectClaims(t *testing.T) {
	pw := "senha123"
	user := &domain.User{ID: "user-abc", Email: "ana@email.com", PasswordHash: hashedPassword(t, pw)}
	svc := newTestAuthService(&mockUserRepo{findByEmail: user}, &mockTokenRepo{})

	result, err := svc.Login(context.Background(), "ana@email.com", pw)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	claims := &jwt.RegisteredClaims{}
	_, err = jwt.ParseWithClaims(result.AccessToken, claims, func(tok *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-abc" {
		t.Errorf("sub: got %q, want user-abc", claims.Subject)
	}
	if claims.ExpiresAt == nil || claims.ExpiresAt.Before(time.Now()) {
		t.Error("token is already expired")
	}
}

// ---- RefreshToken ----

func TestRefreshToken_Valid(t *testing.T) {
	rt := &domain.RefreshToken{
		ID:        "rt-1",
		UserID:    "user-1",
		TokenHash: "", // will be matched by mock
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Revoked:   false,
	}
	svc := newTestAuthService(&mockUserRepo{}, &mockTokenRepo{findByHash: rt})

	result, err := svc.RefreshToken(context.Background(), "any-raw-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AccessToken == "" {
		t.Error("AccessToken is empty")
	}
	if result.ExpiresAt.Before(time.Now()) {
		t.Error("ExpiresAt is in the past")
	}
}

func TestRefreshToken_NotFound(t *testing.T) {
	svc := newTestAuthService(&mockUserRepo{}, &mockTokenRepo{findByHashErr: domain.ErrNotFound})

	_, err := svc.RefreshToken(context.Background(), "bad-token")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRefreshToken_Revoked(t *testing.T) {
	rt := &domain.RefreshToken{
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(time.Hour),
		Revoked:   true,
	}
	svc := newTestAuthService(&mockUserRepo{}, &mockTokenRepo{findByHash: rt})

	_, err := svc.RefreshToken(context.Background(), "any-raw-token")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRefreshToken_Expired(t *testing.T) {
	rt := &domain.RefreshToken{
		UserID:    "user-1",
		ExpiresAt: time.Now().Add(-time.Hour),
		Revoked:   false,
	}
	svc := newTestAuthService(&mockUserRepo{}, &mockTokenRepo{findByHash: rt})

	_, err := svc.RefreshToken(context.Background(), "any-raw-token")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

// ---- Logout ----

func TestLogout_RevokesAllTokens(t *testing.T) {
	tokens := &mockTokenRepo{}
	svc := newTestAuthService(&mockUserRepo{}, tokens)

	err := svc.Logout(context.Background(), "user-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens.revokedUserID != "user-99" {
		t.Errorf("RevokeByUserID not called with user-99, got %q", tokens.revokedUserID)
	}
}

// ---- generateRefreshToken ----

func TestGenerateRefreshToken_Uniqueness(t *testing.T) {
	raw1, hash1, err1 := generateRefreshToken()
	raw2, hash2, err2 := generateRefreshToken()
	if err1 != nil || err2 != nil {
		t.Fatalf("errors: %v %v", err1, err2)
	}
	if raw1 == raw2 {
		t.Error("two calls returned same raw token")
	}
	if hash1 == hash2 {
		t.Error("two calls returned same hash")
	}
	if strings.Contains(raw1, "+") || strings.Contains(raw1, "/") || strings.Contains(raw1, "=") {
		t.Errorf("raw token is not base64url-safe: %q", raw1)
	}
}
