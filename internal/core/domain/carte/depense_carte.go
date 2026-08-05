package carte

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// DepenseCarte trace une dépense détectée sur une carte par
// rapprochement de solde (voir Carte.MettreAJourSolde) — jamais une
// autorisation en temps réel, qui n'existe pas côté agrégateur
// aujourd'hui. DetectedAt est le moment de la détection, pas celui de la
// dépense elle-même (inconnu).
type DepenseCarte struct {
	ID                 string
	CarteID            string
	MontantCentimes    int64
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
		SoldeAvantCentimes: soldeAvantCentimes,
		SoldeApresCentimes: soldeApresCentimes,
		DetectedAt:         time.Now().UTC(),
	}, nil
}
