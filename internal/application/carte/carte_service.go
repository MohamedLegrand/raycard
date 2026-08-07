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

// Sujets des emails transactionnels de la carte. Envoyés en best-effort,
// même principe que le module wallet (voir wallet_service.go).
const (
	sujetEmailCarteCreee      = "Votre carte virtuelle RAYCARD est prête"
	sujetEmailCarteAnnulee    = "Votre carte virtuelle RAYCARD a été annulée"
	sujetEmailDepenseDetectee = "Nouvelle dépense sur votre carte RAYCARD"
)

type carteService struct {
	utilisateurs outputcommun.UtilisateurRepository
	wallets      outputcommun.WalletRepository
	transactions outputwallet.TransactionRepository
	cartes       outputcarte.CarteRepository
	depenses     outputcarte.DepenseCarteRepository
	agregateur   outputcarte.AgregateurCarte
	notifieur    outputcommun.Notifieur
	auditLog     outputcommun.AuditLogRepository
	txManager    outputcommun.TxManager
}

// NewCarteService construit l'implémentation de inputcarte.CarteUseCase et
// inputcarte.AdminCarteUseCase (voir carteService, qui implémente les
// deux).
func NewCarteService(
	utilisateurs outputcommun.UtilisateurRepository,
	wallets outputcommun.WalletRepository,
	transactions outputwallet.TransactionRepository,
	cartes outputcarte.CarteRepository,
	depenses outputcarte.DepenseCarteRepository,
	agregateur outputcarte.AgregateurCarte,
	notifieur outputcommun.Notifieur,
	auditLog outputcommun.AuditLogRepository,
	txManager outputcommun.TxManager,
) inputcarte.CarteUseCase {
	return &carteService{
		utilisateurs: utilisateurs,
		wallets:      wallets,
		transactions: transactions,
		cartes:       cartes,
		depenses:     depenses,
		agregateur:   agregateur,
		notifieur:    notifieur,
		auditLog:     auditLog,
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

	// utilisateur déjà chargé plus haut pour la vérification du palier KYC.
	corps := fmt.Sprintf(
		"<p>Votre carte « %s » est prête, chargée avec %d %s.</p>",
		carteCreee.Label, carteCreee.MontantChargeCentimes, carteCreee.Devise,
	)
	_ = s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailCarteCreee, corps)

	return carteCreee, nil
}

func (s *carteService) ListerCartes(ctx context.Context, utilisateurID string) ([]*domaincarte.Carte, error) {
	return s.cartes.ListByUtilisateurID(ctx, utilisateurID)
}

// ListerCartesAdmin liste les cartes tous utilisateurs confondus — action
// back-office (voir middleware.RequireAdmin).
func (s *carteService) ListerCartesAdmin(ctx context.Context, filtre outputcarte.FiltreCartes) ([]*domaincarte.Carte, error) {
	return s.cartes.ListToutes(ctx, filtre)
}

// ecrireAuditLog trace une action administrateur sensible. Best-effort :
// un échec d'écriture ne doit jamais faire échouer l'action elle-même,
// déjà actée à ce stade (même principe que les notifications email).
func (s *carteService) ecrireAuditLog(ctx context.Context, adminID, action, cibleType, cibleID string) error {
	entree, err := commun.NouvelleEntreeAuditLog(adminID, action, cibleType, cibleID, "")
	if err != nil {
		return err
	}
	return s.auditLog.Create(ctx, entree)
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
	return s.gelerCarte(ctx, c)
}

// GelerCarteAdmin gèle n'importe quelle carte, sans vérification de
// propriétaire — action back-office (voir middleware.RequireAdmin),
// tracée dans l'audit log.
func (s *carteService) GelerCarteAdmin(ctx context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.cartes.FindByID(ctx, carteID)
	if err != nil {
		return nil, err
	}
	c, err = s.gelerCarte(ctx, c)
	if err != nil {
		return nil, err
	}
	_ = s.ecrireAuditLog(ctx, adminID, "carte_gelee_admin", "carte", c.ID)
	return c, nil
}

// gelerCarte valide la transition avant tout appel réseau : inutile de
// solliciter l'agrégateur pour une carte déjà gelée ou annulée.
func (s *carteService) gelerCarte(ctx context.Context, c *domaincarte.Carte) (*domaincarte.Carte, error) {
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
	return s.degelerCarte(ctx, c)
}

