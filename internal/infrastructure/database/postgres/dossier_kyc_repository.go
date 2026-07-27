package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain"
)

type DossierKycRepository struct {
	pool *pgxpool.Pool
}

func NewDossierKycRepository(pool *pgxpool.Pool) *DossierKycRepository {
	return &DossierKycRepository{pool: pool}
}

func (r *DossierKycRepository) Create(ctx context.Context, d *domain.DossierKyc) error {
	const query = `
		INSERT INTO dossiers_kyc (id, utilisateur_id, tier_demande, statut, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := dbFromContext(ctx, r.pool).Exec(ctx, query, d.ID, d.UtilisateurID, d.TierDemande, d.Statut, d.CreatedAt)
	if err != nil {
		return fmt.Errorf("création dossier kyc: %w", err)
	}
	return nil
}

func (r *DossierKycRepository) FindByID(ctx context.Context, id string) (*domain.DossierKyc, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *DossierKycRepository) FindEnAttenteByUtilisateurID(ctx context.Context, utilisateurID string) (*domain.DossierKyc, error) {
	const query = `
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE utilisateur_id = $1 AND statut = 'en_attente'`
	return r.scanUne(dbFromContext(ctx, r.pool).QueryRow(ctx, query, utilisateurID))
}

// findOneBy : colonne provient toujours d'un appel interne codé en dur.
func (r *DossierKycRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*domain.DossierKyc, error) {
	query := fmt.Sprintf(`
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE %s = $1`, colonne)
	return r.scanUne(dbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur))
}

func (r *DossierKycRepository) scanUne(row pgx.Row) (*domain.DossierKyc, error) {
	var d domain.DossierKyc
	var motifRejet, adminID *string

	err := row.Scan(&d.ID, &d.UtilisateurID, &d.TierDemande, &d.Statut, &motifRejet, &adminID, &d.CreatedAt, &d.TraiteAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDossierKycIntrouvable
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

func (r *DossierKycRepository) ListEnAttente(ctx context.Context) ([]*domain.DossierKyc, error) {
	const query = `
		SELECT id, utilisateur_id, tier_demande, statut, motif_rejet, admin_id, created_at, traite_at
		FROM dossiers_kyc WHERE statut = 'en_attente' ORDER BY created_at ASC`

	rows, err := dbFromContext(ctx, r.pool).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liste dossiers kyc en attente: %w", err)
	}
	defer rows.Close()

	var dossiers []*domain.DossierKyc
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

func (r *DossierKycRepository) Update(ctx context.Context, d *domain.DossierKyc) error {
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

	tag, err := dbFromContext(ctx, r.pool).Exec(ctx, query, d.Statut, motifRejet, adminID, d.TraiteAt, d.ID)
	if err != nil {
		return fmt.Errorf("mise à jour dossier kyc: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDossierKycIntrouvable
	}
	return nil
}
