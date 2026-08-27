// Package kyc implémente, avec pgx, le port/output propre à la revue
// KYC manuelle. Les requêtes sont écrites en SQL brut, sans ORM.
package kyc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/kyc"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type DossierKycRepository struct {
	pool *pgxpool.Pool
}

func NewDossierKycRepository(pool *pgxpool.Pool) *DossierKycRepository {
	return &DossierKycRepository{pool: pool}
}

func (r *DossierKycRepository) Create(ctx context.Context, d *kyc.DossierKyc) error {
	const query = `
		INSERT INTO dossiers_kyc (id, utilisateur_id, tier_demande, statut, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, d.ID, d.UtilisateurID, d.TierDemande, d.Statut, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("création dossier kyc: %w", err)
	}
	return nil
}

func (r *DossierKycRepository) FindByID(ctx context.Context, id string) (*kyc.DossierKyc, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *DossierKycRepository) FindEnAttenteByUtilisateurID(ctx context.Context, utilisateurID string) (*kyc.DossierKyc, error) {
	const query = `
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE utilisateur_id = $1 AND statut = 'en_attente'`
	return r.scanUne(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, utilisateurID))
}

// findOneBy : colonne provient toujours d'un appel interne codé en dur.
func (r *DossierKycRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*kyc.DossierKyc, error) {
	query := fmt.Sprintf(`
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE %s = $1`, colonne)
	return r.scanUne(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur))
}

func (r *DossierKycRepository) scanUne(row pgx.Row) (*kyc.DossierKyc, error) {
	var d kyc.DossierKyc
	var motifRejet, adminID *string

	err := row.Scan(&d.ID, &d.UtilisateurID, &d.TierDemande, &d.Statut, &motifRejet, &adminID, &d.CreatedAt, &d.TraiteAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, kyc.ErrDossierKycIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture dossier kyc: %w", err)
	}

	if motifRejet != nil {
		d.MotifRejet = *motifRejet
	}
	if adminID != nil {
		d.AdminID = *adminID
	}
	return &d, nil
}

func (r *DossierKycRepository) ListEnAttente(ctx context.Context) ([]*kyc.DossierKyc, error) {
	const query = `
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE statut = 'en_attente' ORDER BY created_at ASC`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liste dossiers kyc en attente: %w", err)
	}
	defer rows.Close()

	var dossiers []*kyc.DossierKyc
	for rows.Next() {
		d, err := r.scanUne(rows)
		if err != nil {
			return nil, err
		}
		dossiers = append(dossiers, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste dossiers kyc en attente: %w", err)
	}
	return dossiers, nil
}

// ListAll renvoie tous les dossiers, quel que soit leur statut — pour
// les KPI back-office (répartition en_attente/approuve/rejete), jamais
// pour la file de revue elle-même (voir ListEnAttente).
func (r *DossierKycRepository) ListAll(ctx context.Context) ([]*kyc.DossierKyc, error) {
	const query = `
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc ORDER BY created_at DESC`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liste tous les dossiers kyc: %w", err)
	}
	defer rows.Close()

	var dossiers []*kyc.DossierKyc
	for rows.Next() {
		d, err := r.scanUne(rows)
		if err != nil {
			return nil, err
		}
		dossiers = append(dossiers, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste tous les dossiers kyc: %w", err)
	}
	return dossiers, nil
}

func (r *DossierKycRepository) Update(ctx context.Context, d *kyc.DossierKyc) error {
	const query = `
		UPDATE dossiers_kyc
		SET statut = $1, motif_rejet = $2, admin_id = $3, traite_at = $4
		WHERE id = $5`

	var adminID *string
	if d.AdminID != "" {
		adminID = &d.AdminID
	}
	var motifRejet *string
	if d.MotifRejet != "" {
		motifRejet = &d.MotifRejet
	}

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query, d.Statut, motifRejet, adminID, d.TraiteAt, d.ID)
	if err != nil {
		return fmt.Errorf("mise à jour dossier kyc: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return kyc.ErrDossierKycIntrouvable
	}
	return nil
}
