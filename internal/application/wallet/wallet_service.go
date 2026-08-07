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

// Sujets des emails transactionnels du wallet. Envoyés en best-effort :
// un échec d'envoi ne doit jamais faire échouer l'opération financière
// elle-même (voir les appels `_ = s.notifierXxx(...)`).
const (
	sujetEmailRechargeConfirmee = "Votre recharge RAYCARD est confirmée"
	sujetEmailRechargeEchouee   = "Votre recharge RAYCARD a échoué"
	sujetEmailFondsDisponibles  = "Vos fonds RAYCARD sont maintenant disponibles"
	sujetEmailRetraitConfirme   = "Votre retrait RAYCARD est confirmé"
	sujetEmailRetraitEchoue     = "Votre retrait RAYCARD a échoué"
)

type walletService struct {
	utilisateurs outputcommun.UtilisateurRepository
	wallets      outputcommun.WalletRepository
	transactions outputwallet.TransactionRepository
	agregateur   outputwallet.AgregateurPaiement
	notifieur    outputcommun.Notifieur
	auditLog     outputcommun.AuditLogRepository
	txManager    outputcommun.TxManager
}

// NewWalletService construit l'implémentation de inputwallet.WalletUseCase
// et inputwallet.AdminWalletUseCase (voir walletService, qui implémente
// les deux).
func NewWalletService(
	utilisateurs outputcommun.UtilisateurRepository,
	wallets outputcommun.WalletRepository,
	transactions outputwallet.TransactionRepository,
	agregateur outputwallet.AgregateurPaiement,
	notifieur outputcommun.Notifieur,
	auditLog outputcommun.AuditLogRepository,
	txManager outputcommun.TxManager,
) inputwallet.WalletUseCase {
	return &walletService{
		utilisateurs: utilisateurs,
		wallets:      wallets,
		transactions: transactions,
		agregateur:   agregateur,
		notifieur:    notifieur,
		auditLog:     auditLog,
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

// ListerTransactionsAdmin liste les transactions tous wallets confondus —
// action back-office (voir middleware.RequireAdmin).
func (s *walletService) ListerTransactionsAdmin(ctx context.Context, filtre outputwallet.FiltreTransactions) ([]*domainwallet.Transaction, error) {
	return s.transactions.ListToutes(ctx, filtre)
}

// GelerWalletAdmin gèle n'importe quel wallet — action back-office,
// tracée dans l'audit log.
func (s *walletService) GelerWalletAdmin(ctx context.Context, adminID, walletID string) (*commun.Wallet, error) {
	w, err := s.wallets.FindByID(ctx, walletID)
	if err != nil {
		return nil, err
	}
	if err := w.Geler(time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := s.wallets.UpdateSolde(ctx, w); err != nil {
		return nil, fmt.Errorf("mise à jour wallet: %w", err)
	}
	_ = s.ecrireAuditLog(ctx, adminID, "wallet_gele_admin", "wallet", w.ID)
	return w, nil
}

// DegelerWalletAdmin dégèle n'importe quel wallet — action back-office,
// tracée dans l'audit log.
func (s *walletService) DegelerWalletAdmin(ctx context.Context, adminID, walletID string) (*commun.Wallet, error) {
	w, err := s.wallets.FindByID(ctx, walletID)
	if err != nil {
		return nil, err
	}
	if err := w.Degeler(time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := s.wallets.UpdateSolde(ctx, w); err != nil {
		return nil, fmt.Errorf("mise à jour wallet: %w", err)
	}
	_ = s.ecrireAuditLog(ctx, adminID, "wallet_degele_admin", "wallet", w.ID)
	return w, nil
}

// ecrireAuditLog trace une action administrateur sensible. Best-effort :
// un échec d'écriture ne doit jamais faire échouer l'action elle-même,
// déjà actée à ce stade (même principe que les notifications email).
func (s *walletService) ecrireAuditLog(ctx context.Context, adminID, action, cibleType, cibleID string) error {
	entree, err := commun.NouvelleEntreeAuditLog(adminID, action, cibleType, cibleID, "")
	if err != nil {
		return err
	}
	return s.auditLog.Create(ctx, entree)
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
		if err := s.transactions.Update(ctx, transaction); err != nil {
			return err
		}
		_ = s.notifierRechargeEchouee(ctx, transaction)
		return nil
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

	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
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
	if err != nil {
		return err
	}
	_ = s.notifierRechargeConfirmee(ctx, transaction)
	return nil
}

// traiterRetraitReussi ne touche jamais le wallet : le débit a déjà été
// appliqué à l'initiation (voir InitierRetrait). Cette confirmation ne
// fait qu'enregistrer les frais définitifs et clore la transaction.
func (s *walletService) traiterRetraitReussi(ctx context.Context, transaction *domainwallet.Transaction, evt *outputwallet.EvenementWebhook) error {
	if err := transaction.MarquerSucces(evt.FraisCentimes, nil); err != nil {
		return err
	}
	if err := s.transactions.Update(ctx, transaction); err != nil {
		return err
	}
	_ = s.notifierRetraitConfirme(ctx, transaction)
	return nil
}

// traiterRetraitEchoue rembourse le montant débité à l'initiation : c'est
// la seule situation où un remboursement automatique est sûr, puisque
// l'agrégateur confirme explicitement que l'argent n'est jamais parti
// (contrairement à une erreur réseau ambiguë sur l'appel initial).
func (s *walletService) traiterRetraitEchoue(ctx context.Context, transaction *domainwallet.Transaction) error {
	if err := transaction.MarquerEchouee(); err != nil {
		return err
	}

	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
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
	if err != nil {
		return err
	}
	_ = s.notifierRetraitEchoue(ctx, transaction)
	return nil
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
		_ = s.notifierFondsDisponibles(ctx, transaction)
		basculees++
	}
	return basculees, nil
}

// notifierRechargeConfirmee, notifierRechargeEchouee, notifierFondsDisponibles,
// notifierRetraitConfirme et notifierRetraitEchoue envoient chacune un
// email best-effort : un échec d'envoi ne doit jamais remonter à
// l'appelant, l'opération financière est déjà actée à ce stade.

func (s *walletService) notifierRechargeConfirmee(ctx context.Context, transaction *domainwallet.Transaction) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, transaction.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>Votre recharge de %d %s a été confirmée.</p>"+
			"<p>Montant net crédité (après %d %s de frais) : %d %s, disponible sous 48h.</p>",
		transaction.MontantCentimes, transaction.Devise,
		transaction.FraisCentimes, transaction.Devise,
		transaction.MontantNetCentimes(), transaction.Devise,
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailRechargeConfirmee, corps)
}

func (s *walletService) notifierRechargeEchouee(ctx context.Context, transaction *domainwallet.Transaction) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, transaction.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>Votre recharge de %d %s n'a pas abouti.</p>"+
			"<p>Aucun montant n'a été débité de votre compte Mobile Money. Vous pouvez réessayer.</p>",
		transaction.MontantCentimes, transaction.Devise,
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailRechargeEchouee, corps)
}

func (s *walletService) notifierFondsDisponibles(ctx context.Context, transaction *domainwallet.Transaction) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, transaction.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>%d %s sont maintenant disponibles sur votre wallet RAYCARD (recharge du %s).</p>",
		transaction.MontantNetCentimes(), transaction.Devise,
		transaction.CreatedAt.Format("02/01/2006"),
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailFondsDisponibles, corps)
}

func (s *walletService) notifierRetraitConfirme(ctx context.Context, transaction *domainwallet.Transaction) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, transaction.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>Votre retrait de %d %s a été confirmé et envoyé vers votre compte Mobile Money.</p>",
		transaction.MontantCentimes, transaction.Devise,
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailRetraitConfirme, corps)
}

func (s *walletService) notifierRetraitEchoue(ctx context.Context, transaction *domainwallet.Transaction) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, transaction.UtilisateurID)
	if err != nil {
		return err
	}
	corps := fmt.Sprintf(
		"<p>Votre retrait de %d %s n'a pas abouti.</p>"+
			"<p>Le montant a été intégralement recrédité sur votre wallet RAYCARD.</p>",
		transaction.MontantCentimes, transaction.Devise,
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailRetraitEchoue, corps)
}
