package domain

import (
	"context"
	"time"
)

type Table struct {
	ID        string    `db:"id"         json:"id"`
	WeddingID string    `db:"wedding_id" json:"wedding_id"`
	Name      string    `db:"name"       json:"name"`
	Capacity  int       `db:"capacity"   json:"capacity"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	Guests    []Guest   `db:"-"          json:"guests"`
}

type TableRepository interface {
	Create(ctx context.Context, t *Table) error
	FindAll(ctx context.Context, weddingID string) ([]Table, error)
	FindByID(ctx context.Context, id string) (*Table, error)
	Update(ctx context.Context, t *Table) error
	Delete(ctx context.Context, id string) error
	CountGuests(ctx context.Context, tableID string) (int, error)
}
