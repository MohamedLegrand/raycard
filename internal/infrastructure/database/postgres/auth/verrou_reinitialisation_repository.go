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

type VerrouReinitialisationRepository struct {
	pool *pgxpool.Pool
}

func NewVerrouReinitialisationRepository(pool *pgxpool.Pool) *VerrouReinitialisationRepository {
	return &VerrouReinitialisationRepository{pool: pool}
}

func (r *VerrouReinitialisationRepository) FindByAdresseIP(ctx context.Context, adresseIP string) (*auth.VerrouReinitialisation, error) {
	const query = `
		SELECT adresse_ip, nombre_echecs, niveau_escalade, derniere_activite_at, verrouille_jusqua
		FROM verrous_reinitialisation WHERE adresse_ip = $1`

	var v auth.VerrouReinitialisation
	var derniereActiviteAt *time.Time
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, adresseIP).Scan(
		&v.AdresseIP, &v.NombreEchecs, &v.NiveauEscalade, &derniereActiviteAt, &v.VerrouilleJusqua,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrVerrouReinitialisationIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture verrou réinitialisation: %w", err)
	}
	if derniereActiviteAt != nil {
		v.DerniereActiviteAt = *derniereActiviteAt
	}
	return &v, nil
}

// Sauvegarder crée ou met à jour la ligne de l'IP (au plus une par IP).
func (r *VerrouReinitialisationRepository) Sauvegarder(ctx context.Context, v *auth.VerrouReinitialisation) error {
	const query = `
		INSERT INTO verrous_reinitialisation (adresse_ip, nombre_echecs, niveau_escalade, derniere_activite_at, verrouille_jusqua)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (adresse_ip) DO UPDATE
		SET nombre_echecs = EXCLUDED.nombre_echecs,
			niveau_escalade = EXCLUDED.niveau_escalade,
			derniere_activite_at = EXCLUDED.derniere_activite_at,
			verrouille_jusqua = EXCLUDED.verrouille_jusqua`

	var derniereActiviteAt *time.Time
	if !v.DerniereActiviteAt.IsZero() {
		derniereActiviteAt = &v.DerniereActiviteAt
	}

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		v.AdresseIP, v.NombreEchecs, v.NiveauEscalade, derniereActiviteAt, v.VerrouilleJusqua,
	)
	if err != nil {
		return fmt.Errorf("sauvegarde verrou réinitialisation: %w", err)
	}
	return nil
}
