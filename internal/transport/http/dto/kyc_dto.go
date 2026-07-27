// Package dto contient les structures JSON exposées par l'API HTTP et
// leur conversion vers/depuis les types de la couche application
// (input.InscriptionRequest) et du domaine.
package dto

import "raycard/internal/core/ports/input"

type InscriptionRequestDTO struct {
	Nom        string `json:"nom" validate:"required,min=2,max=100" example:"Koné"`
	Prenom     string `json:"prenom" validate:"required,min=2,max=100" example:"Awa"`
	Email      string `json:"email" validate:"required,email" example:"awa.kone@example.com"`
	Telephone  string `json:"telephone" validate:"required,e164" example:"+2250700000000"`
	PaysCode   string `json:"pays_code" validate:"required,len=2" example:"CI"`
	MotDePasse string `json:"mot_de_passe" validate:"required,min=8" example:"motdepasse123"`
}

func (d InscriptionRequestDTO) ToUseCaseRequest() input.InscriptionRequest {
	return input.InscriptionRequest{
		Nom:        d.Nom,
		Prenom:     d.Prenom,
		Email:      d.Email,
		Telephone:  d.Telephone,
		PaysCode:   d.PaysCode,
		MotDePasse: d.MotDePasse,
	}
}

type UtilisateurDTO struct {
	ID        string `json:"id" example:"3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d"`
	Nom       string `json:"nom" example:"Koné"`
	Prenom    string `json:"prenom" example:"Awa"`
	Email     string `json:"email" example:"awa.kone@example.com"`
	Telephone string `json:"telephone" example:"+2250700000000"`
	PaysCode  string `json:"pays_code" example:"CI"`
	KycTier   int    `json:"kyc_tier" example:"1"`
	KycStatut string `json:"kyc_statut" example:"verifie"`
}

type WalletDTO struct {
	ID            string `json:"id" example:"7b8c9d0e-1f2a-4b3c-9d4e-5f6a7b8c9d0e"`
	Devise        string `json:"devise" example:"XOF"`
	SoldeCentimes int64  `json:"solde_centimes" example:"0"`
	Statut        string `json:"statut" example:"actif"`
}

type InscriptionResponseDTO struct {
	Utilisateur UtilisateurDTO `json:"utilisateur"`
	Wallet      WalletDTO      `json:"wallet"`
}

func FromInscriptionResultat(res *input.InscriptionResultat) InscriptionResponseDTO {
	return InscriptionResponseDTO{
		Utilisateur: UtilisateurDTO{
			ID:        res.Utilisateur.ID,
			Nom:       res.Utilisateur.Nom,
			Prenom:    res.Utilisateur.Prenom,
			Email:     res.Utilisateur.Email,
			Telephone: res.Utilisateur.Telephone,
			PaysCode:  res.Utilisateur.PaysCode,
			KycTier:   int(res.Utilisateur.KycTier),
			KycStatut: string(res.Utilisateur.KycStatut),
		},
		Wallet: WalletDTO{
			ID:            res.Wallet.ID,
			Devise:        res.Wallet.Devise,
			SoldeCentimes: res.Wallet.SoldeCentimes,
			Statut:        string(res.Wallet.Statut),
		},
	}
}
