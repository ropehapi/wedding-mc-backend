package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ropehapi/wedding-mc/internal/domain"
)

// mockGuestRepo is a test double for domain.GuestRepository.
type mockGuestRepo struct {
	createErr error
	created   *domain.Guest

	findAllResult []domain.Guest
	findAllErr    error

	findByIDResult *domain.Guest
	findByIDErr    error

	updateErr error
	updated   *domain.Guest

	deleteErr error

	countResult map[domain.RSVPStatus]int
	countErr    error

	updateTableIDErr error
}

func (m *mockGuestRepo) Create(_ context.Context, g *domain.Guest) error {
	if m.createErr != nil {
		return m.createErr
	}
	g.ID = "guest-id-1"
	m.created = g
	return nil
}

func (m *mockGuestRepo) FindAll(_ context.Context, _ string, _ *domain.RSVPStatus) ([]domain.Guest, error) {
	return m.findAllResult, m.findAllErr
}

func (m *mockGuestRepo) FindByID(_ context.Context, _ string) (*domain.Guest, error) {
	return m.findByIDResult, m.findByIDErr
}

func (m *mockGuestRepo) Update(_ context.Context, g *domain.Guest) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = g
	return nil
}

func (m *mockGuestRepo) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

func (m *mockGuestRepo) CountByStatus(_ context.Context, _ string) (map[domain.RSVPStatus]int, error) {
	return m.countResult, m.countErr
}

func (m *mockGuestRepo) FindByAccessCode(_ context.Context, _, _ string) (*domain.Guest, error) {
	return nil, domain.ErrNotFound
}

func (m *mockGuestRepo) UpdateTableID(_ context.Context, _ string, _ *string) error {
	return m.updateTableIDErr
}

func newTestGuestService(guests *mockGuestRepo, weddings *mockWeddingRepo) GuestService {
	return NewGuestService(guests, weddings)
}

func baseWedding() *domain.Wedding {
	return &domain.Wedding{ID: "wedding-id-1", Slug: "ana-e-joao"}
}

// ---- CreateGuest ----

func TestCreateGuest_Success(t *testing.T) {
	weddings := &mockWeddingRepo{findByUserID: baseWedding()}
	guests := &mockGuestRepo{}
	svc := newTestGuestService(guests, weddings)

	g, err := svc.CreateGuest(context.Background(), "user-1", "Ana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.WeddingID != "wedding-id-1" {
		t.Errorf("wrong wedding_id: %q", g.WeddingID)
	}
	if g.Status != domain.RSVPPending {
		t.Errorf("initial status should be pending, got %q", g.Status)
	}
}

func TestCreateGuest_WeddingNotFound(t *testing.T) {
	weddings := &mockWeddingRepo{findByUserIDErr: domain.ErrNotFound}
	svc := newTestGuestService(&mockGuestRepo{}, weddings)

	_, err := svc.CreateGuest(context.Background(), "user-1", "Ana")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- RSVP ----

func TestRSVP_Confirmed(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID, Status: domain.RSVPPending, AccessCode: "123456"}
	weddings := &mockWeddingRepo{findBySlugResults: map[string]*domain.Wedding{w.Slug: w}}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	updated, err := svc.RSVP(context.Background(), w.Slug, guest.ID, "123456", domain.RSVPConfirmed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != domain.RSVPConfirmed {
		t.Errorf("expected confirmed, got %q", updated.Status)
	}
	if updated.RSVPAt == nil {
		t.Error("rsvp_at should be set")
	}
}

func TestRSVP_Declined(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID, Status: domain.RSVPPending, AccessCode: "123456"}
	weddings := &mockWeddingRepo{findBySlugResults: map[string]*domain.Wedding{w.Slug: w}}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	updated, err := svc.RSVP(context.Background(), w.Slug, guest.ID, "123456", domain.RSVPDeclined)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Status != domain.RSVPDeclined {
		t.Errorf("expected declined, got %q", updated.Status)
	}
}

func TestRSVP_InvalidStatus(t *testing.T) {
	svc := newTestGuestService(&mockGuestRepo{}, &mockWeddingRepo{})

	_, err := svc.RSVP(context.Background(), "slug", "guest-1", "123456", domain.RSVPStatus("maybe"))
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation, got %v", err)
	}
}

