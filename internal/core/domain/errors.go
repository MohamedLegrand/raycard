package domain

import "errors"

// Erreurs métier partagées par toutes les entités du domaine.
// Elles sont volontairement génériques : c'est à la couche transport
// de les traduire en codes HTTP appropriés (voir handlers.mapErreurDomaine).
var (
	// Utilisateur / KYC
	ErrUtilisateurIntrouvable = errors.New("utilisateur introuvable")
	ErrEmailDejaUtilise       = errors.New("un compte existe déjà avec cet email")
	ErrTelephoneDejaUtilise   = errors.New("un compte existe déjà avec ce numéro de téléphone")
	ErrPaysNonSupporte        = errors.New("pays non supporté")
	ErrTransitionKycInvalide  = errors.New("transition de statut kyc invalide")
	ErrDonneesInvalides       = errors.New("données invalides")

	// Wallet
	ErrWalletIntrouvable = errors.New("wallet introuvable")
	ErrSoldeInsuffisant  = errors.New("solde insuffisant")
	ErrWalletGele        = errors.New("wallet gelé")
	ErrMontantInvalide   = errors.New("montant invalide")
	ErrPlafondDepasse    = errors.New("plafond du wallet dépassé")

	// Authentification
	ErrIdentifiantsInvalides = errors.New("email ou mot de passe invalide")
	ErrTokenInvalide         = errors.New("token invalide, expiré ou révoqué")
)
