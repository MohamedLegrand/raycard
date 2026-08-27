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

	// ListAll renvoie tous les dossiers quel que soit leur statut — pour
	// les KPI back-office, jamais pour la file de revue (ListEnAttente).
	ListAll(ctx context.Context) ([]*kyc.DossierKyc, error)
	Update(ctx context.Context, d *kyc.DossierKyc) error
}

// DocumentKycRepository persiste les documents d'identité téléversés et
// le texte que l'OCR en a extrait. Une absence de résultat doit être
// signalée par kyc.ErrDocumentKycIntrouvable.
type DocumentKycRepository interface {
	Create(ctx context.Context, d *kyc.DocumentKyc) error
	FindByID(ctx context.Context, id string) (*kyc.DocumentKyc, error)

	// ListByDossierKycID renvoie les documents d'une demande précise —
	// jamais tous les documents d'un utilisateur, pour ne pas mélanger
	// les pièces d'une éventuelle tentative précédente rejetée.
	ListByDossierKycID(ctx context.Context, dossierKycID string) ([]*kyc.DocumentKyc, error)
}
