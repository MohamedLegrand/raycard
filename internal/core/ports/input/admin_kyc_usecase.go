package input

import (
	"context"

	"raycard/internal/core/domain"
)

// AdminKycUseCase orchestre la revue back-office des dossiers KYC
// (Tier 2). Séparé de KycUseCase car ce sont des actions administrateur
// sensibles (voir audit_log), jamais accessibles à un client normal.
type AdminKycUseCase interface {
	ListerDossiersEnAttente(ctx context.Context) ([]*domain.DossierKyc, error)
	ApprouverDossier(ctx context.Context, adminID, dossierID string) error
	RejeterDossier(ctx context.Context, adminID, dossierID, motif string) error
}
