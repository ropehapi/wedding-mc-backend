package domain

import (
	"context"
	"time"
)

type PasswordResetToken struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	TokenHash string     `db:"token_hash"`
	ExpiresAt time.Time  `db:"expires_at"`
	UsedAt    *time.Time `db:"used_at"`
	CreatedAt time.Time  `db:"created_at"`
}

type PasswordResetTokenRepository interface {
	Create(ctx context.Context, t *PasswordResetToken) error
	FindByHash(ctx context.Context, hash string) (*PasswordResetToken, error)
	MarkUsed(ctx context.Context, id string) error
}
