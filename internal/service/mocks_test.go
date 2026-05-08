package service

import (
	"context"
	"io"

	"github.com/ropehapi/wedding-mc/internal/domain"
)

// mockWeddingService is a test double for WeddingService.
type mockWeddingService struct {
	createErr error
	wedding   *domain.Wedding
}

func (m *mockWeddingService) CreateWedding(_ context.Context, _ string, _ CreateWeddingRequest) (*domain.Wedding, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.wedding != nil {
		return m.wedding, nil
	}
	return &domain.Wedding{ID: "wedding-id-123"}, nil
}

func (m *mockWeddingService) GetWedding(_ context.Context, _ string) (*domain.Wedding, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWeddingService) GetWeddingBySlug(_ context.Context, _ string) (*domain.Wedding, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWeddingService) UpdateWedding(_ context.Context, _ string, _ UpdateWeddingRequest) (*domain.Wedding, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWeddingService) UploadPhoto(_ context.Context, _, _ string, _ io.Reader, _ int64) (*domain.WeddingPhoto, error) {
	return nil, domain.ErrNotFound
}
func (m *mockWeddingService) DeletePhoto(_ context.Context, _, _ string) error { return nil }
func (m *mockWeddingService) SetCoverPhoto(_ context.Context, _, _ string) error { return nil }

// mockUserRepo is a test double for domain.UserRepository.
type mockUserRepo struct {
	createErr    error
	findByEmail  *domain.User
	findEmailErr error
	findByID     *domain.User
	findIDErr    error

	created    *domain.User
}

func (m *mockUserRepo) Create(ctx context.Context, u *domain.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	u.ID = "user-id-123"
	m.created = u
	return nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return m.findByEmail, m.findEmailErr
}

func (m *mockUserRepo) FindByID(_ context.Context, _ string) (*domain.User, error) {
	return m.findByID, m.findIDErr
}

// mockTokenRepo is a test double for domain.RefreshTokenRepository.
type mockTokenRepo struct {
	createErr      error
	findByHash     *domain.RefreshToken
	findByHashErr  error
	revokedUserID  string
	revokeErr      error
}

func (m *mockTokenRepo) Create(_ context.Context, rt *domain.RefreshToken) error {
	if m.createErr != nil {
		return m.createErr
	}
	rt.ID = "token-id-456"
	return nil
}

func (m *mockTokenRepo) FindByHash(_ context.Context, _ string) (*domain.RefreshToken, error) {
	return m.findByHash, m.findByHashErr
}

func (m *mockTokenRepo) RevokeByUserID(_ context.Context, userID string) error {
	m.revokedUserID = userID
	return m.revokeErr
}
