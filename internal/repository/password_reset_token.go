package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/ropehapi/wedding-mc/internal/domain"
)

type passwordResetTokenRepo struct {
	db *sqlx.DB
}

func NewPasswordResetTokenRepository(db *sqlx.DB) domain.PasswordResetTokenRepository {
	return &passwordResetTokenRepo{db: db}
}

func (r *passwordResetTokenRepo) Create(ctx context.Context, t *domain.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`
	return r.db.QueryRowContext(ctx, query, t.UserID, t.TokenHash, t.ExpiresAt).
		Scan(&t.ID, &t.CreatedAt)
}

func (r *passwordResetTokenRepo) FindByHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	var t domain.PasswordResetToken
	err := r.db.GetContext(ctx, &t, `SELECT * FROM password_reset_tokens WHERE token_hash = $1`, hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *passwordResetTokenRepo) MarkUsed(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE password_reset_tokens SET used_at = NOW() WHERE id = $1`,
		id,
	)
	return err
}
