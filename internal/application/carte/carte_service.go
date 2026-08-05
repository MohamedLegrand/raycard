// Package carte implémente les use cases (ports/input/carte) en
// orchestrant le domaine et les ports/output. Il ne connaît aucun détail
// de transport (HTTP) ni d'infrastructure concrète (Postgres, SDK de
// l'agrégateur de paiement) : uniquement leurs interfaces.
package carte

import (
	"context"
	"errors"
	"fmt"
	"time"

	domaincarte "raycard/internal/core/domain/carte"
	"raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputcarte "raycard/internal/core/ports/input/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
	outputcommun "raycard/internal/core/ports/output/commun"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

// intervalleVerificationCarteBase/Max bornent la fréquence de sondage du
// solde d'une carte (voir carte.Carte.MettreAJourSolde) : 20s pour une
// carte qui vient de montrer une dépense, jusqu'à 30 min pour une carte
// dormante — évite de solliciter l'agrégateur au même rythme pour toutes
// les cartes, actives ou non.
const (
	intervalleVerificationCarteBase = 20 * time.Second
	intervalleVerificationCarteMax  = 30 * time.Minute
)

type carteService struct {
	utilisateurs outputcommun.UtilisateurRepository
	wallets      outputcommun.WalletRepository
	transactions outputwallet.TransactionRepository
	cartes       outputcarte.CarteRepository
	depenses     outputcarte.DepenseCarteRepository
	agregateur   outputcarte.AgregateurCarte
	txManager    outputcommun.TxManager
}

// NewCarteService construit l'implémentation de inputcarte.CarteUseCase.
func NewCarteService(
	utilisateurs outputcommun.UtilisateurRepository,
	wallets outputcommun.WalletRepository,
	transactions outputwallet.TransactionRepository,
	cartes outputcarte.CarteRepository,
	depenses outputcarte.DepenseCarteRepository,
	agregateur outputcarte.AgregateurCarte,
	txManager outputcommun.TxManager,
) inputcarte.CarteUseCase {
	return &carteService{
		utilisateurs: utilisateurs,
		wallets:      wallets,
		transactions: transactions,
		cartes:       cartes,
		depenses:     depenses,
		agregateur:   agregateur,
		txManager:    txManager,
	}
}

func (s *carteService) CreerCarte(ctx context.Context, utilisateurID string, req inputcarte.CreerCarteRequest) (*domaincarte.Carte, error) {
	utilisateur, err := s.utilisateurs.FindByID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if utilisateur.KycTier < commun.KycTier2 {
		return nil, domaincarte.ErrKycTierInsuffisant
	}

	w, err := s.wallets.FindByUtilisateurID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if w.Statut != commun.StatutWalletActif {
		return nil, commun.ErrWalletGele
	}

	if _, err := s.transactions.FindEnCoursByWalletID(ctx, w.ID); !errors.Is(err, domainwallet.ErrTransactionIntrouvable) {
		if err == nil {
			return nil, domainwallet.ErrTransactionDejaEnCours
		}
		return nil, fmt.Errorf("vérification transaction en cours: %w", err)
	}

	transaction, err := domainwallet.NouvelleTransactionFinancementCarte(w.ID, utilisateurID, w.Devise, req.MontantCentimes)
	if err != nil {
		return nil, err
	}

	// Comme le retrait, le débit précède l'appel à l'agrégateur : on ne
	// peut jamais promettre une carte qu'on ne peut pas financer.
	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := w.Debiter(req.MontantCentimes); err != nil {
			return err
		}
		if err := s.wallets.UpdateSolde(ctx, w); err != nil {
			return fmt.Errorf("débit wallet: %w", err)
		}
		if err := s.transactions.Create(ctx, transaction); err != nil {
			return fmt.Errorf("création transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	resultat, err := s.agregateur.CreerCarte(ctx, outputcarte.CreerCarteParams{
		Label:           req.Label,
		Devise:          w.Devise,
		MontantCentimes: req.MontantCentimes,
	})
	if err != nil {
		// Le débit reste appliqué : on ne sait pas avec certitude si la
		// carte a été émise côté agrégateur (même politique que pour un
		// retrait — voir wallet.WalletUseCase.InitierRetrait). Jamais de
		// remboursement automatique sur une erreur ambiguë ; nouvelle
		// tentative bloquée par FindEnCoursByWalletID jusqu'à réconciliation
		// manuelle (ex: lister les cartes existantes côté agrégateur).
		return nil, fmt.Errorf("%w: %v", domaincarte.ErrEmissionEchouee, err)
	}

	if err := transaction.MarquerEnvoyee(resultat.IDExterne); err != nil {
		return nil, err
	}
	// Émission synchrone : contrairement au cash-in/cash-out, aucun
	// webhook ne viendra confirmer plus tard — la transaction est close
	// immédiatement. Les frais ne sont pas exposés par le SDK à la
	// création (0 par défaut).
	if err := transaction.MarquerSucces(0, nil); err != nil {
		return nil, err
	}

	carteCreee, err := domaincarte.NouvelleCarte(utilisateurID, w.ID, resultat.IDExterne, req.Label, w.Devise, req.MontantCentimes)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.transactions.Update(ctx, transaction); err != nil {
			return fmt.Errorf("mise à jour transaction: %w", err)
		}
		if err := s.cartes.Create(ctx, carteCreee); err != nil {
			return fmt.Errorf("création carte: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return carteCreee, nil
}

func (s *carteService) ListerCartes(ctx context.Context, utilisateurID string) ([]*domaincarte.Carte, error) {
	return s.cartes.ListByUtilisateurID(ctx, utilisateurID)
}

func (s *carteService) ObtenirCarte(ctx context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	return s.carteDeUtilisateur(ctx, utilisateurID, carteID)
}

// carteDeUtilisateur retrouve une carte et vérifie qu'elle appartient bien
// à utilisateurID — jamais domaincarte.ErrCarteIntrouvable ne distingue
// "carte inexistante" de "carte d'un autre utilisateur", pour ne jamais
// révéler l'existence de la carte de quelqu'un d'autre.
func (s *carteService) carteDeUtilisateur(ctx context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.cartes.FindByID(ctx, carteID)
	if err != nil {
		return nil, err
	}
	if c.UtilisateurID != utilisateurID {
		return nil, domaincarte.ErrCarteIntrouvable
	}
	return c, nil
}

func (s *carteService) GelerCarte(ctx context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.carteDeUtilisateur(ctx, utilisateurID, carteID)
	if err != nil {
		return nil, err
	}

	// Valide la transition avant tout appel réseau : inutile de solliciter
	// l'agrégateur pour une carte déjà gelée ou annulée.
	if err := c.Geler(time.Now().UTC()); err != nil {
		return nil, err
	}

	if err := s.agregateur.GelerCarte(ctx, c.IDExterne); err != nil {
		return nil, fmt.Errorf("gel carte: %w", err)
	}

	if err := s.cartes.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("mise à jour carte: %w", err)
	}

	return c, nil
}

func (s *carteService) DegelerCarte(ctx context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.carteDeUtilisateur(ctx, utilisateurID, carteID)
	if err != nil {
		return nil, err
	}

	if err := c.Degeler(time.Now().UTC()); err != nil {
		return nil, err
	}

	if err := s.agregateur.DegelerCarte(ctx, c.IDExterne); err != nil {
		return nil, fmt.Errorf("dégel carte: %w", err)
	}

	if err := s.cartes.Update(ctx, c); err != nil {
		return nil, fmt.Errorf("mise à jour carte: %w", err)
	}

	return c, nil
}

func (s *carteService) RechargerCarte(ctx context.Context, utilisateurID, carteID string, req inputcarte.RechargerCarteRequest) (*domaincarte.Carte, error) {
	c, err := s.carteDeUtilisateur(ctx, utilisateurID, carteID)
	if err != nil {
		return nil, err
	}
	if c.Statut != domaincarte.StatutCarteActive {
		return nil, domaincarte.ErrTransitionCarteInvalide
	}

	w, err := s.wallets.FindByUtilisateurID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if w.Statut != commun.StatutWalletActif {
		return nil, commun.ErrWalletGele
	}

	if _, err := s.transactions.FindEnCoursByWalletID(ctx, w.ID); !errors.Is(err, domainwallet.ErrTransactionIntrouvable) {
		if err == nil {
			return nil, domainwallet.ErrTransactionDejaEnCours
		}
		return nil, fmt.Errorf("vérification transaction en cours: %w", err)
	}

	transaction, err := domainwallet.NouvelleTransactionFinancementCarte(w.ID, utilisateurID, w.Devise, req.MontantCentimes)
	if err != nil {
		return nil, err
	}

	// Même politique que pour la création : le débit précède l'appel à
	// l'agrégateur.
	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := w.Debiter(req.MontantCentimes); err != nil {
			return err
		}
		if err := s.wallets.UpdateSolde(ctx, w); err != nil {
			return fmt.Errorf("débit wallet: %w", err)
		}
		if err := s.transactions.Create(ctx, transaction); err != nil {
			return fmt.Errorf("création transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	soldeApres, err := s.agregateur.RechargerCarte(ctx, c.IDExterne, req.MontantCentimes)
	if err != nil {
		// Le débit reste appliqué : même politique que pour une erreur
		// ambiguë sur la création (voir CreerCarte).
		return nil, fmt.Errorf("%w: %v", domaincarte.ErrRechargeEchouee, err)
	}

	// La carte peut être rechargée plusieurs fois : contrairement à la
	// création (une référence par carte), il faut une référence propre à
	// cette transaction pour ne jamais entrer en conflit avec la
	// contrainte d'unicité sur reference_externe.
	referenceTopup := c.IDExterne + ":topup:" + transaction.ID
	if err := transaction.MarquerEnvoyee(referenceTopup); err != nil {
		return nil, err
	}
	// Recharge synchrone : pas de webhook à venir, comme pour la création.
	if err := transaction.MarquerSucces(0, nil); err != nil {
		return nil, err
	}

	if err := c.Recharger(req.MontantCentimes, soldeApres, time.Now().UTC()); err != nil {
		return nil, err
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.transactions.Update(ctx, transaction); err != nil {
			return fmt.Errorf("mise à jour transaction: %w", err)
		}
		if err := s.cartes.Update(ctx, c); err != nil {
			return fmt.Errorf("mise à jour carte: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s *carteService) AnnulerCarte(ctx context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.carteDeUtilisateur(ctx, utilisateurID, carteID)
	if err != nil {
		return nil, err
	}
	if c.Statut != domaincarte.StatutCarteActive && c.Statut != domaincarte.StatutCarteGelee {
		return nil, domaincarte.ErrTransitionCarteInvalide
	}

	w, err := s.wallets.FindByUtilisateurID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if w.Statut != commun.StatutWalletActif {
		return nil, commun.ErrWalletGele
	}

	if _, err := s.transactions.FindEnCoursByWalletID(ctx, w.ID); !errors.Is(err, domainwallet.ErrTransactionIntrouvable) {
		if err == nil {
			return nil, domainwallet.ErrTransactionDejaEnCours
		}
		return nil, fmt.Errorf("vérification transaction en cours: %w", err)
	}

	// Contrairement au financement/à la recharge, on appelle l'agrégateur
	// avant de toucher au wallet : Cancel détruit la carte de façon
	// irréversible, on ne veut jamais rembourser par anticipation une
	// annulation qui pourrait échouer.
	soldeRestant, err := s.agregateur.AnnulerCarte(ctx, c.IDExterne)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domaincarte.ErrAnnulationEchouee, err)
	}

	if err := c.Annuler(time.Now().UTC()); err != nil {
		return nil, err
	}

	var transaction *domainwallet.Transaction
	if soldeRestant > 0 {
		transaction, err = domainwallet.NouvelleTransactionAnnulationCarte(w.ID, utilisateurID, w.Devise, soldeRestant)
		if err != nil {
			return nil, err
		}
		// Référence unique par annulation, comme pour une recharge : une
		// même carte ne peut être annulée qu'une fois, mais la référence de
		// création est déjà prise par la transaction de financement initiale.
		referenceAnnulation := c.IDExterne + ":annulation:" + transaction.ID
		if err := transaction.MarquerEnvoyee(referenceAnnulation); err != nil {
			return nil, err
		}
		if err := transaction.MarquerSucces(0, nil); err != nil {
			return nil, err
		}
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.cartes.Update(ctx, c); err != nil {
			return fmt.Errorf("mise à jour carte: %w", err)
		}
		if transaction == nil {
			return nil
		}
		if err := w.Crediter(soldeRestant); err != nil {
			return err
		}
		if err := s.wallets.UpdateSolde(ctx, w); err != nil {
			return fmt.Errorf("remboursement wallet: %w", err)
		}
		if err := s.transactions.Create(ctx, transaction); err != nil {
			return fmt.Errorf("création transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s *carteService) ListerDepenses(ctx context.Context, utilisateurID, carteID string) ([]*domaincarte.DepenseCarte, error) {
	if _, err := s.carteDeUtilisateur(ctx, utilisateurID, carteID); err != nil {
		return nil, err
	}
	return s.depenses.ListByCarteID(ctx, carteID)
}

// SynchroniserSoldes interroge l'agrégateur pour chaque carte active et
// détecte les dépenses par comparaison de solde — seul moyen disponible
// faute de webhook de transaction carte côté agrégateur (voir le
// commentaire sur carte.Carte.SoldeCentimes). Appelé périodiquement par
// un job planifié (voir cmd/api/main.go).
func (s *carteService) SynchroniserSoldes(ctx context.Context) (int, error) {
	maintenant := time.Now().UTC()
	cartes, err := s.cartes.ListAVerifier(ctx, maintenant)
	if err != nil {
		return 0, fmt.Errorf("liste cartes à vérifier: %w", err)
	}

	var depensesDetectees int
	for _, c := range cartes {
		soldeObserve, statutObserve, err := s.agregateur.ObtenirEtatCarte(ctx, c.IDExterne)
		if err != nil {
			return depensesDetectees, fmt.Errorf("carte %s: %w", c.ID, err)
		}

		// Un gel ou une annulation décidée côté agrégateur (ex: fraude
		// suspectée) doit se refléter ici, sans attendre une action de
		// l'utilisateur. Une fois la carte non-active, elle sort
		// naturellement des prochains passages (voir ListAVerifier).
		c.SynchroniserStatut(statutObserve, maintenant)

		soldeAvant := c.SoldeCentimes
		montantDepense, err := c.MettreAJourSolde(soldeObserve, maintenant, intervalleVerificationCarteBase, intervalleVerificationCarteMax)
		if err != nil {
			return depensesDetectees, fmt.Errorf("carte %s: %w", c.ID, err)
		}

		err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
			if err := s.cartes.Update(ctx, c); err != nil {
				return fmt.Errorf("mise à jour carte: %w", err)
			}
			if montantDepense <= 0 {
				return nil
			}
			depense, err := domaincarte.NouvelleDepenseCarte(c.ID, montantDepense, soldeAvant, c.SoldeCentimes)
			if err != nil {
				return err
			}
			return s.depenses.Create(ctx, depense)
		})
		if err != nil {
			return depensesDetectees, fmt.Errorf("carte %s: %w", c.ID, err)
		}
		if montantDepense > 0 {
			depensesDetectees++
		}
	}
	return depensesDetectees, nil
}
