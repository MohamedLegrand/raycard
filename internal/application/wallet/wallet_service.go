// Package wallet implémente les use cases (ports/input/wallet) en
// orchestrant le domaine et les ports/output. Il ne connaît aucun détail
// de transport (HTTP) ni d'infrastructure concrète (Postgres, SDK de
// l'agrégateur de paiement) : uniquement leurs interfaces.
package wallet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputwallet "raycard/internal/core/ports/input/wallet"
	outputcommun "raycard/internal/core/ports/output/commun"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

// dureeRetenueCashIn : délai pendant lequel une recharge confirmée reste
// en attente avant de devenir disponible, imposé par l'agrégateur de
// paiement (HR-Skills Pay : 48h).
const dureeRetenueCashIn = 48 * time.Hour

type walletService struct {
	wallets      outputcommun.WalletRepository
	transactions outputwallet.TransactionRepository
	agregateur   outputwallet.AgregateurPaiement
	txManager    outputcommun.TxManager
}

// NewWalletService construit l'implémentation de inputwallet.WalletUseCase.
func NewWalletService(
	wallets outputcommun.WalletRepository,
	transactions outputwallet.TransactionRepository,
	agregateur outputwallet.AgregateurPaiement,
	txManager outputcommun.TxManager,
) inputwallet.WalletUseCase {
	return &walletService{
		wallets:      wallets,
		transactions: transactions,
		agregateur:   agregateur,
		txManager:    txManager,
	}
}

func (s *walletService) ObtenirWallet(ctx context.Context, utilisateurID string) (*commun.Wallet, error) {
	return s.wallets.FindByUtilisateurID(ctx, utilisateurID)
}

func (s *walletService) ListerTransactions(ctx context.Context, utilisateurID string) ([]*domainwallet.Transaction, error) {
	w, err := s.wallets.FindByUtilisateurID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	return s.transactions.ListByWalletID(ctx, w.ID)
}

func (s *walletService) InitierRecharge(ctx context.Context, utilisateurID string, req inputwallet.InitierRechargeRequest) (*domainwallet.Transaction, error) {
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

	transaction, err := domainwallet.NouvelleTransactionRecharge(w.ID, utilisateurID, w.Devise, req.Operateur, req.Telephone, req.MontantCentimes)
	if err != nil {
		return nil, err
	}
	if err := s.transactions.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("création transaction: %w", err)
	}

	// Appel à l'agrégateur en dehors de toute transaction ACID : c'est un
	// appel réseau externe, il ne doit jamais tenir un verrou de ligne
	// PostgreSQL en attendant. Une fois la transaction locale créée
	// EnAttente, elle sert de verrou logique : cet appel ne sera jamais
	// rejoué automatiquement pour cette même transaction (voir
	// AgregateurPaiement).
	resultat, err := s.agregateur.InitierCashIn(ctx, outputwallet.InitierCashInParams{
		Telephone:       req.Telephone,
		Operateur:       req.Operateur,
		PaysCode:        w.PaysCode,
		Devise:          w.Devise,
		MontantCentimes: req.MontantCentimes,
	})
	if err != nil {
		// La transaction reste EnAttente : on ne sait pas avec certitude si
		// l'agrégateur a reçu la demande. Elle ne sera jamais reprise
		// automatiquement — une nouvelle tentative de l'utilisateur sera
		// bloquée par FindEnCoursByWalletID tant qu'une réconciliation
		// manuelle n'aura pas tranché.
		return nil, fmt.Errorf("initiation cash-in: %w", err)
	}

	if err := transaction.MarquerEnvoyee(resultat.ReferenceExterne); err != nil {
		return nil, err
	}
	if err := s.transactions.Update(ctx, transaction); err != nil {
		return nil, fmt.Errorf("mise à jour transaction: %w", err)
	}

	return transaction, nil
}

func (s *walletService) InitierRetrait(ctx context.Context, utilisateurID string, req inputwallet.InitierRetraitRequest) (*domainwallet.Transaction, error) {
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

	transaction, err := domainwallet.NouvelleTransactionRetrait(w.ID, utilisateurID, w.Devise, req.Operateur, req.Telephone, req.MontantCentimes)
	if err != nil {
		return nil, err
	}

	// Contrairement à la recharge, le débit est appliqué ici, avant même
	// l'appel à l'agrégateur : une fois celui-ci sollicité, l'argent part
	// réellement vers l'utilisateur — impossible de promettre un retrait
	// qu'on ne peut pas couvrir. Débit et création de la transaction sont
	// atomiques (une seule écriture DB), pour ne jamais laisser le wallet
	// débité sans trace de la transaction correspondante.
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

	resultat, err := s.agregateur.InitierCashOut(ctx, outputwallet.InitierCashOutParams{
		Telephone:       req.Telephone,
		Operateur:       req.Operateur,
		PaysCode:        w.PaysCode,
		Devise:          w.Devise,
		MontantCentimes: req.MontantCentimes,
	})
	if err != nil {
		// Le débit reste appliqué : on ne sait pas avec certitude si
		// l'agrégateur a reçu la demande. Ne jamais rembourser
		// automatiquement sur une erreur ambiguë ici — seule une
		// confirmation explicite de l'agrégateur (payment.failed) déclenche
		// un remboursement (voir traiterRetraitEchoue). Une nouvelle
		// tentative est bloquée par FindEnCoursByWalletID tant qu'une
		// réconciliation manuelle n'aura pas tranché.
		return nil, fmt.Errorf("initiation cash-out: %w", err)
	}

	if err := transaction.MarquerEnvoyee(resultat.ReferenceExterne); err != nil {
		return nil, err
	}
	if err := s.transactions.Update(ctx, transaction); err != nil {
		return nil, fmt.Errorf("mise à jour transaction: %w", err)
	}

	return transaction, nil
}

