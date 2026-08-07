// Package wallet contient les structures JSON exposées par l'API HTTP
// pour le wallet et ses transactions, et leur conversion vers/depuis les
// types de la couche application et du domaine.
package wallet

import (
	"time"

	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/wallet"
	inputwallet "raycard/internal/core/ports/input/wallet"
)

type WalletDTO struct {
	ID                      string `json:"id" example:"7b8c9d0e-1f2a-4b3c-9d4e-5f6a7b8c9d0e"`
	Devise                  string `json:"devise" example:"XOF"`
	SoldeDisponibleCentimes int64  `json:"solde_disponible_centimes" example:"492500"`
	SoldeEnAttenteCentimes  int64  `json:"solde_en_attente_centimes" example:"0"`
	Statut                  string `json:"statut" example:"actif"`
}

func FromWallet(w *commun.Wallet) WalletDTO {
	return WalletDTO{
		ID:                      w.ID,
		Devise:                  w.Devise,
		SoldeDisponibleCentimes: w.SoldeDisponibleCentimes,
		SoldeEnAttenteCentimes:  w.SoldeEnAttenteCentimes,
		Statut:                  string(w.Statut),
	}
}

// InitierRechargeRequestDTO : Operateur attend un code HR-Skills Pay
// (ex: "ORANGE", "MTN", "WAVE") — voir la liste complète dans la doc de
// l'agrégateur.
type InitierRechargeRequestDTO struct {
	Operateur       string `json:"operateur" validate:"required" example:"ORANGE"`
	Telephone       string `json:"telephone" validate:"required,e164" example:"+2250700000000"`
	MontantCentimes int64  `json:"montant_centimes" validate:"required,gt=0" example:"5000"`
}

func (d InitierRechargeRequestDTO) ToUseCaseRequest() inputwallet.InitierRechargeRequest {
	return inputwallet.InitierRechargeRequest{
		Operateur:       d.Operateur,
		Telephone:       d.Telephone,
		MontantCentimes: d.MontantCentimes,
	}
}

// InitierRetraitRequestDTO : Operateur attend un code HR-Skills Pay (ex:
// "ORANGE", "MTN", "WAVE") — voir la liste complète dans la doc de
// l'agrégateur.
type InitierRetraitRequestDTO struct {
	Operateur       string `json:"operateur" validate:"required" example:"ORANGE"`
	Telephone       string `json:"telephone" validate:"required,e164" example:"+2250700000000"`
	MontantCentimes int64  `json:"montant_centimes" validate:"required,gt=0" example:"5000"`
}

func (d InitierRetraitRequestDTO) ToUseCaseRequest() inputwallet.InitierRetraitRequest {
	return inputwallet.InitierRetraitRequest{
		Operateur:       d.Operateur,
		Telephone:       d.Telephone,
		MontantCentimes: d.MontantCentimes,
	}
}

type TransactionDTO struct {
	ID              string    `json:"id" example:"9c1a2b3d-4e5f-6a7b-8c9d-0e1f2a3b4c5d"`
	Type            string    `json:"type" example:"recharge"`
	Statut          string    `json:"statut" example:"envoyee"`
	MontantCentimes int64     `json:"montant_centimes" example:"5000"`
	FraisCentimes   int64     `json:"frais_centimes" example:"75"`
	Devise          string    `json:"devise" example:"XOF"`
	Operateur       string    `json:"operateur,omitempty" example:"ORANGE"`
	CreatedAt       time.Time `json:"created_at" example:"2026-08-05T18:30:00Z"`
}

func FromTransaction(t *wallet.Transaction) TransactionDTO {
	return TransactionDTO{
		ID:              t.ID,
		Type:            string(t.Type),
		Statut:          string(t.Statut),
		MontantCentimes: t.MontantCentimes,
		FraisCentimes:   t.FraisCentimes,
		Devise:          t.Devise,
		Operateur:       t.Operateur,
		CreatedAt:       t.CreatedAt,
	}
}

func FromTransactions(transactions []*wallet.Transaction) []TransactionDTO {
	dtos := make([]TransactionDTO, 0, len(transactions))
	for _, t := range transactions {
		dtos = append(dtos, FromTransaction(t))
	}
	return dtos
}
