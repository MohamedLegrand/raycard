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

type CleAppareilRepository struct {
	pool *pgxpool.Pool
}

func NewCleAppareilRepository(pool *pgxpool.Pool) *CleAppareilRepository {
	return &CleAppareilRepository{pool: pool}
}

func (r *CleAppareilRepository) Create(ctx context.Context, c *auth.CleAppareil) error {
	const query = `
		INSERT INTO cles_appareil (id, utilisateur_id, cle_publique, nom_appareil, derniere_utilisation_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		c.ID, c.UtilisateurID, c.ClePublique, c.NomAppareil, c.DerniereUtilisationAt, c.RevokedAt, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création clé appareil: %w", err)
	}
	return nil
}

func (r *CleAppareilRepository) FindByID(ctx context.Context, id string) (*auth.CleAppareil, error) {
	const query = `
		SELECT id, utilisateur_id, cle_publique, nom_appareil, derniere_utilisation_at, revoked_at, created_at
		FROM cles_appareil WHERE id = $1`

	var c auth.CleAppareil
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, id).Scan(
		&c.ID, &c.UtilisateurID, &c.ClePublique, &c.NomAppareil, &c.DerniereUtilisationAt, &c.RevokedAt, &c.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrCleAppareilIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture clé appareil: %w", err)
	}
	return &c, nil
}

func (r *CleAppareilRepository) Update(ctx context.Context, c *auth.CleAppareil) error {
	const query = `
		UPDATE cles_appareil
		SET derniere_utilisation_at = $1, revoked_at = $2
		WHERE id = $3`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, c.DerniereUtilisationAt, c.RevokedAt, c.ID)
	if err != nil {
		return fmt.Errorf("mise à jour clé appareil: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrCleAppareilIntrouvable
	}
	return nil
}

func (r *CleAppareilRepository) RevokeAllForUtilisateur(ctx context.Context, utilisateurID string) error {
	const query = `UPDATE cles_appareil SET revoked_at = now() WHERE utilisateur_id = $1 AND revoked_at IS NULL`

	if _, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, utilisateurID); err != nil {
		return fmt.Errorf("révocation de tous les appareils: %w", err)
	}
	return nil
}