func (s *walletService) TraiterWebhook(ctx context.Context, corps []byte, signature string) error {
	evt, err := s.agregateur.ConstruireEvenementWebhook(corps, signature)
	if err != nil {
		return err
	}

	transaction, err := s.transactions.FindByReferenceExterne(ctx, evt.ReferenceExterne)
	if err != nil {
		return err
	}

	// Webhook rejoué (retry réseau côté agrégateur) : la transaction n'est
	// plus Envoyee, donc déjà traitée. On répond succès sans rien
	// réappliquer — comportement idempotent attendu par l'agrégateur.
	if transaction.Statut != domainwallet.StatutTransactionEnvoyee {
		return nil
	}

	switch evt.Type {
	case outputwallet.EvenementPaiementReussi:
		if transaction.Type == domainwallet.TypeTransactionRetrait {
			return s.traiterRetraitReussi(ctx, transaction, evt)
		}
		return s.traiterRechargeReussie(ctx, transaction, evt)
	case outputwallet.EvenementPaiementEchoue:
		if transaction.Type == domainwallet.TypeTransactionRetrait {
			return s.traiterRetraitEchoue(ctx, transaction)
		}
		if err := transaction.MarquerEchouee(); err != nil {
			return err
		}
		return s.transactions.Update(ctx, transaction)
	default:
		return nil
	}
}

// traiterRechargeReussie crédite le wallet en attente : c'est la
// confirmation de l'agrégateur qui déclenche le crédit (l'argent vient de
// l'utilisateur, on ne le compte qu'une fois reçu).
func (s *walletService) traiterRechargeReussie(ctx context.Context, transaction *domainwallet.Transaction, evt *outputwallet.EvenementWebhook) error {
	disponibleLe := time.Now().UTC().Add(dureeRetenueCashIn)
	if err := transaction.MarquerSucces(evt.FraisCentimes, &disponibleLe); err != nil {
		return err
	}

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		w, err := s.wallets.FindByID(ctx, transaction.WalletID)
		if err != nil {
			return err
		}
		if err := w.CrediterEnAttente(transaction.MontantNetCentimes()); err != nil {
			return err
		}
		if err := s.wallets.UpdateSolde(ctx, w); err != nil {
			return fmt.Errorf("crédit wallet: %w", err)
		}
		if err := s.transactions.Update(ctx, transaction); err != nil {
			return fmt.Errorf("mise à jour transaction: %w", err)
		}
		return nil
	})
}

// traiterRetraitReussi ne touche jamais le wallet : le débit a déjà été
// appliqué à l'initiation (voir InitierRetrait). Cette confirmation ne
// fait qu'enregistrer les frais définitifs et clore la transaction.
func (s *walletService) traiterRetraitReussi(ctx context.Context, transaction *domainwallet.Transaction, evt *outputwallet.EvenementWebhook) error {
	if err := transaction.MarquerSucces(evt.FraisCentimes, nil); err != nil {
		return err
	}
	return s.transactions.Update(ctx, transaction)
}

// traiterRetraitEchoue rembourse le montant débité à l'initiation : c'est
// la seule situation où un remboursement automatique est sûr, puisque
// l'agrégateur confirme explicitement que l'argent n'est jamais parti
// (contrairement à une erreur réseau ambiguë sur l'appel initial).
func (s *walletService) traiterRetraitEchoue(ctx context.Context, transaction *domainwallet.Transaction) error {
	if err := transaction.MarquerEchouee(); err != nil {
		return err
	}

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		w, err := s.wallets.FindByID(ctx, transaction.WalletID)
		if err != nil {
			return err
		}
		if err := w.Crediter(transaction.MontantCentimes); err != nil {
			return err
		}
		if err := s.wallets.UpdateSolde(ctx, w); err != nil {
			return fmt.Errorf("remboursement wallet: %w", err)
		}
		if err := s.transactions.Update(ctx, transaction); err != nil {
			return fmt.Errorf("mise à jour transaction: %w", err)
		}
		return nil
	})
}

// BasculerFondsEcheus fait passer d'en-attente à disponible les montants
// dont le délai de retenue de l'agrégateur est écoulé. Appelé
// périodiquement par un job planifié (voir cmd/api/main.go) — jamais
// déclenché par une requête utilisateur.
func (s *walletService) BasculerFondsEcheus(ctx context.Context) (int, error) {
	transactions, err := s.transactions.ListDisponiblesEcheance(ctx, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("liste transactions échues: %w", err)
	}

	var basculees int
	for _, transaction := range transactions {
		err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
			w, err := s.wallets.FindByID(ctx, transaction.WalletID)
			if err != nil {
				return err
			}
			if err := w.BasculerEnAttenteVersDisponible(transaction.MontantNetCentimes()); err != nil {
				return err
			}
			if err := s.wallets.UpdateSolde(ctx, w); err != nil {
				return fmt.Errorf("bascule solde wallet: %w", err)
			}
			transaction.DisponibleLe = nil
			if err := s.transactions.Update(ctx, transaction); err != nil {
				return fmt.Errorf("mise à jour transaction: %w", err)
			}
			return nil
		})
		if err != nil {
			return basculees, fmt.Errorf("transaction %s: %w", transaction.ID, err)
		}
		basculees++
	}
	return basculees, nil
}
