// Package admin regroupe les interfaces (ports entrants) exposées par la
// couche application au transport pour les actions back-office
// transverses aux modules (utilisateurs, audit). Les revues KYC (voir
// kyc.AdminKycUseCase), les wallets/transactions (voir
// wallet.AdminWalletUseCase) et les cartes (voir carte.AdminCarteUseCase)
// ont leur propre use case, dans leur module respectif — regroupés ici
// uniquement ce qui n'appartient à aucun des modules existants.
package admin

import (
	"context"

	domaincarte "raycard/internal/core/domain/carte"
	domaincommun "raycard/internal/core/domain/commun"
	outputcommun "raycard/internal/core/ports/output/commun"
)

// AdminUseCase orchestre les actions back-office transverses aux modules,
// jamais accessibles à un client normal (voir middleware.RequireAdmin).
type AdminUseCase interface {
	ListerUtilisateurs(ctx context.Context, filtre outputcommun.FiltreUtilisateurs) ([]*domaincommun.Utilisateur, error)

	// ObtenirUtilisateur agrège la fiche complète d'un utilisateur —
	// profil, wallet et cartes — pour la vue "détail" du back-office.
	ObtenirUtilisateur(ctx context.Context, utilisateurID string) (*UtilisateurDetail, error)

	ListerAuditLogs(ctx context.Context, filtre outputcommun.FiltreAuditLog) ([]*domaincommun.AuditLog, error)

	// ChangerRoleUtilisateur élève ou rétrograde un compte (client,
	// admin, super_admin) — réservé au super_admin (voir
	// middleware.RequireSuperAdmin). adminID != utilisateurID est
	// vérifié ici : un super_admin ne peut jamais changer son propre
	// rôle, pour éviter un verrouillage accidentel.
	ChangerRoleUtilisateur(ctx context.Context, adminID, utilisateurID string, nouveauRole domaincommun.RoleUtilisateur) (*domaincommun.Utilisateur, error)

	// CreerAdministrateur crée directement un compte admin ou
	// super_admin — réservé au super_admin (voir
	// middleware.RequireSuperAdmin). Différent de ChangerRoleUtilisateur :
	// ne suppose aucun compte client préexistant, pour on-boarder un
	// membre de l'équipe qui n'a jamais utilisé l'app mobile. adminID
	// sert uniquement à tracer l'action dans l'audit log.
	CreerAdministrateur(ctx context.Context, adminID string, req CreerAdministrateurRequest) (*domaincommun.Utilisateur, error)
}

// CreerAdministrateurRequest transporte les données nécessaires à la
// création directe d'un compte admin (voir AdminUseCase.CreerAdministrateur).
type CreerAdministrateurRequest struct {
	Nom        string
	Prenom     string
	Email      string
	Telephone  string
	PaysCode   string
	MotDePasse string
	Role       domaincommun.RoleUtilisateur
}

// UtilisateurDetail agrège le profil, le wallet et les cartes d'un
// utilisateur. Wallet est nil si l'utilisateur n'en a pas encore (ex: un
// compte administrateur, voir commun.NouvelAdministrateur, créé sans
// wallet).
type UtilisateurDetail struct {
	Utilisateur *domaincommun.Utilisateur
	Wallet      *domaincommun.Wallet
	Cartes      []*domaincarte.Carte
}
