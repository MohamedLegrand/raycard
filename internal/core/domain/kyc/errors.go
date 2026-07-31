package kyc

import "errors"

// Erreurs spécifiques aux dossiers de passage de palier KYC.
var (
	ErrDossierKycIntrouvable   = errors.New("dossier kyc introuvable")
	ErrDossierKycDejaEnAttente = errors.New("une demande de passage de palier est déjà en attente")
	ErrDocumentKycIntrouvable  = errors.New("document kyc introuvable")
	ErrFormatDocumentInvalide  = errors.New("format de document non supporté (jpg, jpeg ou png attendu)")
	ErrDossierKycNonModifiable = errors.New("ce dossier n'accepte plus de nouveaux documents")
)
