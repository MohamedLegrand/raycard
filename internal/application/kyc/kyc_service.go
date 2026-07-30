// Package kyc implémente les use cases (ports/input/kyc) en orchestrant
// le domaine et les ports/output. Il ne connaît aucun détail de
// transport (HTTP) ni d'infrastructure concrète (Postgres, partenaires) :
// uniquement leurs interfaces.
package kyc

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"raycard/internal/core/domain/commun"
	domainkyc "raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
	outputcommun "raycard/internal/core/ports/output/commun"
	outputkyc "raycard/internal/core/ports/output/kyc"
)

type kycService struct {
	utilisateurs outputcommun.UtilisateurRepository
	wallets      outputcommun.WalletRepository
	reglesKyc    outputcommun.ReglesKycRepository
	dossiersKyc  outputkyc.DossierKycRepository
	txManager    outputcommun.TxManager
}

// NewKycService construit l'implémentation de inputkyc.KycUseCase.
func NewKycService(
	utilisateurs outputcommun.UtilisateurRepository,
	wallets outputcommun.WalletRepository,
	reglesKyc outputcommun.ReglesKycRepository,
	dossiersKyc outputkyc.DossierKycRepository,
	txManager outputcommun.TxManager,
) inputkyc.KycUseCase {
	return &kycService{
		utilisateurs: utilisateurs,
		wallets:      wallets,
		reglesKyc:    reglesKyc,
		dossiersKyc:  dossiersKyc,
		txManager:    txManager,
	}
}

func (s *kycService) Inscrire(ctx context.Context, req inputkyc.InscriptionRequest) (*inputkyc.InscriptionResultat, error) {
	if _, err := s.utilisateurs.FindByEmail(ctx, req.Email); !errors.Is(err, commun.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, commun.ErrEmailDejaUtilise
		}
		return nil, fmt.Errorf("vérification email: %w", err)
	}

	if _, err := s.utilisateurs.FindByTelephone(ctx, req.Telephone); !errors.Is(err, commun.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, commun.ErrTelephoneDejaUtilise
		}
		return nil, fmt.Errorf("vérification téléphone: %w", err)
	}

	// Le pays doit être couvert par une règle KYC Tier 1 : c'est cette
	// règle (jamais une constante codée en dur) qui fixe la devise et le
	// plafond initial du wallet.
	regle, err := s.reglesKyc.FindByPaysEtTier(ctx, req.PaysCode, commun.KycTier1)
	if err != nil {
		return nil, err
	}

	motDePasseHash, err := bcrypt.GenerateFromPassword([]byte(req.MotDePasse), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hachage mot de passe: %w", err)
	}

	utilisateur, err := commun.NouveauUtilisateur(req.Nom, req.Prenom, req.Email, req.Telephone, req.PaysCode, string(motDePasseHash))
	if err != nil {
		return nil, err
	}
	if err := utilisateur.ValiderKycTier1(); err != nil {
		return nil, err
	}

	wallet, err := commun.NouveauWallet(utilisateur.ID, utilisateur.PaysCode, regle.Devise, regle.PlafondSoldeCentimes)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.utilisateurs.Create(ctx, utilisateur); err != nil {
			return fmt.Errorf("création utilisateur: %w", err)
		}
		if err := s.wallets.Create(ctx, wallet); err != nil {
			return fmt.Errorf("création wallet: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &inputkyc.InscriptionResultat{Utilisateur: utilisateur, Wallet: wallet}, nil
}

func (s *kycService) DemanderTier2(ctx context.Context, utilisateurID string) (*domainkyc.DossierKyc, error) {
	utilisateur, err := s.utilisateurs.FindByID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if utilisateur.KycTier != commun.KycTier1 {
		return nil, commun.ErrTransitionKycInvalide
	}

	if _, err := s.dossiersKyc.FindEnAttenteByUtilisateurID(ctx, utilisateurID); !errors.Is(err, domainkyc.ErrDossierKycIntrouvable) {
		if err == nil {
			return nil, domainkyc.ErrDossierKycDejaEnAttente
		}
		return nil, fmt.Errorf("vérification dossier kyc existant: %w", err)
	}

	dossier, err := domainkyc.NouveauDossierKyc(utilisateurID)
	if err != nil {
		return nil, err
	}
	if err := s.dossiersKyc.Create(ctx, dossier); err != nil {
		return nil, fmt.Errorf("création dossier kyc: %w", err)
	}

	return dossier, nil
}
