package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/auth"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type VerrouConnexionRepository struct {
	pool *pgxpool.Pool
}

func NewVerrouConnexionRepository(pool *pgxpool.Pool) *VerrouConnexionRepository {
	return &VerrouConnexionRepository{pool: pool}
}

func (r *VerrouConnexionRepository) FindByUtilisateurID(ctx context.Context, utilisateurID string) (*auth.VerrouConnexion, error) {
	const query = `
		SELECT utilisateur_id, nombre_echecs, niveau_escalade, derniere_activite_at, verrouille_jusqua
		FROM verrous_connexion WHERE utilisateur_id = $1`

	var v auth.VerrouConnexion
	var derniereActiviteAt *time.Time
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, utilisateurID).Scan(
		&v.UtilisateurID, &v.NombreEchecs, &v.NiveauEscalade, &derniereActiviteAt, &v.VerrouilleJusqua,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrVerrouIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture verrou connexion: %w", err)
	}
	if derniereActiviteAt != nil {
		v.DerniereActiviteAt = *derniereActiviteAt
	}
	return &v, nil
}

// Sauvegarder crée ou met à jour la ligne de l'utilisateur (au plus une
// par utilisateur).
func (r *VerrouConnexionRepository) Sauvegarder(ctx context.Context, v *auth.VerrouConnexion) error {
	const query = `
		INSERT INTO verrous_connexion (utilisateur_id, nombre_echecs, niveau_escalade, derniere_activite_at, verrouille_jusqua)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (utilisateur_id) DO UPDATE
		SET nombre_echecs = EXCLUDED.nombre_echecs,
			niveau_escalade = EXCLUDED.niveau_escalade,
			derniere_activite_at = EXCLUDED.derniere_activite_at,
			verrouille_jusqua = EXCLUDED.verrouille_jusqua`

	var derniereActiviteAt *time.Time
	if !v.DerniereActiviteAt.IsZero() {
		derniereActiviteAt = &v.DerniereActiviteAt
	}

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		v.UtilisateurID, v.NombreEchecs, v.NiveauEscalade, derniereActiviteAt, v.VerrouilleJusqua,
	)
	if err != nil {
		return fmt.Errorf("sauvegarde verrou connexion: %w", err)
	}
	return nil
}
