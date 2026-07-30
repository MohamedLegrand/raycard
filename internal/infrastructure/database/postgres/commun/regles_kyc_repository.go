package commun

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/commun"
)

type ReglesKycRepository struct {
	pool *pgxpool.Pool
}

func NewReglesKycRepository(pool *pgxpool.Pool) *ReglesKycRepository {
	return &ReglesKycRepository{pool: pool}
}

func (r *ReglesKycRepository) FindByPaysEtTier(ctx context.Context, paysCode string, tier commun.KycTier) (*commun.RegleKyc, error) {
	const query = `
		SELECT pays_code, tier, devise, plafond_solde_centimes, plafond_mensuel_centimes
		FROM regles_kyc_pays WHERE pays_code = $1 AND tier = $2`

	var regle commun.RegleKyc
	err := DbFromContext(ctx, r.pool).QueryRow(ctx, query, paysCode, tier).Scan(
		&regle.PaysCode, &regle.Tier, &regle.Devise, &regle.PlafondSoldeCentimes, &regle.PlafondMensuelCentimes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, commun.ErrPaysNonSupporte
	}
	if err != nil {
		return nil, fmt.Errorf("lecture règle kyc: %w", err)
	}
	return &regle, nil
}
