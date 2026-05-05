package service

import (
	"context"
	"fmt"

	"github.com/ropehapi/wedding-mc/internal/domain"
)

type TableService interface {
	CreateTable(ctx context.Context, userID, name string, capacity int) (*domain.Table, error)
	ListTables(ctx context.Context, userID string) ([]domain.Table, []domain.Guest, error)
	UpdateTable(ctx context.Context, userID, tableID string, name *string, capacity *int) (*domain.Table, error)
	DeleteTable(ctx context.Context, userID, tableID string) error
	AssignGuest(ctx context.Context, userID, tableID, guestID string) error
	UnassignGuest(ctx context.Context, userID, tableID, guestID string) error
}

type tableService struct {
	tables   domain.TableRepository
	guests   domain.GuestRepository
	weddings domain.WeddingRepository
}

func NewTableService(tables domain.TableRepository, guests domain.GuestRepository, weddings domain.WeddingRepository) TableService {
	return &tableService{tables: tables, guests: guests, weddings: weddings}
}

func (s *tableService) CreateTable(ctx context.Context, userID, name string, capacity int) (*domain.Table, error) {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	t := &domain.Table{
		WeddingID: w.ID,
		Name:      name,
		Capacity:  capacity,
	}
	if err := s.tables.Create(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *tableService) ListTables(ctx context.Context, userID string) ([]domain.Table, []domain.Guest, error) {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	tables, err := s.tables.FindAll(ctx, w.ID)
	if err != nil {
		return nil, nil, err
	}

	allGuests, err := s.guests.FindAll(ctx, w.ID, nil)
	if err != nil {
		return nil, nil, err
	}

	tableIndex := make(map[string]int, len(tables))
	for i := range tables {
		tables[i].Guests = []domain.Guest{}
		tableIndex[tables[i].ID] = i
	}

	var unassigned []domain.Guest
	for _, g := range allGuests {
		if g.TableID == nil {
			unassigned = append(unassigned, g)
			continue
		}
		if idx, ok := tableIndex[*g.TableID]; ok {
			tables[idx].Guests = append(tables[idx].Guests, g)
		}
	}
	if unassigned == nil {
		unassigned = []domain.Guest{}
	}

	return tables, unassigned, nil
}

func (s *tableService) UpdateTable(ctx context.Context, userID, tableID string, name *string, capacity *int) (*domain.Table, error) {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	t, err := s.tables.FindByID(ctx, tableID)
	if err != nil {
		return nil, err
	}
	if t.WeddingID != w.ID {
		return nil, domain.ErrNotFound
	}

	if capacity != nil {
		count, err := s.tables.CountGuests(ctx, tableID)
		if err != nil {
			return nil, err
		}
		if *capacity < count {
			return nil, fmt.Errorf("%w: capacity below current occupancy", domain.ErrValidation)
		}
		t.Capacity = *capacity
	}
	if name != nil {
		t.Name = *name
	}

	if err := s.tables.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *tableService) DeleteTable(ctx context.Context, userID, tableID string) error {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}

	t, err := s.tables.FindByID(ctx, tableID)
	if err != nil {
		return err
	}
	if t.WeddingID != w.ID {
		return domain.ErrNotFound
	}

	return s.tables.Delete(ctx, tableID)
}

func (s *tableService) AssignGuest(ctx context.Context, userID, tableID, guestID string) error {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}

	t, err := s.tables.FindByID(ctx, tableID)
	if err != nil {
		return err
	}
	if t.WeddingID != w.ID {
		return domain.ErrNotFound
	}

	g, err := s.guests.FindByID(ctx, guestID)
	if err != nil {
		return err
	}
	if g.WeddingID != w.ID {
		return domain.ErrNotFound
	}

	if g.TableID != nil && *g.TableID == tableID {
		return nil
	}

	count, err := s.tables.CountGuests(ctx, tableID)
	if err != nil {
		return err
	}
	if count >= t.Capacity {
		return fmt.Errorf("%w: capacity exceeded", domain.ErrValidation)
	}

	return s.guests.UpdateTableID(ctx, guestID, &tableID)
}

func (s *tableService) UnassignGuest(ctx context.Context, userID, tableID, guestID string) error {
	w, err := s.weddings.FindByUserID(ctx, userID)
	if err != nil {
		return err
	}

	t, err := s.tables.FindByID(ctx, tableID)
	if err != nil {
		return err
	}
	if t.WeddingID != w.ID {
		return domain.ErrNotFound
	}

	g, err := s.guests.FindByID(ctx, guestID)
	if err != nil {
		return err
	}
	if g.WeddingID != w.ID {
		return domain.ErrNotFound
	}

	if g.TableID == nil || *g.TableID != tableID {
		return domain.ErrNotAssigned
	}

	return s.guests.UpdateTableID(ctx, guestID, nil)
}
