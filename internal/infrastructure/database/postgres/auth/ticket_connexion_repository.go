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

type TicketConnexionRepository struct {
	pool *pgxpool.Pool
}

func NewTicketConnexionRepository(pool *pgxpool.Pool) *TicketConnexionRepository {
	return &TicketConnexionRepository{pool: pool}
}

func (r *TicketConnexionRepository) Create(ctx context.Context, t *auth.TicketConnexion) error {
	const query = `
		INSERT INTO tickets_connexion
			(id, utilisateur_id, ticket_hash, code_hash, tentatives_restantes, expire_at, utilise_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		t.ID, t.UtilisateurID, t.TicketHash, t.CodeHash, t.TentativesRestantes, t.ExpireAt, t.UtiliseAt, t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création ticket connexion: %w", err)
	}
	return nil
}

func (r *TicketConnexionRepository) FindByHash(ctx context.Context, ticketHash string) (*auth.TicketConnexion, error) {
	const query = `
		SELECT id, utilisateur_id, ticket_hash, code_hash, tentatives_restantes, expire_at, utilise_at, created_at
		FROM tickets_connexion WHERE ticket_hash = $1`

	var t auth.TicketConnexion
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, ticketHash).Scan(
		&t.ID, &t.UtilisateurID, &t.TicketHash, &t.CodeHash, &t.TentativesRestantes, &t.ExpireAt, &t.UtiliseAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrTokenInvalide
	}
	if err != nil {
		return nil, fmt.Errorf("lecture ticket connexion: %w", err)
	}
	return &t, nil
}

func (r *TicketConnexionRepository) Update(ctx context.Context, t *auth.TicketConnexion) error {
	const query = `
		UPDATE tickets_connexion
		SET tentatives_restantes = $1, utilise_at = $2
		WHERE id = $3`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, t.TentativesRestantes, t.UtiliseAt, t.ID)
	if err != nil {
		return fmt.Errorf("mise à jour ticket connexion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrTokenInvalide
	}
	return nil
}
