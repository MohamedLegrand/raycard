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

type ChallengeEmpreinteRepository struct {
	pool *pgxpool.Pool
}

func NewChallengeEmpreinteRepository(pool *pgxpool.Pool) *ChallengeEmpreinteRepository {
	return &ChallengeEmpreinteRepository{pool: pool}
}

func (r *ChallengeEmpreinteRepository) Create(ctx context.Context, c *auth.ChallengeEmpreinte) error {
	const query = `
		INSERT INTO challenges_empreinte (id, cle_appareil_id, nonce, expire_at, utilise_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		c.ID, c.CleAppareilID, c.Nonce, c.ExpireAt, c.UtiliseAt, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création challenge empreinte: %w", err)
	}
	return nil
}

func (r *ChallengeEmpreinteRepository) FindByID(ctx context.Context, id string) (*auth.ChallengeEmpreinte, error) {
	const query = `
		SELECT id, cle_appareil_id, nonce, expire_at, utilise_at, created_at
		FROM challenges_empreinte WHERE id = $1`

	var c auth.ChallengeEmpreinte
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, id).Scan(
		&c.ID, &c.CleAppareilID, &c.Nonce, &c.ExpireAt, &c.UtiliseAt, &c.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrChallengeIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture challenge empreinte: %w", err)
	}
	return &c, nil
}

func (r *ChallengeEmpreinteRepository) Update(ctx context.Context, c *auth.ChallengeEmpreinte) error {
	const query = `UPDATE challenges_empreinte SET utilise_at = $1 WHERE id = $2`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, c.UtiliseAt, c.ID)
	if err != nil {
		return fmt.Errorf("mise à jour challenge empreinte: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrChallengeIntrouvable
	}
	return nil
}
