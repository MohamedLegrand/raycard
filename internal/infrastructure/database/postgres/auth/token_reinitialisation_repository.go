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

type TokenReinitialisationRepository struct {
	pool *pgxpool.Pool
}

func NewTokenReinitialisationRepository(pool *pgxpool.Pool) *TokenReinitialisationRepository {
	return &TokenReinitialisationRepository{pool: pool}
}

func (r *TokenReinitialisationRepository) Create(ctx context.Context, t *auth.TokenReinitialisation) error {
	const query = `
		INSERT INTO tokens_reinitialisation (id, utilisateur_id, token_hash, expire_at, utilise_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		t.ID, t.UtilisateurID, t.TokenHash, t.ExpireAt, t.UtiliseAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création token réinitialisation: %w", err)
	}
	return nil
}

func (r *TokenReinitialisationRepository) FindByHash(ctx context.Context, tokenHash string) (*auth.TokenReinitialisation, error) {
	const query = `
		SELECT id, utilisateur_id, token_hash, expire_at, utilise_at, created_at
		FROM tokens_reinitialisation WHERE token_hash = $1`

	var t auth.TokenReinitialisation
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, tokenHash).Scan(
		&t.ID, &t.UtilisateurID, &t.TokenHash, &t.ExpireAt, &t.UtiliseAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrTokenInvalide
	}
	if err != nil {
		return nil, fmt.Errorf("lecture token réinitialisation: %w", err)
	}
	return &t, nil
}

func (r *TokenReinitialisationRepository) MarquerUtilise(ctx context.Context, id string) error {
	const query = `UPDATE tokens_reinitialisation SET utilise_at = now() WHERE id = $1 AND utilise_at IS NULL`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("consommation token réinitialisation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrTokenInvalide
	}
	return nil
}
