// Package kyc regroupe les ports sortants propres à la revue KYC
// manuelle. Les implémentations concrètes vivent dans
// internal/infrastructure.
package kyc

import (
	"context"

	"raycard/internal/core/domain/kyc"
)

// DossierKycRepository persiste les demandes de passage de palier KYC.
type DossierKycRepository interface {
	Create(ctx context.Context, d *kyc.DossierKyc) error
	FindByID(ctx context.Context, id string) (*kyc.DossierKyc, error)
	FindEnAttenteByUtilisateurID(ctx context.Context, utilisateurID string) (*kyc.DossierKyc, error)
	ListEnAttente(ctx context.Context) ([]*kyc.DossierKyc, error)
	Update(ctx context.Context, d *kyc.DossierKyc) error
}
