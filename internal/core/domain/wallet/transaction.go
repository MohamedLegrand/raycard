// Package wallet contient le domaine du portefeuille : les transactions
// de recharge (et, plus tard, retrait et financement de carte), rattachées
// au Wallet du module commun.
package wallet

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// TypeTransaction catégorise le mouvement de fonds.
type TypeTransaction string

const (
	TypeTransactionRecharge         TypeTransaction = "recharge"
	TypeTransactionRetrait          TypeTransaction = "retrait"
	TypeTransactionFinancementCarte TypeTransaction = "financement_carte"
	TypeTransactionAnnulationCarte  TypeTransaction = "annulation_carte"
)

// StatutTransaction représente le cycle de vie d'une transaction, en
// miroir du principe "créer en attente -> confirmation externe -> effet
// seulement après" appliqué à toute opération financière du wallet.
type StatutTransaction string

const (
	// StatutTransactionEnAttente : créée localement, pas encore envoyée à
	// l'agrégateur de paiement.
	StatutTransactionEnAttente StatutTransaction = "en_attente"
	// StatutTransactionEnvoyee : l'agrégateur a accusé réception de la
	// demande (ex: 202 PENDING). Ce statut est le verrou qui empêche un
	// second appel SDK pour la même transaction — une fois franchi, seule
	// une réconciliation manuelle peut renvoyer la demande.
	StatutTransactionEnvoyee StatutTransaction = "envoyee"
	StatutTransactionSucces  StatutTransaction = "succes"
	StatutTransactionEchouee StatutTransaction = "echouee"
)

