package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain"
)

type UtilisateurRepository struct {
	pool *pgxpool.Pool
}

func NewUtilisateurRepository(pool *pgxpool.Pool) *UtilisateurRepository {
	return &UtilisateurRepository{pool: pool}
}

func (r *UtilisateurRepository) Create(ctx context.Context, u *domain.Utilisateur) error {
	const query = `
		INSERT INTO utilisateurs
			(id, nom, prenom, email, telephone, pays_code, mot_de_passe_hash, role, kyc_tier, kyc_statut, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := dbFromContext(ctx, r.pool).Exec(ctx, query,
		u.ID, u.Nom, u.Prenom, u.Email, u.Telephone, u.PaysCode, u.MotDePasseHash, u.Role,
		u.KycTier, u.KycStatut, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création utilisateur: %w", err)
	}
	return nil
}

func (r *UtilisateurRepository) FindByID(ctx context.Context, id string) (*domain.Utilisateur, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *UtilisateurRepository) FindByEmail(ctx context.Context, email string) (*domain.Utilisateur, error) {
	return r.findOneBy(ctx, "email", email)
}

func (r *UtilisateurRepository) FindByTelephone(ctx context.Context, telephone string) (*domain.Utilisateur, error) {
	return r.findOneBy(ctx, "telephone", telephone)
}

// findOneBy centralise le SELECT ; colonne provient toujours d'un appel
// interne codé en dur (jamais d'une entrée utilisateur), donc pas de
// risque d'injection SQL malgré la construction de la requête.
func (r *UtilisateurRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*domain.Utilisateur, error) {
	query := fmt.Sprintf(`
		SELECT id, nom, prenom, email, telephone, pays_code, mot_de_passe_hash, role, kyc_tier, kyc_statut, created_at, updated_at
		FROM utilisateurs WHERE %s = $1`, colonne)

	var u domain.Utilisateur
	err := dbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur).Scan(
		&u.ID, &u.Nom, &u.Prenom, &u.Email, &u.Telephone, &u.PaysCode, &u.MotDePasseHash, &u.Role,
		&u.KycTier, &u.KycStatut, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUtilisateurIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture utilisateur: %w", err)
	}
	return &u, nil
}

func (r *UtilisateurRepository) UpdateStatutKyc(ctx context.Context, u *domain.Utilisateur) error {
	const query = `UPDATE utilisateurs SET kyc_tier = $1, kyc_statut = $2, updated_at = $3 WHERE id = $4`

	tag, err := dbFromContext(ctx, r.pool).Exec(ctx, query, u.KycTier, u.KycStatut, u.UpdatedAt, u.ID)
	if err != nil {
		return fmt.Errorf("mise à jour statut kyc: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUtilisateurIntrouvable
	}
	return nil
}

func (r *UtilisateurRepository) UpdateMotDePasse(ctx context.Context, u *domain.Utilisateur) error {
	const query = `UPDATE utilisateurs SET mot_de_passe_hash = $1, updated_at = now() WHERE id = $2`

	tag, err := dbFromContext(ctx, r.pool).Exec(ctx, query, u.MotDePasseHash, u.ID)
	if err != nil {
		return fmt.Errorf("mise à jour mot de passe: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUtilisateurIntrouvable
	}
	return nil
}
