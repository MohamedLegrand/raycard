package commun

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/commun"
)

type AuditLogRepository struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

func (r *AuditLogRepository) Create(ctx context.Context, entry *commun.AuditLog) error {
	const query = `
		INSERT INTO audit_log (id, admin_id, action, cible_type, cible_id, details_json, created_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::jsonb, $7)`

	_, err := DbFromContext(ctx, r.pool).Exec(ctx, query,
		entry.ID, entry.AdminID, entry.Action, entry.CibleType, entry.CibleID, entry.DetailsJSON, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création entrée audit log: %w", err)
	}
	return nil
}
