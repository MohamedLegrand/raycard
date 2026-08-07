// Package wallet regroupe les ports sortants du module wallet :
// persistance des transactions et accès à l'agrégateur de paiement
// externe. Les implémentations concrètes vivent dans
// internal/infrastructure.
package wallet

import (
	"context"
	"time"

	"raycard/internal/core/domain/wallet"
)

// TransactionRepository persiste les transactions de wallet. Une absence
// de résultat doit être signalée par wallet.ErrTransactionIntrouvable.
type TransactionRepository interface {
	Create(ctx context.Context, t *wallet.Transaction) error
	FindByID(ctx context.Context, id string) (*wallet.Transaction, error)
	FindByReferenceExterne(ctx context.Context, referenceExterne string) (*wallet.Transaction, error)
	Update(ctx context.Context, t *wallet.Transaction) error

	// FindEnCoursByWalletID trouve une transaction encore EnAttente ou
	// Envoyee pour ce wallet, s'il y en a une. Utilisé pour empêcher deux
	// recharges simultanées sur le même wallet. Retourne
	// wallet.ErrTransactionIntrouvable si aucune n'est en cours.
	FindEnCoursByWalletID(ctx context.Context, walletID string) (*wallet.Transaction, error)

	// ListDisponiblesEcheance liste les transactions en Succes dont
	// DisponibleLe est dépassé et dont le montant net est encore en
	// attente côté wallet — utilisé par le job qui bascule les fonds
	// d'en-attente vers disponible après le délai de retenue.
	ListDisponiblesEcheance(ctx context.Context, avant time.Time) ([]*wallet.Transaction, error)

	// ListByWalletID liste l'historique complet des transactions d'un
	// wallet (recharge, retrait, financement de carte...), les plus
	// récentes d'abord.
	ListByWalletID(ctx context.Context, walletID string) ([]*wallet.Transaction, error)
}

// InitierCashInParams décrit une demande de collecte Mobile Money.
type InitierCashInParams struct {
	Telephone       string
	Operateur       string
	PaysCode        string
	Devise          string
	MontantCentimes int64
}

// InitierCashInResultat est renvoyé dès que l'agrégateur accuse réception
// de la demande — bien avant la confirmation effective de l'utilisateur
// sur son téléphone.
type InitierCashInResultat struct {
	ReferenceExterne string
}

// InitierCashOutParams décrit une demande de décaissement Mobile Money.
type InitierCashOutParams struct {
	Telephone       string
	Operateur       string
	PaysCode        string
	Devise          string
	MontantCentimes int64
}

// InitierCashOutResultat est renvoyé dès que l'agrégateur accuse
// réception de la demande de décaissement.
type InitierCashOutResultat struct {
	ReferenceExterne string
}

// TypeEvenementWebhook catégorise la notification asynchrone reçue de
// l'agrégateur de paiement.
type TypeEvenementWebhook string

const (
	EvenementPaiementReussi TypeEvenementWebhook = "payment.succeeded"
	EvenementPaiementEchoue TypeEvenementWebhook = "payment.failed"
)

// EvenementWebhook est la représentation, indépendante du fournisseur,
// d'une notification de confirmation de paiement.
type EvenementWebhook struct {
	Type             TypeEvenementWebhook
	ReferenceExterne string
	FraisCentimes    int64
}

// AgregateurPaiement isole tout appel à l'agrégateur de paiement externe
// (aujourd'hui HR-Skills Pay) derrière une interface propre au domaine.
// Le cœur métier ne connaît jamais le SDK concret : changer d'agrégateur
// un jour ne devrait toucher que l'implémentation de ce port, dans
// internal/infrastructure.
//
// Important : ce port n'accepte pas de clé d'idempotence fournie par
// l'appelant — le SDK hrpay actuel n'expose aucun moyen de la contrôler
// (il en génère une en interne à chaque appel). La protection contre un
// double appel se fait donc en amont, au niveau de la Transaction locale :
// InitierCashIn ne doit jamais être rappelée pour une transaction déjà
// passée au statut Envoyee (voir wallet.Transaction.MarquerEnvoyee).
type AgregateurPaiement interface {
	InitierCashIn(ctx context.Context, params InitierCashInParams) (*InitierCashInResultat, error)

	// InitierCashOut, comme InitierCashIn, ne doit jamais être rappelée
	// pour une transaction déjà passée au statut Envoyee.
	InitierCashOut(ctx context.Context, params InitierCashOutParams) (*InitierCashOutResultat, error)

	// ConstruireEvenementWebhook vérifie la signature HMAC du corps brut
	// reçu et retourne l'évènement décodé. Retourne
	// wallet.ErrWebhookSignatureInvalide si la signature ne correspond pas.
	ConstruireEvenementWebhook(corps []byte, signature string) (*EvenementWebhook, error)
}
