// Package application implémente les use cases (ports/input) en
// orchestrant le domaine et les ports/output. Il ne connaît aucun
// détail de transport (HTTP) ni d'infrastructure concrète (Postgres,
// partenaires) : uniquement leurs interfaces.
package application

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/core/ports/output"
)

type kycService struct {
	utilisateurs output.UtilisateurRepository
	wallets      output.WalletRepository
	reglesKyc    output.ReglesKycRepository
	txManager    output.TxManager
}

// NewKycService construit l'implémentation de input.KycUseCase.
func NewKycService(
	utilisateurs output.UtilisateurRepository,
	wallets output.WalletRepository,
	reglesKyc output.ReglesKycRepository,
	txManager output.TxManager,
) input.KycUseCase {
	return &kycService{
		utilisateurs: utilisateurs,
		wallets:      wallets,
		reglesKyc:    reglesKyc,
		txManager:    txManager,
	}
}

func (s *kycService) Inscrire(ctx context.Context, req input.InscriptionRequest) (*input.InscriptionResultat, error) {
	if _, err := s.utilisateurs.FindByEmail(ctx, req.Email); !errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, domain.ErrEmailDejaUtilise
		}
		return nil, fmt.Errorf("vérification email: %w", err)
	}

	if _, err := s.utilisateurs.FindByTelephone(ctx, req.Telephone); !errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		if err == nil {
			return nil, domain.ErrTelephoneDejaUtilise
		}
		return nil, fmt.Errorf("vérification téléphone: %w", err)
	}

	// Le pays doit être couvert par une règle KYC Tier 1 : c'est cette
	// règle (jamais une constante codée en dur) qui fixe la devise et le
	// plafond initial du wallet.
	regle, err := s.reglesKyc.FindByPaysEtTier(ctx, req.PaysCode, domain.KycTier1)
	if err != nil {
		return nil, err
	}

	motDePasseHash, err := bcrypt.GenerateFromPassword([]byte(req.MotDePasse), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hachage mot de passe: %w", err)
	}

	utilisateur, err := domain.NouveauUtilisateur(req.Nom, req.Prenom, req.Email, req.Telephone, req.PaysCode, string(motDePasseHash))
	if err != nil {
		return nil, err
	}
	if err := utilisateur.ValiderKycTier1(); err != nil {
		return nil, err
	}

	wallet, err := domain.NouveauWallet(utilisateur.ID, utilisateur.PaysCode, regle.Devise, regle.PlafondSoldeCentimes)
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

	return &input.InscriptionResultat{Utilisateur: utilisateur, Wallet: wallet}, nil
}
