package commun

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/commun"
	outputcommun "raycard/internal/core/ports/output/commun"
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

func (r *AuditLogRepository) List(ctx context.Context, filtre outputcommun.FiltreAuditLog) ([]*commun.AuditLog, error) {
	query := `SELECT id, admin_id, action, cible_type, cible_id, details_json::text, created_at FROM audit_log`

	var conditions []string
	var args []any
	if filtre.AdminID != "" {
		args = append(args, filtre.AdminID)
		conditions = append(conditions, fmt.Sprintf("admin_id = $%d", len(args)))
	}
	if filtre.CibleType != "" {
		args = append(args, filtre.CibleType)
		conditions = append(conditions, fmt.Sprintf("cible_type = $%d", len(args)))
	}
	if filtre.CibleID != "" {
		args = append(args, filtre.CibleID)
		conditions = append(conditions, fmt.Sprintf("cible_id = $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := DbFromContext(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste audit log: %w", err)
	}
	defer rows.Close()

	var entrees []*commun.AuditLog
	for rows.Next() {
		var e commun.AuditLog
		var detailsJSON *string
		if err := rows.Scan(&e.ID, &e.AdminID, &e.Action, &e.CibleType, &e.CibleID, &detailsJSON, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("lecture entrée audit log: %w", err)
		}
		if detailsJSON != nil {
			e.DetailsJSON = *detailsJSON
		}
		entrees = append(entrees, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste audit log: %w", err)
	}
	return entrees, nil
}
