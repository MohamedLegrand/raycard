// Package admin implémente les use cases (ports/input/admin) back-office
// transverses aux modules (utilisateurs, audit). Il ne connaît aucun
// détail de transport (HTTP) ni d'infrastructure concrète (Postgres) :
// uniquement les interfaces de ports/output.
package admin

import (
	"context"
	"errors"

	domaincommun "raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	outputcarte "raycard/internal/core/ports/output/carte"
	outputcommun "raycard/internal/core/ports/output/commun"
)

type adminService struct {
	utilisateurs outputcommun.UtilisateurRepository
	wallets      outputcommun.WalletRepository
	cartes       outputcarte.CarteRepository
	auditLog     outputcommun.AuditLogRepository
}

// NewAdminService construit l'implémentation de inputadmin.AdminUseCase.
func NewAdminService(
	utilisateurs outputcommun.UtilisateurRepository,
	wallets outputcommun.WalletRepository,
	cartes outputcarte.CarteRepository,
	auditLog outputcommun.AuditLogRepository,
) inputadmin.AdminUseCase {
	return &adminService{utilisateurs: utilisateurs, wallets: wallets, cartes: cartes, auditLog: auditLog}
}

func (s *adminService) ListerUtilisateurs(ctx context.Context, filtre outputcommun.FiltreUtilisateurs) ([]*domaincommun.Utilisateur, error) {
	return s.utilisateurs.ListAll(ctx, filtre)
}

func (s *adminService) ObtenirUtilisateur(ctx context.Context, utilisateurID string) (*inputadmin.UtilisateurDetail, error) {
	utilisateur, err := s.utilisateurs.FindByID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}

	detail := &inputadmin.UtilisateurDetail{Utilisateur: utilisateur}

	// Un compte administrateur n'a pas de wallet (voir
	// commun.NouvelAdministrateur) : son absence n'est pas une erreur ici.
	w, err := s.wallets.FindByUtilisateurID(ctx, utilisateurID)
	if err != nil && !errors.Is(err, domaincommun.ErrWalletIntrouvable) {
		return nil, err
	}
	if err == nil {
		detail.Wallet = w
	}

	cartes, err := s.cartes.ListByUtilisateurID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	detail.Cartes = cartes

	return detail, nil
}

func (s *adminService) ListerAuditLogs(ctx context.Context, filtre outputcommun.FiltreAuditLog) ([]*domaincommun.AuditLog, error) {
	return s.auditLog.List(ctx, filtre)
}
