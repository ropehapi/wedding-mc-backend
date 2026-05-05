package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/lib/pq"
	"github.com/jmoiron/sqlx"
	"github.com/ropehapi/wedding-mc/internal/domain"
)

func generateAccessCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1_000_000))
	return fmt.Sprintf("%06d", n.Int64())
}

type guestRepo struct {
	db *sqlx.DB
}

func NewGuestRepository(db *sqlx.DB) domain.GuestRepository {
	return &guestRepo{db: db}
}

func (r *guestRepo) Create(ctx context.Context, g *domain.Guest) error {
	query := `
		INSERT INTO guests (wedding_id, name, status, access_code)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	for range 10 {
		g.AccessCode = generateAccessCode()
		err := r.db.QueryRowContext(ctx, query, g.WeddingID, g.Name, g.Status, g.AccessCode).
			Scan(&g.ID, &g.CreatedAt, &g.UpdatedAt)
		if err == nil {
			return nil
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			continue
		}
		return err
	}
	return fmt.Errorf("failed to generate unique access code after retries")
}

func (r *guestRepo) FindAll(ctx context.Context, weddingID string, status *domain.RSVPStatus) ([]domain.Guest, error) {
	guests := []domain.Guest{}
	if status != nil {
		err := r.db.SelectContext(ctx, &guests,
			`SELECT * FROM guests WHERE wedding_id = $1 AND status = $2 ORDER BY created_at`,
			weddingID, *status,
		)
		return guests, err
	}
	err := r.db.SelectContext(ctx, &guests,
		`SELECT * FROM guests WHERE wedding_id = $1 ORDER BY created_at`,
		weddingID,
	)
	return guests, err
}

func (r *guestRepo) FindByID(ctx context.Context, id string) (*domain.Guest, error) {
	var g domain.Guest
	err := r.db.GetContext(ctx, &g, `SELECT * FROM guests WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *guestRepo) FindByAccessCode(ctx context.Context, weddingID, accessCode string) (*domain.Guest, error) {
	var g domain.Guest
	err := r.db.GetContext(ctx, &g,
		`SELECT * FROM guests WHERE wedding_id = $1 AND access_code = $2`,
		weddingID, accessCode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *guestRepo) UpdateTableID(ctx context.Context, guestID string, tableID *string) error {
	query := `UPDATE guests SET table_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, tableID, guestID)
	return err
}

func (r *guestRepo) Update(ctx context.Context, g *domain.Guest) error {
	query := `
		UPDATE guests SET
			name       = $1,
			status     = $2,
			rsvp_at    = $3,
			updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at`
	err := r.db.QueryRowContext(ctx, query, g.Name, g.Status, g.RSVPAt, g.ID).Scan(&g.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func (r *guestRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM guests WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *guestRepo) CountByStatus(ctx context.Context, weddingID string) (map[domain.RSVPStatus]int, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM guests WHERE wedding_id = $1 GROUP BY status`,
		weddingID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[domain.RSVPStatus]int{
		domain.RSVPPending:   0,
		domain.RSVPConfirmed: 0,
		domain.RSVPDeclined:  0,
	}
	for rows.Next() {
		var status domain.RSVPStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, rows.Err()
}
