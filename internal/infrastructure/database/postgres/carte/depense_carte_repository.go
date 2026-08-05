package carte

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/carte"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type DepenseCarteRepository struct {
	pool *pgxpool.Pool
}

func NewDepenseCarteRepository(pool *pgxpool.Pool) *DepenseCarteRepository {
	return &DepenseCarteRepository{pool: pool}
}

func (r *DepenseCarteRepository) Create(ctx context.Context, d *carte.DepenseCarte) error {
	const query = `
		INSERT INTO depenses_carte (id, carte_id, montant_centimes, solde_avant_centimes, solde_apres_centimes, detected_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		d.ID, d.CarteID, d.MontantCentimes, d.SoldeAvantCentimes, d.SoldeApresCentimes, d.DetectedAt,
	)
	if err != nil {
		return fmt.Errorf("création dépense carte: %w", err)
	}
	return nil
}

func (r *DepenseCarteRepository) ListByCarteID(ctx context.Context, carteID string) ([]*carte.DepenseCarte, error) {
	const query = `
		SELECT id, carte_id, montant_centimes, solde_avant_centimes, solde_apres_centimes, detected_at
		FROM depenses_carte WHERE carte_id = $1 ORDER BY detected_at DESC`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, carteID)
	if err != nil {
		return nil, fmt.Errorf("liste dépenses carte: %w", err)
	}
	defer rows.Close()

	var depenses []*carte.DepenseCarte
	for rows.Next() {
		var d carte.DepenseCarte
		if err := rows.Scan(&d.ID, &d.CarteID, &d.MontantCentimes, &d.SoldeAvantCentimes, &d.SoldeApresCentimes, &d.DetectedAt); err != nil {
			return nil, fmt.Errorf("lecture dépense carte: %w", err)
		}
		depenses = append(depenses, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste dépenses carte: %w", err)
	}
	return depenses, nil
}
