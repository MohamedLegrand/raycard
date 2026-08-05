package commun

import "time"

// StatutWallet représente l'état opérationnel d'un wallet.
type StatutWallet string

const (
	StatutWalletActif StatutWallet = "actif"
	StatutWalletGele  StatutWallet = "gele"
)

// Wallet est le portefeuille Mobile Money d'un utilisateur. Les montants
// sont exprimés en unité mineure de la devise ("centimes") pour éviter
// tout calcul en virgule flottante ; pour une devise sans décimale
// (ex: XOF), l'unité mineure coïncide avec l'unité principale.
//
// Le solde est scindé en deux : Disponible (utilisable immédiatement) et
// EnAttente (crédité mais bloqué, ex: le délai de retenue de 48h imposé
// par l'agrégateur de paiement après une recharge Mobile Money). Le
// plafond réglementaire du palier KYC s'applique au solde total des deux.
type Wallet struct {
	ID                      string
	UtilisateurID           string
	PaysCode                string
	Devise                  string // ISO 4217, ex: "XOF"
	SoldeDisponibleCentimes int64
	SoldeEnAttenteCentimes  int64
	PlafondSoldeCentimes    int64
	Statut                  StatutWallet
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// NouveauWallet crée un wallet vide (solde nul) et actif, borné par le
// plafond du palier KYC du pays de l'utilisateur.
func NouveauWallet(utilisateurID, paysCode, devise string, plafondSoldeCentimes int64) (*Wallet, error) {
	if utilisateurID == "" || paysCode == "" || devise == "" {
		return nil, ErrDonneesInvalides
	}
	if plafondSoldeCentimes < 0 {
		return nil, ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()

	return &Wallet{
		ID:                      NewID(),
		UtilisateurID:           utilisateurID,
		PaysCode:                paysCode,
		Devise:                  devise,
		SoldeDisponibleCentimes: 0,
		SoldeEnAttenteCentimes:  0,
		PlafondSoldeCentimes:    plafondSoldeCentimes,
		Statut:                  StatutWalletActif,
		CreatedAt:               maintenant,
		UpdatedAt:               maintenant,
	}, nil
}

// SoldeTotalCentimes est le solde soumis au plafond réglementaire :
// disponible + en attente.
func (w *Wallet) SoldeTotalCentimes() int64 {
	return w.SoldeDisponibleCentimes + w.SoldeEnAttenteCentimes
}

// Crediter augmente directement le solde disponible (ex: cashback). Le
// plafond réglementaire du palier KYC ne doit jamais être dépassé.
func (w *Wallet) Crediter(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.Statut != StatutWalletActif {
		return ErrWalletGele
	}
	if w.SoldeTotalCentimes()+montantCentimes > w.PlafondSoldeCentimes {
		return ErrPlafondDepasse
	}
	w.SoldeDisponibleCentimes += montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// CrediterEnAttente crédite le solde en attente — utilisé quand
// l'agrégateur de paiement confirme une recharge mais impose un délai de
// retenue avant que les fonds ne soient utilisables (voir
// BasculerEnAttenteVersDisponible).
func (w *Wallet) CrediterEnAttente(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.Statut != StatutWalletActif {
		return ErrWalletGele
	}
	if w.SoldeTotalCentimes()+montantCentimes > w.PlafondSoldeCentimes {
		return ErrPlafondDepasse
	}
	w.SoldeEnAttenteCentimes += montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// BasculerEnAttenteVersDisponible libère un montant précédemment crédité
// en attente, une fois le délai de retenue de l'agrégateur écoulé.
func (w *Wallet) BasculerEnAttenteVersDisponible(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.SoldeEnAttenteCentimes < montantCentimes {
		return ErrMontantInvalide
	}
	w.SoldeEnAttenteCentimes -= montantCentimes
	w.SoldeDisponibleCentimes += montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Debiter diminue le solde disponible du wallet. Le solde en attente
// n'est jamais utilisable pour un débit.
func (w *Wallet) Debiter(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.Statut != StatutWalletActif {
		return ErrWalletGele
	}
	if w.SoldeDisponibleCentimes < montantCentimes {
		return ErrSoldeInsuffisant
	}
	w.SoldeDisponibleCentimes -= montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}
