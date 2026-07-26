// Package dto contient les structures JSON exposées par l'API HTTP et
// leur conversion vers/depuis les types de la couche application
// (input.InscriptionRequest) et du domaine.
package dto

import "raycard/internal/core/ports/input"

type InscriptionRequestDTO struct {
	Nom        string `json:"nom" validate:"required,min=2,max=100"`
	Prenom     string `json:"prenom" validate:"required,min=2,max=100"`
	Email      string `json:"email" validate:"required,email"`
	Telephone  string `json:"telephone" validate:"required,e164"`
	PaysCode   string `json:"pays_code" validate:"required,len=2"`
	MotDePasse string `json:"mot_de_passe" validate:"required,min=8"`
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
	ID        string `json:"id"`
	Nom       string `json:"nom"`
	Prenom    string `json:"prenom"`
	Email     string `json:"email"`
	Telephone string `json:"telephone"`
	PaysCode  string `json:"pays_code"`
	KycTier   int    `json:"kyc_tier"`
	KycStatut string `json:"kyc_statut"`
}

type WalletDTO struct {
	ID            string `json:"id"`
	Devise        string `json:"devise"`
	SoldeCentimes int64  `json:"solde_centimes"`
	Statut        string `json:"statut"`
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
