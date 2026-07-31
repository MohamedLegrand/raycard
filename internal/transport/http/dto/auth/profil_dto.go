package auth

import (
	"raycard/internal/core/domain/commun"
	"raycard/internal/core/ports/input/auth"
)

// UtilisateurDTO expose les informations de profil de l'utilisateur
// authentifié. Ne contient jamais le mot de passe (même haché).
type UtilisateurDTO struct {
	ID          string `json:"id" example:"3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d"`
	Nom         string `json:"nom" example:"Koné"`
	Prenom      string `json:"prenom" example:"Awa"`
	Email       string `json:"email" example:"awa.kone@example.com"`
	Telephone   string `json:"telephone" example:"+2250700000000"`
	PaysCode    string `json:"pays_code" example:"CI"`
	PhotoProfil string `json:"photo_profil,omitempty"`
	KycTier     int    `json:"kyc_tier" example:"1"`
	KycStatut   string `json:"kyc_statut" example:"verifie"`
}

func FromUtilisateur(u *commun.Utilisateur) UtilisateurDTO {
	return UtilisateurDTO{
		ID:          u.ID,
		Nom:         u.Nom,
		Prenom:      u.Prenom,
		Email:       u.Email,
		Telephone:   u.Telephone,
		PaysCode:    u.PaysCode,
		PhotoProfil: u.PhotoProfil,
		KycTier:     int(u.KycTier),
		KycStatut:   string(u.KycStatut),
	}
}

type ModifierProfilRequestDTO struct {
	Nom    string `json:"nom" validate:"required,min=2,max=100" example:"Koné"`
	Prenom string `json:"prenom" validate:"required,min=2,max=100" example:"Awa"`
}

func (d ModifierProfilRequestDTO) ToUseCaseRequest() auth.ModifierProfilRequest {
	return auth.ModifierProfilRequest{Nom: d.Nom, Prenom: d.Prenom}
}

// ChangerMotDePasseRequestDTO : contrairement à ReinitialiserRequestDTO
// (mot de passe oublié, prouvé par un code email), ce flux nécessite le
// mot de passe actuel — l'utilisateur est déjà authentifié.
type ChangerMotDePasseRequestDTO struct {
	MotDePasseActuel  string `json:"mot_de_passe_actuel" validate:"required" example:"ancienmotdepasse123"`
	NouveauMotDePasse string `json:"nouveau_mot_de_passe" validate:"required,min=8" example:"nouveaumotdepasse456"`
}

type DemanderChangementEmailRequestDTO struct {
	NouvelEmail string `json:"nouvel_email" validate:"required,email" example:"nouveau.email@example.com"`
}

// ConfirmerChangementEmailRequestDTO : le code est envoyé au NOUVEL
// email, jamais à l'ancien.
type ConfirmerChangementEmailRequestDTO struct {
	Code string `json:"code" validate:"required,len=6,numeric" example:"042951"`
}
