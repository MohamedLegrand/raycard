package kyc

import "errors"

// Erreurs spécifiques aux dossiers de passage de palier KYC.
var (
	ErrDossierKycIntrouvable   = errors.New("dossier kyc introuvable")
	ErrDossierKycDejaEnAttente = errors.New("une demande de passage de palier est déjà en attente")
)
