// Package admin fournit un fake de inputadmin.AdminUseCase pour les
// tests HTTP de la couche transport — même principe que
// test/transport/http/carte.
package admin

import (
	"context"

	domaincommun "raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	outputcommun "raycard/internal/core/ports/output/commun"
)

type AdminUseCaseFake struct {
	ListerUtilisateursResultat []*domaincommun.Utilisateur
	ListerUtilisateursErr      error
	DernierFiltreUtilisateurs  outputcommun.FiltreUtilisateurs

	ObtenirUtilisateurResultat *inputadmin.UtilisateurDetail
	ObtenirUtilisateurErr      error

	ListerAuditLogsResultat []*domaincommun.AuditLog
	ListerAuditLogsErr      error
	DernierFiltreAuditLog   outputcommun.FiltreAuditLog

	ChangerRoleUtilisateurResultat *domaincommun.Utilisateur
	ChangerRoleUtilisateurErr      error
	DernierRoleDemande             domaincommun.RoleUtilisateur

	CreerAdministrateurResultat *domaincommun.Utilisateur
	CreerAdministrateurErr      error
	DerniereReqCreerAdmin       inputadmin.CreerAdministrateurRequest

	DernierAdminID       string
	DernierUtilisateurID string
}

func (f *AdminUseCaseFake) ListerUtilisateurs(_ context.Context, filtre outputcommun.FiltreUtilisateurs) ([]*domaincommun.Utilisateur, error) {
	f.DernierFiltreUtilisateurs = filtre
	return f.ListerUtilisateursResultat, f.ListerUtilisateursErr
}

func (f *AdminUseCaseFake) ObtenirUtilisateur(_ context.Context, utilisateurID string) (*inputadmin.UtilisateurDetail, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirUtilisateurResultat, f.ObtenirUtilisateurErr
}

func (f *AdminUseCaseFake) ListerAuditLogs(_ context.Context, filtre outputcommun.FiltreAuditLog) ([]*domaincommun.AuditLog, error) {
	f.DernierFiltreAuditLog = filtre
	return f.ListerAuditLogsResultat, f.ListerAuditLogsErr
}

func (f *AdminUseCaseFake) ChangerRoleUtilisateur(_ context.Context, adminID, utilisateurID string, nouveauRole domaincommun.RoleUtilisateur) (*domaincommun.Utilisateur, error) {
	f.DernierAdminID, f.DernierUtilisateurID, f.DernierRoleDemande = adminID, utilisateurID, nouveauRole
	return f.ChangerRoleUtilisateurResultat, f.ChangerRoleUtilisateurErr
}

func (f *AdminUseCaseFake) CreerAdministrateur(_ context.Context, adminID string, req inputadmin.CreerAdministrateurRequest) (*domaincommun.Utilisateur, error) {
	f.DernierAdminID = adminID
	f.DerniereReqCreerAdmin = req
	return f.CreerAdministrateurResultat, f.CreerAdministrateurErr
}
