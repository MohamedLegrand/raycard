package carte

import (
	"math"
	"time"

	"raycard/internal/core/domain/commun"
)

// tauxCashback est le pourcentage crédité automatiquement sur le wallet
// après chaque dépense carte détectée : une incitation marketing, pas un
// remboursement de frais réels. 0,02% reste négligeable par dépense (pas
// de plafond dédié pour l'instant).
const tauxCashback = 0.0002

// DepenseCarte trace une dépense détectée sur une carte par
// rapprochement de solde (voir Carte.MettreAJourSolde) — jamais une
// autorisation en temps réel, qui n'existe pas côté agrégateur
// aujourd'hui. DetectedAt est le moment de la détection, pas celui de la
// dépense elle-même (inconnu).
type DepenseCarte struct {
	ID                 string
	CarteID            string
	MontantCentimes    int64
	CashbackCentimes   int64
	SoldeAvantCentimes int64
	SoldeApresCentimes int64
	DetectedAt         time.Time
}

func NouvelleDepenseCarte(carteID string, montantCentimes, soldeAvantCentimes, soldeApresCentimes int64) (*DepenseCarte, error) {
	if carteID == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if montantCentimes <= 0 {
		return nil, commun.ErrMontantInvalide
	}

	return &DepenseCarte{
		ID:                 commun.NewID(),
		CarteID:            carteID,
		MontantCentimes:    montantCentimes,
		CashbackCentimes:   calculerCashback(montantCentimes),
		SoldeAvantCentimes: soldeAvantCentimes,
		SoldeApresCentimes: soldeApresCentimes,
		DetectedAt:         time.Now().UTC(),
	}, nil
}

// calculerCashback arrondit au centime le plus proche ; peut retourner 0
// sur les très petits montants, auquel cas aucun crédit n'est déclenché.
func calculerCashback(montantDepenseCentimes int64) int64 {
	return int64(math.Round(float64(montantDepenseCentimes) * tauxCashback))
}