func TestRSVP_PendingStatusRejected(t *testing.T) {
	svc := newTestGuestService(&mockGuestRepo{}, &mockWeddingRepo{})

	_, err := svc.RSVP(context.Background(), "slug", "guest-1", "123456", domain.RSVPPending)
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("expected ErrValidation for 'pending' status, got %v", err)
	}
}

func TestRSVP_GuestFromAnotherWedding(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: "other-wedding-id", AccessCode: "123456"}
	weddings := &mockWeddingRepo{findBySlugResults: map[string]*domain.Wedding{w.Slug: w}}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	_, err := svc.RSVP(context.Background(), w.Slug, guest.ID, "123456", domain.RSVPConfirmed)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRSVP_WrongAccessCode(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID, Status: domain.RSVPPending, AccessCode: "123456"}
	weddings := &mockWeddingRepo{findBySlugResults: map[string]*domain.Wedding{w.Slug: w}}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	_, err := svc.RSVP(context.Background(), w.Slug, guest.ID, "000000", domain.RSVPConfirmed)
	if !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestRSVP_SlugNotFound(t *testing.T) {
	weddings := &mockWeddingRepo{}
	svc := newTestGuestService(&mockGuestRepo{}, weddings)

	_, err := svc.RSVP(context.Background(), "unknown-slug", "g-1", "123456", domain.RSVPConfirmed)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- GetSummary ----

func TestGetSummary_CorrectCounts(t *testing.T) {
	w := baseWedding()
	counts := map[domain.RSVPStatus]int{
		domain.RSVPPending:   3,
		domain.RSVPConfirmed: 5,
		domain.RSVPDeclined:  2,
	}
	weddings := &mockWeddingRepo{findByUserID: w}
	guests := &mockGuestRepo{countResult: counts}
	svc := newTestGuestService(guests, weddings)

	result, err := svc.GetSummary(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result[domain.RSVPPending] != 3 {
		t.Errorf("pending: want 3, got %d", result[domain.RSVPPending])
	}
	if result[domain.RSVPConfirmed] != 5 {
		t.Errorf("confirmed: want 5, got %d", result[domain.RSVPConfirmed])
	}
	if result[domain.RSVPDeclined] != 2 {
		t.Errorf("declined: want 2, got %d", result[domain.RSVPDeclined])
	}
}

// ---- UpdateGuest ----

func TestUpdateGuest_Success(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID, Name: "Old Name"}
	weddings := &mockWeddingRepo{findByUserID: w}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	updated, err := svc.UpdateGuest(context.Background(), "user-1", "g-1", "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name 'New Name', got %q", updated.Name)
	}
}

func TestUpdateGuest_GuestFromAnotherWedding(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: "other-id"}
	weddings := &mockWeddingRepo{findByUserID: w}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	_, err := svc.UpdateGuest(context.Background(), "user-1", "g-1", "Name")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- DeleteGuest ----

func TestDeleteGuest_Success(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID}
	weddings := &mockWeddingRepo{findByUserID: w}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	err := svc.DeleteGuest(context.Background(), "user-1", "g-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteGuest_GuestFromAnotherWedding(t *testing.T) {
	w := baseWedding()
	guest := &domain.Guest{ID: "g-1", WeddingID: "other-id"}
	weddings := &mockWeddingRepo{findByUserID: w}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	err := svc.DeleteGuest(context.Background(), "user-1", "g-1")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- RSVPAt timestamp ----

func TestRSVP_SetsTimestamp(t *testing.T) {
	w := baseWedding()
	before := time.Now()
	guest := &domain.Guest{ID: "g-1", WeddingID: w.ID, AccessCode: "123456"}
	weddings := &mockWeddingRepo{findBySlugResults: map[string]*domain.Wedding{w.Slug: w}}
	guests := &mockGuestRepo{findByIDResult: guest}
	svc := newTestGuestService(guests, weddings)

	updated, err := svc.RSVP(context.Background(), w.Slug, "g-1", "123456", domain.RSVPConfirmed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.RSVPAt == nil || updated.RSVPAt.Before(before) {
		t.Error("rsvp_at should be set to a recent timestamp")
	}
}
