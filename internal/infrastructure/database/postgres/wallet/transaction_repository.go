package wallet

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

func (r *TransactionRepository) Create(ctx context.Context, t *wallet.Transaction) error {
	const query = `
		INSERT INTO transactions_wallet
			(id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		t.ID, t.WalletID, t.UtilisateurID, t.Type, t.Statut, t.MontantCentimes, t.FraisCentimes, t.Devise, t.Operateur, t.Telephone,
		referenceExterneOuNil(t.ReferenceExterne), t.DisponibleLe, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création transaction wallet: %w", err)
	}
	return nil
}

func (r *TransactionRepository) FindByID(ctx context.Context, id string) (*wallet.Transaction, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *TransactionRepository) FindByReferenceExterne(ctx context.Context, referenceExterne string) (*wallet.Transaction, error) {
	return r.findOneBy(ctx, "reference_externe", referenceExterne)
}

// findOneBy : colonne provient toujours d'un appel interne codé en dur.
func (r *TransactionRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*wallet.Transaction, error) {
	query := fmt.Sprintf(`
		SELECT id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at
		FROM transactions_wallet WHERE %s = $1`, colonne)

	t, err := scanTransaction(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, wallet.ErrTransactionIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture transaction wallet: %w", err)
	}
	return t, nil
}

func (r *TransactionRepository) FindEnCoursByWalletID(ctx context.Context, walletID string) (*wallet.Transaction, error) {
	const query = `
		SELECT id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at
		FROM transactions_wallet
		WHERE wallet_id = $1 AND statut IN ('en_attente', 'envoyee')
		LIMIT 1`

	t, err := scanTransaction(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, walletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, wallet.ErrTransactionIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture transaction en cours: %w", err)
	}
	return t, nil
}

func (r *TransactionRepository) ListDisponiblesEcheance(ctx context.Context, avant time.Time) ([]*wallet.Transaction, error) {
	const query = `
		SELECT id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at
		FROM transactions_wallet
		WHERE statut = 'succes' AND disponible_le IS NOT NULL AND disponible_le <= $1`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, avant)
	if err != nil {
		return nil, fmt.Errorf("liste transactions échues: %w", err)
	}
	defer rows.Close()

	var transactions []*wallet.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture transaction échue: %w", err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste transactions échues: %w", err)
	}
	return transactions, nil
}

func (r *TransactionRepository) ListByWalletID(ctx context.Context, walletID string) ([]*wallet.Transaction, error) {
	const query = `
		SELECT id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at
		FROM transactions_wallet
		WHERE wallet_id = $1
		ORDER BY created_at DESC`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, walletID)
	if err != nil {
		return nil, fmt.Errorf("liste transactions wallet: %w", err)
	}
	defer rows.Close()

	var transactions []*wallet.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture transaction wallet: %w", err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste transactions wallet: %w", err)
	}
	return transactions, nil
}

func (r *TransactionRepository) ListToutes(ctx context.Context, filtre outputwallet.FiltreTransactions) ([]*wallet.Transaction, error) {
	query := `
		SELECT id, wallet_id, utilisateur_id, type, statut, montant_centimes, frais_centimes, devise, operateur, telephone, reference_externe, disponible_le, created_at, updated_at
		FROM transactions_wallet`

	var conditions []string
	var args []any
	if filtre.UtilisateurID != "" {
		args = append(args, filtre.UtilisateurID)
		conditions = append(conditions, fmt.Sprintf("utilisateur_id = $%d", len(args)))
	}
	if filtre.Statut != "" {
		args = append(args, filtre.Statut)
		conditions = append(conditions, fmt.Sprintf("statut = $%d", len(args)))
	}
	if filtre.Type != "" {
		args = append(args, filtre.Type)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*wallet.Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture transaction: %w", err)
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste transactions: %w", err)
	}
	return transactions, nil
}

func (r *TransactionRepository) Update(ctx context.Context, t *wallet.Transaction) error {
	const query = `
		UPDATE transactions_wallet
		SET statut = $1, frais_centimes = $2, reference_externe = $3, disponible_le = $4, updated_at = $5
		WHERE id = $6`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		t.Statut, t.FraisCentimes, referenceExterneOuNil(t.ReferenceExterne), t.DisponibleLe, t.UpdatedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("mise à jour transaction wallet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return wallet.ErrTransactionIntrouvable
	}
	return nil
}

// referenceExterneOuNil : reference_externe porte une contrainte UNIQUE en
// base — y stocker une chaîne vide interdirait toute deuxième transaction
// EnAttente simultanée (deux chaînes vides entreraient en conflit). NULL,
// lui, ne compte jamais comme un doublon.
func referenceExterneOuNil(referenceExterne string) *string {
	if referenceExterne == "" {
		return nil
	}
	return &referenceExterne
}

type ligne interface {
	Scan(dest ...any) error
}

func scanTransaction(row ligne) (*wallet.Transaction, error) {
	var t wallet.Transaction
	var referenceExterne *string
	err := row.Scan(
		&t.ID, &t.WalletID, &t.UtilisateurID, &t.Type, &t.Statut, &t.MontantCentimes, &t.FraisCentimes, &t.Devise, &t.Operateur, &t.Telephone,
		&referenceExterne, &t.DisponibleLe, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if referenceExterne != nil {
		t.ReferenceExterne = *referenceExterne
	}
	return &t, nil
}
