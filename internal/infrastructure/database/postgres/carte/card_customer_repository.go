package carte

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/carte"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type CardCustomerRepository struct {
	pool *pgxpool.Pool
}

func NewCardCustomerRepository(pool *pgxpool.Pool) *CardCustomerRepository {
	return &CardCustomerRepository{pool: pool}
}

func (r *CardCustomerRepository) Create(ctx context.Context, c *carte.CardCustomer) error {
	const query = `
		INSERT INTO card_customers (id, utilisateur_id, id_externe, statut, motif_rejet, created_at, updated_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		c.ID, c.UtilisateurID, c.IDExterne, c.Statut, c.MotifRejet, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création card customer: %w", err)
	}
	return nil
}

func (r *CardCustomerRepository) FindByUtilisateurID(ctx context.Context, utilisateurID string) (*carte.CardCustomer, error) {
	const query = `
		SELECT id, utilisateur_id, COALESCE(id_externe, ''), statut, motif_rejet, created_at, updated_at
		FROM card_customers WHERE utilisateur_id = $1`

	c, err := scanCardCustomer(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, utilisateurID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, carte.ErrCardCustomerIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture card customer: %w", err)
	}
	return c, nil
}

func (r *CardCustomerRepository) Update(ctx context.Context, c *carte.CardCustomer) error {
	const query = `
		UPDATE card_customers
		SET id_externe = NULLIF($1, ''), statut = $2, motif_rejet = $3, updated_at = $4
		WHERE id = $5`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, c.IDExterne, c.Statut, c.MotifRejet, c.UpdatedAt, c.ID)
	if err != nil {
		return fmt.Errorf("mise à jour card customer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return carte.ErrCardCustomerIntrouvable
	}
	return nil
}

// scanCardCustomer réutilise le type ligne défini dans carte_repository.go
// (même package) — pas besoin d'une seconde interface de scan.
func scanCardCustomer(row ligne) (*carte.CardCustomer, error) {
	var c carte.CardCustomer
	if err := row.Scan(&c.ID, &c.UtilisateurID, &c.IDExterne, &c.Statut, &c.MotifRejet, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	return &c, nil
}
