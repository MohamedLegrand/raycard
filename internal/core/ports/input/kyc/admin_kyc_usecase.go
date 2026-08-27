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

	// ListerTousDossiers renvoie tous les dossiers, quel que soit leur
	// statut — pour les KPI back-office (répartition
	// en_attente/approuve/rejete), jamais pour la file de revue
	// elle-même (ListerDossiersEnAttente).
	ListerTousDossiers(ctx context.Context) ([]*kycdomain.DossierKyc, error)
	ApprouverDossier(ctx context.Context, adminID, dossierID string) error
	RejeterDossier(ctx context.Context, adminID, dossierID, motif string) error

	// ListerDocuments renvoie les documents téléversés pour une demande
	// précise (et le texte que l'OCR en a extrait), pour aider
	// l'administrateur à la traiter. Jamais tous les documents d'un
	// utilisateur : une tentative précédente rejetée ne doit pas se
	// mélanger à la demande en cours.
	ListerDocuments(ctx context.Context, dossierKycID string) ([]*kycdomain.DocumentKyc, error)

	// RecupererDocument renvoie le contenu brut d'un document (l'image
	// elle-même), pour que l'administrateur puisse l'examiner visuellement
	// pendant la revue — le texte OCR seul ne suffit jamais à une décision.
	RecupererDocument(ctx context.Context, documentID string) (*kycdomain.DocumentKyc, []byte, error)
}
