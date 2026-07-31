package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/auth"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type TokenChangementEmailRepository struct {
	pool *pgxpool.Pool
}

func NewTokenChangementEmailRepository(pool *pgxpool.Pool) *TokenChangementEmailRepository {
	return &TokenChangementEmailRepository{pool: pool}
}

func (r *TokenChangementEmailRepository) Create(ctx context.Context, t *auth.TokenChangementEmail) error {
	const query = `
		INSERT INTO tokens_changement_email (id, utilisateur_id, nouvel_email, token_hash, expire_at, utilise_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		t.ID, t.UtilisateurID, t.NouvelEmail, t.TokenHash, t.ExpireAt, t.UtiliseAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création token changement email: %w", err)
	}
	return nil
}

func (r *TokenChangementEmailRepository) FindByHash(ctx context.Context, tokenHash string) (*auth.TokenChangementEmail, error) {
	const query = `
		SELECT id, utilisateur_id, nouvel_email, token_hash, expire_at, utilise_at, created_at
		FROM tokens_changement_email WHERE token_hash = $1`

	var t auth.TokenChangementEmail
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UtilisateurID, &t.NouvelEmail, &t.TokenHash, &t.ExpireAt, &t.UtiliseAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrTokenInvalide
	}
	if err != nil {
		return nil, fmt.Errorf("lecture token changement email: %w", err)
	}
	return &t, nil
}

func (r *TokenChangementEmailRepository) MarquerUtilise(ctx context.Context, id string) error {
	const query = `UPDATE tokens_changement_email SET utilise_at = now() WHERE id = $1 AND utilise_at IS NULL`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("consommation token changement email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrTokenInvalide
	}
	return nil
}
