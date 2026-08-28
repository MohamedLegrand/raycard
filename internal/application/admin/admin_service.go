// Package admin implémente les use cases (ports/input/admin) back-office
// transverses aux modules (utilisateurs, audit). Il ne connaît aucun
// détail de transport (HTTP) ni d'infrastructure concrète (Postgres) :
// uniquement les interfaces de ports/output.
package admin

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

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

func (s *adminService) ChangerRoleUtilisateur(ctx context.Context, adminID, utilisateurID string, nouveauRole domaincommun.RoleUtilisateur) (*domaincommun.Utilisateur, error) {
	if adminID == utilisateurID {
		return nil, domaincommun.ErrAutoModificationRole
	}

	utilisateur, err := s.utilisateurs.FindByID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}

	ancienRole := utilisateur.Role
	if err := utilisateur.ChangerRole(nouveauRole); err != nil {
		return nil, err
	}
	if err := s.utilisateurs.UpdateRole(ctx, utilisateur); err != nil {
		return nil, err
	}

	entree, err := domaincommun.NouvelleEntreeAuditLog(
		adminID, "role_utilisateur_modifie", "utilisateur", utilisateurID,
		fmt.Sprintf(`{"ancien_role":%q,"nouveau_role":%q}`, ancienRole, nouveauRole),
	)
	if err == nil {
		// Best-effort : l'action a déjà réussi, un échec de traçabilité ne
		// doit pas la faire échouer rétroactivement pour l'appelant.
		_ = s.auditLog.Create(ctx, entree)
	}

	return utilisateur, nil
}

// CreerAdministrateur crée directement un compte admin ou super_admin —
// même vérification d'unicité email/téléphone que l'inscription cliente
// (voir kyc.kycService.Inscrire), mais sans wallet ni palier KYC à
// franchir (voir commun.NouvelAdministrateur).
func (s *adminService) CreerAdministrateur(ctx context.Context, adminID string, req inputadmin.CreerAdministrateurRequest) (*domaincommun.Utilisateur, error) {
	if _, err := s.utilisateurs.FindByEmail(ctx, req.Email); !errors.Is(err, domaincommun.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, domaincommun.ErrEmailDejaUtilise
		}
		return nil, fmt.Errorf("vérification email: %w", err)
	}
	if _, err := s.utilisateurs.FindByTelephone(ctx, req.Telephone); !errors.Is(err, domaincommun.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, domaincommun.ErrTelephoneDejaUtilise
		}
		return nil, fmt.Errorf("vérification téléphone: %w", err)
	}

	motDePasseHash, err := bcrypt.GenerateFromPassword([]byte(req.MotDePasse), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hachage mot de passe: %w", err)
	}

	admin, err := domaincommun.NouvelAdministrateur(req.Nom, req.Prenom, req.Email, req.Telephone, req.PaysCode, string(motDePasseHash), req.Role)
	if err != nil {
		return nil, err
	}

	if err := s.utilisateurs.Create(ctx, admin); err != nil {
		return nil, fmt.Errorf("création administrateur: %w", err)
	}

	entree, err := domaincommun.NouvelleEntreeAuditLog(
		adminID, "administrateur_cree", "utilisateur", admin.ID,
		fmt.Sprintf(`{"email":%q,"role":%q}`, admin.Email, admin.Role),
	)
	if err == nil {
		// Best-effort, même logique que ChangerRoleUtilisateur : le compte
		// est déjà créé, un échec de traçabilité ne doit pas le défaire.
		_ = s.auditLog.Create(ctx, entree)
	}

	return admin, nil
}
