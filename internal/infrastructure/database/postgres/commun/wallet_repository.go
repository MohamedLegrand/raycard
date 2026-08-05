package commun

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/commun"
)

type WalletRepository struct {
	pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

func (r *WalletRepository) Create(ctx context.Context, w *commun.Wallet) error {
	const query = `
		INSERT INTO wallets
			(id, utilisateur_id, pays_code, devise, solde_disponible_centimes, solde_en_attente_centimes, plafond_solde_centimes, statut, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := DbFromContext(ctx, r.pool).Exec(ctx, query,
		w.ID, w.UtilisateurID, w.PaysCode, w.Devise, w.SoldeDisponibleCentimes, w.SoldeEnAttenteCentimes, w.PlafondSoldeCentimes, w.Statut, w.CreatedAt, w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création wallet: %w", err)
	}
	return nil
}

func (r *WalletRepository) FindByID(ctx context.Context, id string) (*commun.Wallet, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *WalletRepository) FindByUtilisateurID(ctx context.Context, utilisateurID string) (*commun.Wallet, error) {
	return r.findOneBy(ctx, "utilisateur_id", utilisateurID)
}

// findOneBy : colonne provient toujours d'un appel interne codé en dur.
func (r *WalletRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*commun.Wallet, error) {
	query := fmt.Sprintf(`
		SELECT id, utilisateur_id, pays_code, devise, solde_disponible_centimes, solde_en_attente_centimes, plafond_solde_centimes, statut, created_at, updated_at
		FROM wallets WHERE %s = $1`, colonne)

	var w commun.Wallet
	err := DbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur).Scan(
		&w.ID, &w.UtilisateurID, &w.PaysCode, &w.Devise, &w.SoldeDisponibleCentimes, &w.SoldeEnAttenteCentimes, &w.PlafondSoldeCentimes, &w.Statut, &w.CreatedAt, &w.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, commun.ErrWalletIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture wallet: %w", err)
	}
	return &w, nil
}

func (r *WalletRepository) UpdateSolde(ctx context.Context, w *commun.Wallet) error {
	const query = `UPDATE wallets SET solde_disponible_centimes = $1, solde_en_attente_centimes = $2, statut = $3, updated_at = $4 WHERE id = $5`

	tag, err := DbFromContext(ctx, r.pool).Exec(ctx, query, w.SoldeDisponibleCentimes, w.SoldeEnAttenteCentimes, w.Statut, w.UpdatedAt, w.ID)
	if err != nil {
		return fmt.Errorf("mise à jour solde wallet: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return commun.ErrWalletIntrouvable
	}
	return nil
}
