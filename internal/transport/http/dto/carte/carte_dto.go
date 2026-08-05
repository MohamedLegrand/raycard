// Package carte contient les structures JSON exposées par l'API HTTP
// pour les cartes virtuelles, et leur conversion vers/depuis les types de
// la couche application et du domaine.
package carte

import (
	"time"

	"raycard/internal/core/domain/carte"
	inputcarte "raycard/internal/core/ports/input/carte"
)

type CreerCarteRequestDTO struct {
	Label           string `json:"label" validate:"required,min=2,max=100" example:"Carte courses"`
	MontantCentimes int64  `json:"montant_centimes" validate:"required,gt=0" example:"10000"`
}

func (d CreerCarteRequestDTO) ToUseCaseRequest() inputcarte.CreerCarteRequest {
	return inputcarte.CreerCarteRequest{Label: d.Label, MontantCentimes: d.MontantCentimes}
}

type RechargerCarteRequestDTO struct {
	MontantCentimes int64 `json:"montant_centimes" validate:"required,gt=0" example:"5000"`
}

func (d RechargerCarteRequestDTO) ToUseCaseRequest() inputcarte.RechargerCarteRequest {
	return inputcarte.RechargerCarteRequest{MontantCentimes: d.MontantCentimes}
}

// CarteDTO n'expose jamais le PAN ni le CVV : le SDK de l'agrégateur ne
// les fournit pas au-delà de la création, et le principe général de
// RAYCARD est de ne jamais les persister côté serveur.
type CarteDTO struct {
	ID                    string `json:"id" example:"3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d"`
	Label                 string `json:"label" example:"Carte courses"`
	Devise                string `json:"devise" example:"XOF"`
	MontantChargeCentimes int64  `json:"montant_charge_centimes" example:"10000"`
	// SoldeCentimes est le dernier solde connu, mis à jour périodiquement
	// (pas en temps réel — voir carte.Carte.SoldeCentimes).
	SoldeCentimes int64  `json:"solde_centimes" example:"7500"`
	Statut        string `json:"statut" example:"active"`
}

func FromCarte(c *carte.Carte) CarteDTO {
	return CarteDTO{
		ID:                    c.ID,
		Label:                 c.Label,
		Devise:                c.Devise,
		MontantChargeCentimes: c.MontantChargeCentimes,
		SoldeCentimes:         c.SoldeCentimes,
		Statut:                string(c.Statut),
	}
}

func FromCartes(cartes []*carte.Carte) []CarteDTO {
	dtos := make([]CarteDTO, 0, len(cartes))
	for _, c := range cartes {
		dtos = append(dtos, FromCarte(c))
	}
	return dtos
}

// DepenseCarteDTO trace une dépense détectée par rapprochement de solde
// — jamais une autorisation en temps réel (voir carte.DepenseCarte).
type DepenseCarteDTO struct {
	ID              string    `json:"id" example:"1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"`
	MontantCentimes int64     `json:"montant_centimes" example:"2500"`
	DetectedAt      time.Time `json:"detected_at" example:"2026-08-05T18:30:00Z"`
}

func FromDepense(d *carte.DepenseCarte) DepenseCarteDTO {
	return DepenseCarteDTO{
		ID:              d.ID,
		MontantCentimes: d.MontantCentimes,
		DetectedAt:      d.DetectedAt,
	}
}

func FromDepenses(depenses []*carte.DepenseCarte) []DepenseCarteDTO {
	dtos := make([]DepenseCarteDTO, 0, len(depenses))
	for _, d := range depenses {
		dtos = append(dtos, FromDepense(d))
	}
	return dtos
}
