// Package wallet regroupe les interfaces (ports entrants) exposées par
// la couche application au transport pour le wallet et ses transactions.
package wallet

import (
	"context"

	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/wallet"
)

// InitierRechargeRequest transporte les données brutes d'une demande de
// recharge Mobile Money depuis le transport vers l'application.
type InitierRechargeRequest struct {
	Operateur       string
	Telephone       string
	MontantCentimes int64
}

// InitierRetraitRequest transporte les données brutes d'une demande de
// retrait Mobile Money depuis le transport vers l'application.
type InitierRetraitRequest struct {
	Operateur       string
	Telephone       string
	MontantCentimes int64
}

// WalletUseCase orchestre la consultation du wallet et ses transactions.
type WalletUseCase interface {
	ObtenirWallet(ctx context.Context, utilisateurID string) (*commun.Wallet, error)

	// InitierRecharge crée une transaction en attente et déclenche la
	// collecte Mobile Money auprès de l'agrégateur de paiement. Le wallet
	// n'est crédité qu'à la confirmation du paiement (voir TraiterWebhook),
	// jamais à cet appel.
	InitierRecharge(ctx context.Context, utilisateurID string, req InitierRechargeRequest) (*wallet.Transaction, error)

	// InitierRetrait débite immédiatement le solde disponible puis
	// déclenche le décaissement Mobile Money auprès de l'agrégateur.
	// Contrairement à la recharge, le débit précède la confirmation : une
	// fois l'agrégateur sollicité, l'argent part réellement vers
	// l'utilisateur — on ne peut jamais promettre un retrait qu'on ne
	// peut pas couvrir.
	InitierRetrait(ctx context.Context, utilisateurID string, req InitierRetraitRequest) (*wallet.Transaction, error)

	// TraiterWebhook vérifie et applique une notification de confirmation
	// de paiement reçue de l'agrégateur.
	TraiterWebhook(ctx context.Context, corps []byte, signature string) error

	// BasculerFondsEcheus fait passer d'en-attente à disponible les
	// montants dont le délai de retenue de l'agrégateur est écoulé.
	// Appelé périodiquement par un job planifié, jamais par une requête
	// utilisateur. Retourne le nombre de transactions basculées.
	BasculerFondsEcheus(ctx context.Context) (int, error)
}
