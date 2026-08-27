// Package commun regroupe les éléments du domaine partagés par
// plusieurs modules (auth, kyc, et les modules à venir : wallet,
// cartes...) — l'utilisateur, le wallet, les règles KYC par pays, les
// erreurs génériques et l'audit trail. Rien ici n'appartient à un seul
// module.
package commun

import "errors"

// Erreurs métier partagées par toutes les entités du domaine.
// Elles sont volontairement génériques : c'est à la couche transport
// de les traduire en codes HTTP appropriés (voir handlers.MapErreurDomaine).
var (
	// Utilisateur / KYC
	ErrUtilisateurIntrouvable = errors.New("utilisateur introuvable")
	ErrEmailDejaUtilise       = errors.New("un compte existe déjà avec cet email")
	ErrTelephoneDejaUtilise   = errors.New("un compte existe déjà avec ce numéro de téléphone")
	ErrPaysNonSupporte        = errors.New("pays non supporté")
	ErrTransitionKycInvalide  = errors.New("transition de statut kyc invalide")
	ErrDonneesInvalides       = errors.New("données invalides")
	ErrPhotoProfilAbsente     = errors.New("aucune photo de profil")

	// Wallet
	ErrWalletIntrouvable        = errors.New("wallet introuvable")
	ErrSoldeInsuffisant         = errors.New("solde insuffisant")
	ErrWalletGele               = errors.New("wallet gelé")
	ErrMontantInvalide          = errors.New("montant invalide")
	ErrPlafondDepasse           = errors.New("plafond du wallet dépassé")
	ErrTransitionWalletInvalide = errors.New("transition de statut de wallet invalide")
)
