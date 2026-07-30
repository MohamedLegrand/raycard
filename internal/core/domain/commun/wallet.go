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
type Wallet struct {
	ID                   string
	UtilisateurID        string
	PaysCode             string
	Devise               string // ISO 4217, ex: "XOF"
	SoldeCentimes        int64
	PlafondSoldeCentimes int64
	Statut               StatutWallet
	CreatedAt            time.Time
	UpdatedAt            time.Time
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
		ID:                   NewID(),
		UtilisateurID:        utilisateurID,
		PaysCode:             paysCode,
		Devise:               devise,
		SoldeCentimes:        0,
		PlafondSoldeCentimes: plafondSoldeCentimes,
		Statut:               StatutWalletActif,
		CreatedAt:            maintenant,
		UpdatedAt:            maintenant,
	}, nil
}

// Crediter augmente le solde du wallet. Le plafond réglementaire du
// palier KYC ne doit jamais être dépassé.
func (w *Wallet) Crediter(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.Statut != StatutWalletActif {
		return ErrWalletGele
	}
	if w.SoldeCentimes+montantCentimes > w.PlafondSoldeCentimes {
		return ErrPlafondDepasse
	}
	w.SoldeCentimes += montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Debiter diminue le solde du wallet.
func (w *Wallet) Debiter(montantCentimes int64) error {
	if montantCentimes <= 0 {
		return ErrMontantInvalide
	}
	if w.Statut != StatutWalletActif {
		return ErrWalletGele
	}
	if w.SoldeCentimes < montantCentimes {
		return ErrSoldeInsuffisant
	}
	w.SoldeCentimes -= montantCentimes
	w.UpdatedAt = time.Now().UTC()
	return nil
}
