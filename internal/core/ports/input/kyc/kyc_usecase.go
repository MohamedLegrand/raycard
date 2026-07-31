// Package kyc regroupe les interfaces (ports entrants) exposées par la
// couche application au transport pour l'inscription et la revue KYC.
package kyc

import (
	"context"

	"raycard/internal/core/domain/commun"
	kycdomain "raycard/internal/core/domain/kyc"
)

// InscriptionRequest transporte les données brutes d'inscription
// depuis le transport vers l'application. Ce n'est pas une entité du
// domaine : le mot de passe y est encore en clair (le hachage est fait
// par l'implémentation de KycUseCase).
type InscriptionRequest struct {
	Nom        string
	Prenom     string
	Email      string
	Telephone  string
	PaysCode   string
	MotDePasse string
}

// InscriptionResultat regroupe l'utilisateur et le wallet créés
// atomiquement lors de l'inscription.
type InscriptionResultat struct {
	Utilisateur *commun.Utilisateur
	Wallet      *commun.Wallet
}

// TeleverserDocumentRequest transporte le fichier brut et sa
// catégorisation. DossierKycID rattache le document à une demande
// précise : jamais à l'utilisateur seul, pour ne pas mélanger les
// pièces d'une éventuelle tentative précédente rejetée.
type TeleverserDocumentRequest struct {
	DossierKycID string
	TypeDocument kycdomain.TypeDocument
	NomFichier   string
	Contenu      []byte
}

// KycUseCase orchestre l'inscription et le cycle de vie du dossier KYC
// d'un utilisateur.
type KycUseCase interface {
	Inscrire(ctx context.Context, req InscriptionRequest) (*InscriptionResultat, error)

	// DemanderTier2 crée une demande de passage au Tier 2 pour
	// l'utilisateur authentifié donné. Toujours revue manuellement.
	DemanderTier2(ctx context.Context, utilisateurID string) (*kycdomain.DossierKyc, error)

	// TeleverserDocument stocke un document d'identité et en extrait le
	// texte par OCR local (Tesseract). Le texte extrait n'est qu'une aide
	// à la saisie pour l'administrateur qui traitera le dossier — aucune
	// décision n'est prise automatiquement à partir de son contenu. Le
	// dossier ciblé doit appartenir à l'utilisateur authentifié et être
	// encore en attente de revue.
	TeleverserDocument(ctx context.Context, utilisateurID string, req TeleverserDocumentRequest) (*kycdomain.DocumentKyc, error)
}
