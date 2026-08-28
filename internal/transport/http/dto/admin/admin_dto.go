// Package admin contient les structures JSON exposées par l'API HTTP
// pour le back-office (utilisateurs, audit), et leur conversion depuis
// les types de la couche application et du domaine.
package admin

import (
	"time"

	"raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	"raycard/internal/transport/http/dto/carte"
	"raycard/internal/transport/http/dto/wallet"
)

type UtilisateurDTO struct {
	ID        string    `json:"id" example:"3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d"`
	Nom       string    `json:"nom" example:"Koné"`
	Prenom    string    `json:"prenom" example:"Awa"`
	Email     string    `json:"email" example:"awa.kone@example.com"`
	Telephone string    `json:"telephone" example:"+2250700000000"`
	PaysCode  string    `json:"pays_code" example:"CI"`
	Role      string    `json:"role" example:"utilisateur"`
	KycTier   int       `json:"kyc_tier" example:"1"`
	KycStatut string    `json:"kyc_statut" example:"verifie"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-05T18:30:00Z"`
}

func FromUtilisateur(u *commun.Utilisateur) UtilisateurDTO {
	return UtilisateurDTO{
		ID:        u.ID,
		Nom:       u.Nom,
		Prenom:    u.Prenom,
		Email:     u.Email,
		Telephone: u.Telephone,
		PaysCode:  u.PaysCode,
		Role:      string(u.Role),
		KycTier:   int(u.KycTier),
		KycStatut: string(u.KycStatut),
		CreatedAt: u.CreatedAt,
	}
}

func FromUtilisateurs(utilisateurs []*commun.Utilisateur) []UtilisateurDTO {
	dtos := make([]UtilisateurDTO, 0, len(utilisateurs))
	for _, u := range utilisateurs {
		dtos = append(dtos, FromUtilisateur(u))
	}
	return dtos
}

// UtilisateurDetailDTO est la fiche complète d'un utilisateur pour la vue
// "détail" du back-office. Wallet est absent (omitempty) pour un compte
// administrateur, qui n'en a pas (voir commun.NouvelAdministrateur).
type UtilisateurDetailDTO struct {
	Utilisateur UtilisateurDTO    `json:"utilisateur"`
	Wallet      *wallet.WalletDTO `json:"wallet,omitempty"`
	Cartes      []carte.CarteDTO  `json:"cartes"`
}

func FromUtilisateurDetail(d *inputadmin.UtilisateurDetail) UtilisateurDetailDTO {
	dto := UtilisateurDetailDTO{
		Utilisateur: FromUtilisateur(d.Utilisateur),
		Cartes:      carte.FromCartes(d.Cartes),
	}
	if d.Wallet != nil {
		w := wallet.FromWallet(d.Wallet)
		dto.Wallet = &w
	}
	return dto
}

// AuditLogDTO trace une action administrateur sensible (voir
// commun.AuditLog).
type AuditLogDTO struct {
	ID          string    `json:"id" example:"9c1a2b3d-4e5f-6a7b-8c9d-0e1f2a3b4c5d"`
	AdminID     string    `json:"admin_id" example:"3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d"`
	Action      string    `json:"action" example:"carte_gelee_admin"`
	CibleType   string    `json:"cible_type" example:"carte"`
	CibleID     string    `json:"cible_id" example:"7b8c9d0e-1f2a-4b3c-9d4e-5f6a7b8c9d0e"`
	DetailsJSON string    `json:"details_json,omitempty" example:"{\"motif\":\"document illisible\"}"`
	CreatedAt   time.Time `json:"created_at" example:"2026-08-05T18:30:00Z"`
}

func FromAuditLog(a *commun.AuditLog) AuditLogDTO {
	return AuditLogDTO{
		ID:          a.ID,
		AdminID:     a.AdminID,
		Action:      a.Action,
		CibleType:   a.CibleType,
		CibleID:     a.CibleID,
		DetailsJSON: a.DetailsJSON,
		CreatedAt:   a.CreatedAt,
	}
}

// ChangerRoleRequestDTO : role doit être l'une des trois valeurs
// exactes du domaine (voir commun.RoleUtilisateur) — "utilisateur"
// rétrograde un admin en simple client.
type ChangerRoleRequestDTO struct {
	Role string `json:"role" validate:"required,oneof=utilisateur admin super_admin" example:"admin"`
}

// CreerAdministrateurRequestDTO : contrairement à ChangerRoleRequestDTO,
// role n'accepte pas "utilisateur" — cet endpoint crée un compte
// d'emblée admin, jamais un compte client (voir
// commun.NouvelAdministrateur).
type CreerAdministrateurRequestDTO struct {
	Nom        string `json:"nom" validate:"required" example:"Koné"`
	Prenom     string `json:"prenom" validate:"required" example:"Awa"`
	Email      string `json:"email" validate:"required,email" example:"awa.kone@raycard.app"`
	Telephone  string `json:"telephone" validate:"required" example:"+2250700000000"`
	PaysCode   string `json:"pays_code" validate:"required,len=2" example:"CI"`
	MotDePasse string `json:"mot_de_passe" validate:"required,min=8" example:"motdepasse123"`
	Role       string `json:"role" validate:"required,oneof=admin super_admin" example:"admin"`
}

func FromAuditLogs(entrees []*commun.AuditLog) []AuditLogDTO {
	dtos := make([]AuditLogDTO, 0, len(entrees))
	for _, e := range entrees {
		dtos = append(dtos, FromAuditLog(e))
	}
	return dtos
}
