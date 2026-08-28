package kyc

import (
	"context"
	"encoding/json"
	"fmt"

	"raycard/internal/core/domain/commun"
	domainkyc "raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
	outputcommun "raycard/internal/core/ports/output/commun"
	outputkyc "raycard/internal/core/ports/output/kyc"
)

type adminKycService struct {
	utilisateurs outputcommun.UtilisateurRepository
	dossiersKyc  outputkyc.DossierKycRepository
	documentsKyc outputkyc.DocumentKycRepository
	stockage     outputcommun.StockageFichier
	auditLog     outputcommun.AuditLogRepository
	txManager    outputcommun.TxManager
}

// NewAdminKycService construit l'implémentation de inputkyc.AdminKycUseCase.
func NewAdminKycService(
	utilisateurs outputcommun.UtilisateurRepository,
	dossiersKyc outputkyc.DossierKycRepository,
	documentsKyc outputkyc.DocumentKycRepository,
	stockage outputcommun.StockageFichier,
	auditLog outputcommun.AuditLogRepository,
	txManager outputcommun.TxManager,
) inputkyc.AdminKycUseCase {
	return &adminKycService{
		utilisateurs: utilisateurs,
		dossiersKyc:  dossiersKyc,
		documentsKyc: documentsKyc,
		stockage:     stockage,
		auditLog:     auditLog,
		txManager:    txManager,
	}
}

func (s *adminKycService) ListerDossiersEnAttente(ctx context.Context) ([]*domainkyc.DossierKyc, error) {
	return s.dossiersKyc.ListEnAttente(ctx)
}

func (s *adminKycService) ListerTousDossiers(ctx context.Context) ([]*domainkyc.DossierKyc, error) {
	return s.dossiersKyc.ListAll(ctx)
}

// ListerDocuments renvoie les documents téléversés pour une demande
// précise, pour aider l'administrateur à la traiter.
func (s *adminKycService) ListerDocuments(ctx context.Context, dossierKycID string) ([]*domainkyc.DocumentKyc, error) {
	return s.documentsKyc.ListByDossierKycID(ctx, dossierKycID)
}

// RecupererDocument renvoie le contenu brut d'un document, pour que
// l'administrateur puisse l'examiner visuellement — la revue manuelle
// ne peut pas se contenter du texte OCR seul.
func (s *adminKycService) RecupererDocument(ctx context.Context, documentID string) (*domainkyc.DocumentKyc, []byte, error) {
	document, err := s.documentsKyc.FindByID(ctx, documentID)
	if err != nil {
		return nil, nil, err
	}

	contenu, err := s.stockage.Lire(ctx, document.CheminFichier)
	if err != nil {
		return nil, nil, fmt.Errorf("lecture document: %w", err)
	}

	return document, contenu, nil
}

func (s *adminKycService) ApprouverDossier(ctx context.Context, adminID, dossierID string) error {
	dossier, err := s.dossiersKyc.FindByID(ctx, dossierID)
	if err != nil {
		return err
	}

	utilisateur, err := s.utilisateurs.FindByID(ctx, dossier.UtilisateurID)
	if err != nil {
		return fmt.Errorf("recherche utilisateur: %w", err)
	}

	if err := dossier.Approuver(adminID); err != nil {
		return err
	}
	if err := utilisateur.PasserAuTier2(); err != nil {
		return err
	}

	entree, err := commun.NouvelleEntreeAuditLog(adminID, "kyc_tier2_approuve", "utilisateur", utilisateur.ID, "")
	if err != nil {
		return err
	}

	// Le dossier, l'utilisateur et la trace d'audit doivent avancer
	// ensemble : c'est une action administrateur sensible sur plusieurs
	// tables (exigence ACID du cahier des charges).
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.dossiersKyc.Update(ctx, dossier); err != nil {
			return fmt.Errorf("mise à jour dossier kyc: %w", err)
		}
		if err := s.utilisateurs.UpdateStatutKyc(ctx, utilisateur); err != nil {
			return fmt.Errorf("mise à jour utilisateur: %w", err)
		}
		if err := s.auditLog.Create(ctx, entree); err != nil {
			return fmt.Errorf("écriture audit log: %w", err)
		}
		return nil
	})
}

func (s *adminKycService) RejeterDossier(ctx context.Context, adminID, dossierID, motif string) error {
	dossier, err := s.dossiersKyc.FindByID(ctx, dossierID)
	if err != nil {
		return err
	}

	if err := dossier.Rejeter(adminID, motif); err != nil {
		return err
	}

	// Le detail est du JSON valide (colonne audit_log.details_json en
	// jsonb côté Postgres) : le motif brut ne suffit pas.
	details, err := json.Marshal(map[string]string{"motif": motif})
	if err != nil {
		return fmt.Errorf("sérialisation motif: %w", err)
	}

	entree, err := commun.NouvelleEntreeAuditLog(adminID, "kyc_tier2_rejete", "utilisateur", dossier.UtilisateurID, string(details))
	if err != nil {
		return err
	}

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.dossiersKyc.Update(ctx, dossier); err != nil {
			return fmt.Errorf("mise à jour dossier kyc: %w", err)
		}
		if err := s.auditLog.Create(ctx, entree); err != nil {
			return fmt.Errorf("écriture audit log: %w", err)
		}
		return nil
	})
}
