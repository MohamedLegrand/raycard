// Package auth implémente, avec pgx, les ports/output propres à
// l'authentification : sessions (refresh tokens), réinitialisation de
// mot de passe, 2FA. Les requêtes sont écrites en SQL brut, sans ORM.
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

type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, rt *auth.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (id, utilisateur_id, token_hash, expire_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		rt.ID, rt.UtilisateurID, rt.TokenHash, rt.ExpireAt, rt.RevokedAt, rt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création refresh token: %w", err)
	}
	return nil
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	const query = `
		SELECT id, utilisateur_id, token_hash, expire_at, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = $1`

	var rt auth.RefreshToken
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, tokenHash).Scan(
		&rt.ID, &rt.UtilisateurID, &rt.TokenHash, &rt.ExpireAt, &rt.RevokedAt, &rt.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrTokenInvalide
	}
	if err != nil {
		return nil, fmt.Errorf("lecture refresh token: %w", err)
	}
	return &rt, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id string) error {
	const query = `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("révocation refresh token: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrTokenInvalide
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUtilisateur(ctx context.Context, utilisateurID string) error {
	const query = `UPDATE refresh_tokens SET revoked_at = now() WHERE utilisateur_id = $1 AND revoked_at IS NULL`

	if _, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, utilisateurID); err != nil {
		return fmt.Errorf("révocation de toutes les sessions: %w", err)
	}
	return nil
}