// Transaction trace un mouvement de fonds sur un Wallet initié via un
// agrégateur de paiement externe (ex: HR-Skills Pay). Le montant brut est
// celui demandé par l'utilisateur ; les frais et le montant net ne sont
// connus qu'à la confirmation.
type Transaction struct {
	ID               string
	WalletID         string
	UtilisateurID    string
	Type             TypeTransaction
	Statut           StatutTransaction
	MontantCentimes  int64
	FraisCentimes    int64
	Devise           string
	Operateur        string
	Telephone        string
	ReferenceExterne string
	DisponibleLe     *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NouvelleTransactionRecharge crée une transaction de recharge en
// attente, avant tout appel à l'agrégateur de paiement. Le wallet n'est
// crédité qu'à la confirmation (voir MarquerSucces) : l'argent vient de
// l'utilisateur, on ne le compte qu'une fois reçu.
func NouvelleTransactionRecharge(walletID, utilisateurID, devise, operateur, telephone string, montantCentimes int64) (*Transaction, error) {
	return nouvelleTransaction(TypeTransactionRecharge, walletID, utilisateurID, devise, operateur, telephone, montantCentimes)
}

// NouvelleTransactionRetrait crée une transaction de retrait en attente.
// Contrairement à la recharge, le débit est appliqué immédiatement par
// l'appelant (voir wallet.WalletUseCase.InitierRetrait) : l'argent part
// vers l'utilisateur dès la confirmation de l'agrégateur, on ne peut
// jamais promettre un retrait qu'on ne peut pas couvrir.
func NouvelleTransactionRetrait(walletID, utilisateurID, devise, operateur, telephone string, montantCentimes int64) (*Transaction, error) {
	return nouvelleTransaction(TypeTransactionRetrait, walletID, utilisateurID, devise, operateur, telephone, montantCentimes)
}

// NouvelleTransactionFinancementCarte crée une transaction de financement
// de carte en attente. Contrairement à la recharge/au retrait, aucun
// opérateur ni téléphone Mobile Money n'est impliqué : c'est un mouvement
// interne entre le wallet et la carte, initié via l'agrégateur. Comme le
// retrait, le débit est appliqué immédiatement par l'appelant (voir
// carte.CarteUseCase.CreerCarte) — on ne peut pas promettre une carte
// qu'on ne peut pas financer.
func NouvelleTransactionFinancementCarte(walletID, utilisateurID, devise string, montantCentimes int64) (*Transaction, error) {
	return nouvelleTransactionSansOperateur(TypeTransactionFinancementCarte, walletID, utilisateurID, devise, montantCentimes)
}

// NouvelleTransactionAnnulationCarte crée une transaction de remboursement
// suite à l'annulation d'une carte : le solde qui restait dessus revient
// au wallet. Comme le financement, aucun opérateur ni téléphone Mobile
// Money n'est impliqué.
func NouvelleTransactionAnnulationCarte(walletID, utilisateurID, devise string, montantCentimes int64) (*Transaction, error) {
	return nouvelleTransactionSansOperateur(TypeTransactionAnnulationCarte, walletID, utilisateurID, devise, montantCentimes)
}

func nouvelleTransactionSansOperateur(typeTransaction TypeTransaction, walletID, utilisateurID, devise string, montantCentimes int64) (*Transaction, error) {
	if walletID == "" || utilisateurID == "" || devise == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if montantCentimes <= 0 {
		return nil, commun.ErrMontantInvalide
	}

	maintenant := time.Now().UTC()
	return &Transaction{
		ID:              commun.NewID(),
		WalletID:        walletID,
		UtilisateurID:   utilisateurID,
		Type:            typeTransaction,
		Statut:          StatutTransactionEnAttente,
		MontantCentimes: montantCentimes,
		Devise:          devise,
		CreatedAt:       maintenant,
		UpdatedAt:       maintenant,
	}, nil
}

func nouvelleTransaction(typeTransaction TypeTransaction, walletID, utilisateurID, devise, operateur, telephone string, montantCentimes int64) (*Transaction, error) {
	if walletID == "" || utilisateurID == "" || devise == "" || telephone == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if operateur == "" {
		return nil, ErrOperateurNonSupporte
	}
	if montantCentimes <= 0 {
		return nil, commun.ErrMontantInvalide
	}

	maintenant := time.Now().UTC()
	return &Transaction{
		ID:              commun.NewID(),
		WalletID:        walletID,
		UtilisateurID:   utilisateurID,
		Type:            typeTransaction,
		Statut:          StatutTransactionEnAttente,
		MontantCentimes: montantCentimes,
		Devise:          devise,
		Operateur:       operateur,
		Telephone:       telephone,
		CreatedAt:       maintenant,
		UpdatedAt:       maintenant,
	}, nil
}

// MarquerEnvoyee transitionne EnAttente -> Envoyee, une fois l'appel à
// l'agrégateur accepté. Ne doit jamais être appelée deux fois pour la
// même transaction.
func (t *Transaction) MarquerEnvoyee(referenceExterne string) error {
	if t.Statut != StatutTransactionEnAttente {
		return ErrTransitionTransactionInvalide
	}
	if referenceExterne == "" {
		return commun.ErrDonneesInvalides
	}
	t.ReferenceExterne = referenceExterne
	t.Statut = StatutTransactionEnvoyee
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// MarquerSucces transitionne Envoyee -> Succes suite à la confirmation de
// l'agrégateur (webhook payment.succeeded). disponibleLe est l'échéance à
// laquelle le montant net bascule d'en-attente vers disponible — n'a de
// sens que pour une recharge (nil pour un retrait, qui n'a pas de délai
// de retenue : l'argent a déjà quitté le wallet à l'initiation).
func (t *Transaction) MarquerSucces(fraisCentimes int64, disponibleLe *time.Time) error {
	if t.Statut != StatutTransactionEnvoyee {
		return ErrTransitionTransactionInvalide
	}
	if fraisCentimes < 0 || fraisCentimes >= t.MontantCentimes {
		return commun.ErrMontantInvalide
	}
	t.FraisCentimes = fraisCentimes
	t.DisponibleLe = disponibleLe
	t.Statut = StatutTransactionSucces
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// MarquerEchouee transitionne Envoyee -> Echouee suite à un
// payment.failed de l'agrégateur.
func (t *Transaction) MarquerEchouee() error {
	if t.Statut != StatutTransactionEnvoyee {
		return ErrTransitionTransactionInvalide
	}
	t.Statut = StatutTransactionEchouee
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// MontantNetCentimes est le montant effectivement crédité au wallet
// (brut moins frais). N'a de sens qu'une fois la transaction en Succes.
func (t *Transaction) MontantNetCentimes() int64 {
	return t.MontantCentimes - t.FraisCentimes
}
