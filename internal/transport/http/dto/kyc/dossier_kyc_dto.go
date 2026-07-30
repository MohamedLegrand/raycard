package kyc

import (
	"time"

	"raycard/internal/core/domain/kyc"
)

type DossierKycDTO struct {
	ID            string     `json:"id"`
	UtilisateurID string     `json:"utilisateur_id"`
	TierDemande   int        `json:"tier_demande"`
	Statut        string     `json:"statut"`
	MotifRejet    string     `json:"motif_rejet,omitempty"`
	AdminID       string     `json:"admin_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	TraiteAt      *time.Time `json:"traite_at,omitempty"`
}

func FromDossierKyc(d *kyc.DossierKyc) DossierKycDTO {
	return DossierKycDTO{
		ID:            d.ID,
		UtilisateurID: d.UtilisateurID,
		TierDemande:   int(d.TierDemande),
		Statut:        string(d.Statut),
		MotifRejet:    d.MotifRejet,
		AdminID:       d.AdminID,
		CreatedAt:     d.CreatedAt,
		TraiteAt:      d.TraiteAt,
	}
}

func FromDossiersKyc(dossiers []*kyc.DossierKyc) []DossierKycDTO {
	resultat := make([]DossierKycDTO, 0, len(dossiers))
	for _, d := range dossiers {
		resultat = append(resultat, FromDossierKyc(d))
	}
	return resultat
}

type RejeterDossierRequestDTO struct {
	Motif string `json:"motif" validate:"required,min=3" example:"pièce d'identité illisible"`
}
