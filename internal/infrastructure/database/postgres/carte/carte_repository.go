package carte

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type CarteRepository struct {
	pool *pgxpool.Pool
}

func NewCarteRepository(pool *pgxpool.Pool) *CarteRepository {
	return &CarteRepository{pool: pool}
}

func (r *CarteRepository) Create(ctx context.Context, c *carte.Carte) error {
	const query = `
		INSERT INTO cartes
			(id, utilisateur_id, wallet_id, id_externe, label, devise, montant_charge_centimes, solde_centimes, prochaine_verification_at, niveau_verification, statut, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		c.ID, c.UtilisateurID, c.WalletID, c.IDExterne, c.Label, c.Devise, c.MontantChargeCentimes, c.SoldeCentimes,
		c.ProchaineVerificationAt, c.NiveauVerification, c.Statut, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création carte: %w", err)
	}
	return nil
}

func (r *CarteRepository) FindByID(ctx context.Context, id string) (*carte.Carte, error) {
	const query = `
		SELECT id, utilisateur_id, wallet_id, id_externe, label, devise, montant_charge_centimes, solde_centimes, prochaine_verification_at, niveau_verification, statut, created_at, updated_at
		FROM cartes WHERE id = $1`

	c, err := scanCarte(commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, carte.ErrCarteIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture carte: %w", err)
	}
	return c, nil
}

func (r *CarteRepository) ListByUtilisateurID(ctx context.Context, utilisateurID string) ([]*carte.Carte, error) {
	const query = `
		SELECT id, utilisateur_id, wallet_id, id_externe, label, devise, montant_charge_centimes, solde_centimes, prochaine_verification_at, niveau_verification, statut, created_at, updated_at
		FROM cartes WHERE utilisateur_id = $1 ORDER BY created_at DESC`

	return r.listBy(ctx, query, utilisateurID)
}

func (r *CarteRepository) ListAVerifier(ctx context.Context, avant time.Time) ([]*carte.Carte, error) {
	const query = `
		SELECT id, utilisateur_id, wallet_id, id_externe, label, devise, montant_charge_centimes, solde_centimes, prochaine_verification_at, niveau_verification, statut, created_at, updated_at
		FROM cartes WHERE statut = 'active' AND prochaine_verification_at <= $1`

	return r.listBy(ctx, query, avant)
}

func (r *CarteRepository) ListToutes(ctx context.Context, filtre outputcarte.FiltreCartes) ([]*carte.Carte, error) {
	query := `
		SELECT id, utilisateur_id, wallet_id, id_externe, label, devise, montant_charge_centimes, solde_centimes, prochaine_verification_at, niveau_verification, statut, created_at, updated_at
		FROM cartes`

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
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	return r.listBy(ctx, query, args...)
}

func (r *CarteRepository) listBy(ctx context.Context, query string, args ...any) ([]*carte.Carte, error) {
	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste cartes: %w", err)
	}
	defer rows.Close()

	var cartes []*carte.Carte
	for rows.Next() {
		c, err := scanCarte(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture carte: %w", err)
		}
		cartes = append(cartes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste cartes: %w", err)
	}
	return cartes, nil
}

func (r *CarteRepository) Update(ctx context.Context, c *carte.Carte) error {
	const query = `
		UPDATE cartes
		SET solde_centimes = $1, prochaine_verification_at = $2, niveau_verification = $3, statut = $4, updated_at = $5
		WHERE id = $6`

	tag, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		c.SoldeCentimes, c.ProchaineVerificationAt, c.NiveauVerification, c.Statut, c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("mise à jour carte: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return carte.ErrCarteIntrouvable
	}
	return nil
}

type ligne interface {
	Scan(dest ...any) error
}

func scanCarte(row ligne) (*carte.Carte, error) {
	var c carte.Carte
	err := row.Scan(
		&c.ID, &c.UtilisateurID, &c.WalletID, &c.IDExterne, &c.Label, &c.Devise, &c.MontantChargeCentimes, &c.SoldeCentimes,
		&c.ProchaineVerificationAt, &c.NiveauVerification, &c.Statut, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}
