package domain

import (
	"context"
	"time"
)

type RSVPStatus string

const (
	RSVPPending   RSVPStatus = "pending"
	RSVPConfirmed RSVPStatus = "confirmed"
	RSVPDeclined  RSVPStatus = "declined"
)

type Guest struct {
	ID         string     `db:"id" json:"id"`
	WeddingID  string     `db:"wedding_id" json:"wedding_id"`
	Name       string     `db:"name" json:"name"`
	Status     RSVPStatus `db:"status" json:"status"`
	AccessCode string     `db:"access_code" json:"access_code,omitempty"`
	TableID    *string    `db:"table_id" json:"table_id,omitempty"`
	RSVPAt     *time.Time `db:"rsvp_at" json:"rsvp_at,omitempty"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at" json:"updated_at"`
}

type GuestRepository interface {
	Create(ctx context.Context, g *Guest) error
	FindAll(ctx context.Context, weddingID string, status *RSVPStatus) ([]Guest, error)
	FindByID(ctx context.Context, id string) (*Guest, error)
	FindByAccessCode(ctx context.Context, weddingID, accessCode string) (*Guest, error)
	Update(ctx context.Context, g *Guest) error
	UpdateTableID(ctx context.Context, guestID string, tableID *string) error
	Delete(ctx context.Context, id string) error
	CountByStatus(ctx context.Context, weddingID string) (map[RSVPStatus]int, error)
}
