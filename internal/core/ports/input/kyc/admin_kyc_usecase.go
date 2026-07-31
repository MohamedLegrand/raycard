package kyc

import (
	"context"

	kycdomain "raycard/internal/core/domain/kyc"
)

// AdminKycUseCase orchestre la revue back-office des dossiers KYC
// (Tier 2). Séparé de KycUseCase car ce sont des actions administrateur
// sensibles (voir audit_log), jamais accessibles à un client normal.
type AdminKycUseCase interface {
	ListerDossiersEnAttente(ctx context.Context) ([]*kycdomain.DossierKyc, error)
	ApprouverDossier(ctx context.Context, adminID, dossierID string) error
	RejeterDossier(ctx context.Context, adminID, dossierID, motif string) error

	// ListerDocuments renvoie les documents d'identité téléversés par un
	// utilisateur (et le texte que l'OCR en a extrait), pour aider
	// l'administrateur à traiter son dossier de passage de palier.
	ListerDocuments(ctx context.Context, utilisateurID string) ([]*kycdomain.DocumentKyc, error)

	// RecupererDocument renvoie le contenu brut d'un document (l'image
	// elle-même), pour que l'administrateur puisse l'examiner visuellement
	// pendant la revue — le texte OCR seul ne suffit jamais à une décision.
	RecupererDocument(ctx context.Context, documentID string) (*kycdomain.DocumentKyc, []byte, error)
}
