package commun

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKeyTx struct{}

// dbtx est satisfaite à la fois par *pgxpool.Pool et par pgx.Tx : les
// repositories l'utilisent pour exécuter leurs requêtes indifféremment
// sur une connexion du pool ou sur la transaction en cours, sans code
// dupliqué.
type dbtx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager implémente output.TxManager avec une transaction pgx.
type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

// WithinTransaction ouvre une transaction, l'injecte dans le context
// transmis à fn, puis commit si fn réussit ou rollback sinon. Tout flux
// touchant plusieurs tables (ex: inscription = utilisateur + wallet)
// doit passer par ici pour rester ACID.
func (m *TxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ouverture transaction: %w", err)
	}
	// Rollback devient un no-op après un Commit réussi ; son erreur (ex:
	// pgx.ErrTxClosed) est donc intentionnellement ignorée ici.
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(context.WithValue(ctx, ctxKeyTx{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// DbFromContext retourne la transaction en cours si présente dans ctx,
// sinon le pool de connexions. Exportée pour que les sous-packages
// postgres/auth et postgres/kyc puissent l'utiliser depuis leurs propres
// repositories.
func DbFromContext(ctx context.Context, pool *pgxpool.Pool) dbtx {
	if tx, ok := ctx.Value(ctxKeyTx{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
