// Package commun implémente, avec pgx, les ports/output partagés par
// plusieurs modules : persistance utilisateur/wallet, règles KYC par
// pays, audit trail, gestion des transactions. Les requêtes sont
// écrites en SQL brut, sans ORM.
package commun

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connexion postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}
