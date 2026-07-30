package commun

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/commun"
)

type UtilisateurRepository struct {
	pool *pgxpool.Pool
}

func NewUtilisateurRepository(pool *pgxpool.Pool) *UtilisateurRepository {
	return &UtilisateurRepository{pool: pool}
}

func (r *UtilisateurRepository) Create(ctx context.Context, u *commun.Utilisateur) error {
	const query = `
		INSERT INTO utilisateurs
			(id, nom, prenom, email, telephone, pays_code, mot_de_passe_hash, google_id, role, kyc_tier, kyc_statut, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	_, err := DbFromContext(ctx, r.pool).Exec(ctx, query,
		u.ID, u.Nom, u.Prenom, u.Email, u.Telephone, u.PaysCode,
		aNilSiVide(u.MotDePasseHash), aNilSiVide(u.GoogleID), u.Role,
		u.KycTier, u.KycStatut, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("création utilisateur: %w", err)
	}
	return nil
}

func (r *UtilisateurRepository) FindByID(ctx context.Context, id string) (*commun.Utilisateur, error) {
	return r.findOneBy(ctx, "id", id)
}

func (r *UtilisateurRepository) FindByEmail(ctx context.Context, email string) (*commun.Utilisateur, error) {
	return r.findOneBy(ctx, "email", email)
}

func (r *UtilisateurRepository) FindByTelephone(ctx context.Context, telephone string) (*commun.Utilisateur, error) {
	return r.findOneBy(ctx, "telephone", telephone)
}

func (r *UtilisateurRepository) FindByGoogleID(ctx context.Context, googleID string) (*commun.Utilisateur, error) {
	return r.findOneBy(ctx, "google_id", googleID)
}

// findOneBy centralise le SELECT ; colonne provient toujours d'un appel
// interne codé en dur (jamais d'une entrée utilisateur), donc pas de
// risque d'injection SQL malgré la construction de la requête.
func (r *UtilisateurRepository) findOneBy(ctx context.Context, colonne string, valeur any) (*commun.Utilisateur, error) {
	query := fmt.Sprintf(`
		SELECT id, nom, prenom, email, telephone, pays_code, mot_de_passe_hash, google_id, role, kyc_tier, kyc_statut, created_at, updated_at
		FROM utilisateurs WHERE %s = $1`, colonne)

	var u commun.Utilisateur
	var motDePasseHash, googleID *string
	err := DbFromContext(ctx, r.pool).QueryRow(ctx, query, valeur).Scan(
		&u.ID, &u.Nom, &u.Prenom, &u.Email, &u.Telephone, &u.PaysCode, &motDePasseHash, &googleID, &u.Role,
		&u.KycTier, &u.KycStatut, &u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, commun.ErrUtilisateurIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture utilisateur: %w", err)
	}

	u.MotDePasseHash = videSiNil(motDePasseHash)
	u.GoogleID = videSiNil(googleID)
	return &u, nil
}

func (r *UtilisateurRepository) UpdateStatutKyc(ctx context.Context, u *commun.Utilisateur) error {
	const query = `UPDATE utilisateurs SET kyc_tier = $1, kyc_statut = $2, updated_at = $3 WHERE id = $4`

	tag, err := DbFromContext(ctx, r.pool).Exec(ctx, query, u.KycTier, u.KycStatut, u.UpdatedAt, u.ID)
	if err != nil {
		return fmt.Errorf("mise à jour statut kyc: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return commun.ErrUtilisateurIntrouvable
	}
	return nil
}

func (r *UtilisateurRepository) UpdateMotDePasse(ctx context.Context, u *commun.Utilisateur) error {
	const query = `UPDATE utilisateurs SET mot_de_passe_hash = $1, updated_at = now() WHERE id = $2`

	tag, err := DbFromContext(ctx, r.pool).Exec(ctx, query, aNilSiVide(u.MotDePasseHash), u.ID)
	if err != nil {
		return fmt.Errorf("mise à jour mot de passe: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return commun.ErrUtilisateurIntrouvable
	}
	return nil
}

func (r *UtilisateurRepository) LierGoogleID(ctx context.Context, u *commun.Utilisateur) error {
	const query = `UPDATE utilisateurs SET google_id = $1, updated_at = $2 WHERE id = $3`

	tag, err := DbFromContext(ctx, r.pool).Exec(ctx, query, aNilSiVide(u.GoogleID), u.UpdatedAt, u.ID)
	if err != nil {
		return fmt.Errorf("liaison compte google: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return commun.ErrUtilisateurIntrouvable
	}
	return nil
}

// aNilSiVide/videSiNil font le pont entre le domaine (chaîne vide =
// absence de valeur) et Postgres (NULL = absence de valeur) pour
// mot_de_passe_hash et google_id, tous deux optionnels.
func aNilSiVide(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func videSiNil(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
