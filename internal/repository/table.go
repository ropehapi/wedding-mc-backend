package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/ropehapi/wedding-mc/internal/domain"
)

type tableRepo struct {
	db *sqlx.DB
}

func NewTableRepository(db *sqlx.DB) domain.TableRepository {
	return &tableRepo{db: db}
}

func (r *tableRepo) Create(ctx context.Context, t *domain.Table) error {
	query := `
		INSERT INTO tables (wedding_id, name, capacity)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query, t.WeddingID, t.Name, t.Capacity).
		Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *tableRepo) FindAll(ctx context.Context, weddingID string) ([]domain.Table, error) {
	tables := []domain.Table{}
	err := r.db.SelectContext(ctx, &tables,
		`SELECT * FROM tables WHERE wedding_id = $1 ORDER BY name`,
		weddingID,
	)
	return tables, err
}

func (r *tableRepo) FindByID(ctx context.Context, id string) (*domain.Table, error) {
	var t domain.Table
	err := r.db.GetContext(ctx, &t, `SELECT * FROM tables WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *tableRepo) Update(ctx context.Context, t *domain.Table) error {
	query := `
		UPDATE tables SET
			name       = $1,
			capacity   = $2,
			updated_at = NOW()
		WHERE id = $3
		RETURNING updated_at`
	err := r.db.QueryRowContext(ctx, query, t.Name, t.Capacity, t.ID).Scan(&t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

func (r *tableRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM tables WHERE id = $1`, id)
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

func (r *tableRepo) CountGuests(ctx context.Context, tableID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM guests WHERE table_id = $1`, tableID).Scan(&count)
	return count, err
}
