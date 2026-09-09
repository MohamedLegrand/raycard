// Package kyc fournit des fakes des use cases KYC (inputkyc.KycUseCase,
// inputkyc.AdminKycUseCase) pour les tests HTTP de la couche transport —
// même principe que test/transport/http/carte : ports ENTRANTS fakés,
// pour vérifier le routing/la validation/le mapping d'erreurs sans
// retester la logique métier (déjà couverte côté service).
package kyc

import (
	"context"

	domainkyc "raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
)

type KycUseCaseFake struct {
	InscrireResultat    *inputkyc.InscriptionResultat
	InscrireErr         error
	DerniereReqInscrire inputkyc.InscriptionRequest

	DemanderTier2Resultat *domainkyc.DossierKyc
	DemanderTier2Err      error

	TeleverserDocumentResultat *domainkyc.DocumentKyc
	TeleverserDocumentErr      error
	DerniereReqTeleverser      inputkyc.TeleverserDocumentRequest

	ObtenirDossierCourantResultat *domainkyc.DossierKyc
	ObtenirDossierCourantErr      error

	DernierUtilisateurID string
}

func (f *KycUseCaseFake) Inscrire(_ context.Context, req inputkyc.InscriptionRequest) (*inputkyc.InscriptionResultat, error) {
	f.DerniereReqInscrire = req
	return f.InscrireResultat, f.InscrireErr
}

func (f *KycUseCaseFake) DemanderTier2(_ context.Context, utilisateurID string) (*domainkyc.DossierKyc, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.DemanderTier2Resultat, f.DemanderTier2Err
}

func (f *KycUseCaseFake) TeleverserDocument(_ context.Context, utilisateurID string, req inputkyc.TeleverserDocumentRequest) (*domainkyc.DocumentKyc, error) {
	f.DernierUtilisateurID = utilisateurID
	f.DerniereReqTeleverser = req
	return f.TeleverserDocumentResultat, f.TeleverserDocumentErr
}

func (f *KycUseCaseFake) ObtenirDossierCourant(_ context.Context, utilisateurID string) (*domainkyc.DossierKyc, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirDossierCourantResultat, f.ObtenirDossierCourantErr
}

// AdminKycUseCaseFake implémente inputkyc.AdminKycUseCase.
type AdminKycUseCaseFake struct {
	ListerDossiersEnAttenteResultat []*domainkyc.DossierKyc
	ListerDossiersEnAttenteErr      error

	ListerTousDossiersResultat []*domainkyc.DossierKyc
	ListerTousDossiersErr      error

	ApprouverDossierErr error

	RejeterDossierErr error
	DernierMotifRejet string

	ListerDocumentsResultat []*domainkyc.DocumentKyc
	ListerDocumentsErr      error

	RecupererDocumentResultat *domainkyc.DocumentKyc
	RecupererDocumentContenu  []byte
	RecupererDocumentErr      error

	DernierAdminID    string
	DernierDossierID  string
	DernierDocumentID string
}

func (f *AdminKycUseCaseFake) ListerDossiersEnAttente(_ context.Context) ([]*domainkyc.DossierKyc, error) {
	return f.ListerDossiersEnAttenteResultat, f.ListerDossiersEnAttenteErr
}

func (f *AdminKycUseCaseFake) ListerTousDossiers(_ context.Context) ([]*domainkyc.DossierKyc, error) {
	return f.ListerTousDossiersResultat, f.ListerTousDossiersErr
}

func (f *AdminKycUseCaseFake) ApprouverDossier(_ context.Context, adminID, dossierID string) error {
	f.DernierAdminID, f.DernierDossierID = adminID, dossierID
	return f.ApprouverDossierErr
}

func (f *AdminKycUseCaseFake) RejeterDossier(_ context.Context, adminID, dossierID, motif string) error {
	f.DernierAdminID, f.DernierDossierID, f.DernierMotifRejet = adminID, dossierID, motif
	return f.RejeterDossierErr
}

func (f *AdminKycUseCaseFake) ListerDocuments(_ context.Context, dossierKycID string) ([]*domainkyc.DocumentKyc, error) {
	f.DernierDossierID = dossierKycID
	return f.ListerDocumentsResultat, f.ListerDocumentsErr
}

func (f *AdminKycUseCaseFake) RecupererDocument(_ context.Context, documentID string) (*domainkyc.DocumentKyc, []byte, error) {
	f.DernierDocumentID = documentID
	return f.RecupererDocumentResultat, f.RecupererDocumentContenu, f.RecupererDocumentErr
}