// DegelerCarteAdmin dégèle n'importe quelle carte, sans vérification de
// propriétaire — action back-office, tracée dans l'audit log.
func (s *carteService) DegelerCarteAdmin(ctx context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.cartes.FindByID(ctx, carteID)
	if err != nil {
		return nil, err
	}
	c, err = s.degelerCarte(ctx, c)
	if err != nil {
		return nil, err
	}
	_ = s.ecrireAuditLog(ctx, adminID, "carte_degelee_admin", "carte", c.ID)
	return c, nil
}

func (s *carteService) degelerCarte(ctx context.Context, c *domaincarte.Carte) (*domaincarte.Carte, error) {
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
	return s.annulerCarte(ctx, c)
}

// AnnulerCarteAdmin annule n'importe quelle carte, sans vérification de
// propriétaire — action back-office, tracée dans l'audit log.
func (s *carteService) AnnulerCarteAdmin(ctx context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	c, err := s.cartes.FindByID(ctx, carteID)
	if err != nil {
		return nil, err
	}
	c, err = s.annulerCarte(ctx, c)
	if err != nil {
		return nil, err
	}
	_ = s.ecrireAuditLog(ctx, adminID, "carte_annulee_admin", "carte", c.ID)
	return c, nil
}

func (s *carteService) annulerCarte(ctx context.Context, c *domaincarte.Carte) (*domaincarte.Carte, error) {
	if c.Statut != domaincarte.StatutCarteActive && c.Statut != domaincarte.StatutCarteGelee {
		return nil, domaincarte.ErrTransitionCarteInvalide
	}

	w, err := s.wallets.FindByUtilisateurID(ctx, c.UtilisateurID)
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
		transaction, err = domainwallet.NouvelleTransactionAnnulationCarte(w.ID, c.UtilisateurID, w.Devise, soldeRestant)
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

	_ = s.notifierCarteAnnulee(ctx, c, soldeRestant)

	return c, nil
}

// notifierCarteAnnulee envoie un email best-effort après une annulation
// réussie : un échec d'envoi ne doit jamais remonter à l'appelant,
// l'annulation est déjà actée à ce stade.
func (s *carteService) notifierCarteAnnulee(ctx context.Context, c *domaincarte.Carte, soldeRestantCentimes int64) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, c.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf("<p>Votre carte « %s » a été annulée.</p>", c.Label)
	if soldeRestantCentimes > 0 {
		corps += fmt.Sprintf("<p>%d %s ont été recrédités sur votre wallet.</p>", soldeRestantCentimes, c.Devise)
	}
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailCarteAnnulee, corps)
}

// crediterCashback ajoute le cashback au solde disponible du wallet,
// immédiatement (contrairement à une recharge, ce n'est pas un flux Mobile
// Money externe soumis à un délai de rétention côté agrégateur).
func (s *carteService) crediterCashback(ctx context.Context, walletID string, montantCentimes int64) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		w, err := s.wallets.FindByID(ctx, walletID)
		if err != nil {
			return err
		}
		if err := w.Crediter(montantCentimes); err != nil {
			return err
		}
		return s.wallets.UpdateSolde(ctx, w)
	})
}

// notifierDepenseDetectee envoie un email best-effort après une dépense
// détectée par rapprochement de solde (voir SynchroniserSoldes) : un échec
// d'envoi ne doit jamais faire échouer la synchronisation.
func (s *carteService) notifierDepenseDetectee(ctx context.Context, c *domaincarte.Carte, montantDepenseCentimes int64) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, c.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>Une dépense de %d %s a été détectée sur votre carte « %s ».</p>"+
			"<p>Solde restant sur la carte : %d %s.</p>",
		montantDepenseCentimes, c.Devise, c.Label,
		c.SoldeCentimes, c.Devise,
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailDepenseDetectee, corps)
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

		var montantCashback int64
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
			montantCashback = depense.CashbackCentimes
			return s.depenses.Create(ctx, depense)
		})
		if err != nil {
			return depensesDetectees, fmt.Errorf("carte %s: %w", c.ID, err)
		}
		if montantDepense > 0 {
			depensesDetectees++
			_ = s.notifierDepenseDetectee(ctx, c, montantDepense)
			// Best-effort et hors transaction principale : un wallet gelé ou
			// déjà au plafond ne doit jamais faire échouer la détection de
			// dépense elle-même, qui reflète un fait déjà survenu côté carte.
			if montantCashback > 0 {
				_ = s.crediterCashback(ctx, c.WalletID, montantCashback)
			}
		}
	}
	return depensesDetectees, nil
}
