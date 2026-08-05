package wallet

import "errors"

// Erreurs spécifiques aux transactions de wallet (recharge, retrait...).
var (
	ErrTransactionIntrouvable        = errors.New("transaction introuvable")
	ErrTransactionDejaEnCours        = errors.New("une recharge est déjà en cours pour ce wallet")
	ErrTransitionTransactionInvalide = errors.New("transition de statut de transaction invalide")
	ErrWebhookSignatureInvalide      = errors.New("signature de webhook invalide")
	ErrWebhookDejaTraite             = errors.New("évènement webhook déjà traité")
	ErrOperateurNonSupporte          = errors.New("opérateur mobile money non supporté")
)
