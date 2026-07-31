package kyc

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"raycard/internal/core/domain/kyc"
	"raycard/internal/infrastructure/database/postgres/commun"
)

type DocumentKycRepository struct {
	pool *pgxpool.Pool
}

func NewDocumentKycRepository(pool *pgxpool.Pool) *DocumentKycRepository {
	return &DocumentKycRepository{pool: pool}
}

func (r *DocumentKycRepository) Create(ctx context.Context, d *kyc.DocumentKyc) error {
	const query = `
		INSERT INTO documents_kyc (id, utilisateur_id, nom_fichier, chemin_fichier, texte_extrait, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := commun.DbFromContext(ctx, r.pool).Exec(ctx, query,
		d.ID, d.UtilisateurID, d.NomFichier, d.CheminFichier, d.TexteExtrait, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("création document kyc: %w", err)
	}
	return nil
}

func (r *DocumentKycRepository) FindByID(ctx context.Context, id string) (*kyc.DocumentKyc, error) {
	const query = `
		SELECT id, utilisateur_id, nom_fichier, chemin_fichier, texte_extrait, created_at
		FROM documents_kyc WHERE id = $1`

	var d kyc.DocumentKyc
	err := commun.DbFromContext(ctx, r.pool).QueryRow(ctx, query, id).Scan(
		&d.ID, &d.UtilisateurID, &d.NomFichier, &d.CheminFichier, &d.TexteExtrait, &d.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, kyc.ErrDocumentKycIntrouvable
	}
	if err != nil {
		return nil, fmt.Errorf("lecture document kyc: %w", err)
	}
	return &d, nil
}

func (r *DocumentKycRepository) ListByUtilisateurID(ctx context.Context, utilisateurID string) ([]*kyc.DocumentKyc, error) {
	const query = `
		SELECT id, utilisateur_id, nom_fichier, chemin_fichier, texte_extrait, created_at
		FROM documents_kyc WHERE utilisateur_id = $1 ORDER BY created_at DESC`

	rows, err := commun.DbFromContext(ctx, r.pool).Query(ctx, query, utilisateurID)
	if err != nil {
		return nil, fmt.Errorf("liste documents kyc: %w", err)
	}
	defer rows.Close()

	var documents []*kyc.DocumentKyc
	for rows.Next() {
		var d kyc.DocumentKyc
		if err := rows.Scan(&d.ID, &d.UtilisateurID, &d.NomFichier, &d.CheminFichier, &d.TexteExtrait, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("lecture document kyc: %w", err)
		}
		documents = append(documents, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("liste documents kyc: %w", err)
	}
	return documents, nil
}
