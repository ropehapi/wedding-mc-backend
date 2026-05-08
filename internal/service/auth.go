package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ropehapi/wedding-mc/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// AuthService defines the authentication business logic contract.
type AuthService interface {
	Register(ctx context.Context, name, email, password, brideName, groomName string) (*domain.User, error)
	Login(ctx context.Context, email, password string) (*LoginResult, error)
	RefreshToken(ctx context.Context, refreshToken string) (*RefreshResult, error)
	Logout(ctx context.Context, userID string) error
}

// LoginResult holds the tokens returned after a successful login.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// RefreshResult holds the new access token after a token refresh.
type RefreshResult struct {
	AccessToken string
	ExpiresAt   time.Time
}

type authService struct {
	users         domain.UserRepository
	tokens        domain.RefreshTokenRepository
	weddings      WeddingService
	jwtSecret     string
	jwtExpiry     time.Duration
	refreshExpiry time.Duration
}

// NewAuthService creates a new AuthService with the given dependencies.
func NewAuthService(
	users domain.UserRepository,
	tokens domain.RefreshTokenRepository,
	weddings WeddingService,
	jwtSecret string,
	jwtExpiry time.Duration,
	refreshExpiry time.Duration,
) AuthService {
	return &authService{
		users:         users,
		tokens:        tokens,
		weddings:      weddings,
		jwtSecret:     jwtSecret,
		jwtExpiry:     jwtExpiry,
		refreshExpiry: refreshExpiry,
	}
}

func (s *authService) Register(ctx context.Context, name, email, password, brideName, groomName string) (*domain.User, error) {
	if len(password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", domain.ErrValidation)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: string(hash),
	}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}

	defaultDate := time.Now().AddDate(1, 0, 0)
	defaultLocation := "A definir"
	_, err = s.weddings.CreateWedding(ctx, u.ID, CreateWeddingRequest{
		BrideName: brideName,
		GroomName: groomName,
		Date:      defaultDate,
		Location:  defaultLocation,
	})
	if err != nil {
		return nil, fmt.Errorf("create wedding: %w", err)
	}

	return u, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (*LoginResult, error) {
	u, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrUnauthorized
	}

	accessToken, expiresAt, err := s.generateAccessToken(u.ID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	rawRefresh, hashRefresh, err := generateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	rt := &domain.RefreshToken{
		UserID:    u.ID,
		TokenHash: hashRefresh,
		ExpiresAt: time.Now().Add(s.refreshExpiry),
	}
	if err := s.tokens.Create(ctx, rt); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (*RefreshResult, error) {
	hash := hashToken(refreshToken)

	rt, err := s.tokens.FindByHash(ctx, hash)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	if rt.Revoked || time.Now().After(rt.ExpiresAt) {
		return nil, domain.ErrUnauthorized
	}

	accessToken, expiresAt, err := s.generateAccessToken(rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	return &RefreshResult{
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *authService) Logout(ctx context.Context, userID string) error {
	return s.tokens.RevokeByUserID(ctx, userID)
}

func (s *authService) generateAccessToken(userID string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.jwtExpiry)
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// generateRefreshToken creates a cryptographically random token.
// Returns the raw value (sent to the client) and its SHA-256 hash (stored in DB).
func generateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash = hashToken(raw)
	return raw, hash, nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
