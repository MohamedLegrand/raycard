// Package wallet fournit de faux repositories et un faux agrégateur de
// paiement en mémoire pour les tests du service applicatif wallet.
// Ce n'est volontairement pas un fichier _test.go : wallet_service_test.go
// vit dans le package wallet_test et doit pouvoir importer ces symboles.
package wallet

import (
	"context"
	"errors"
	"time"

	domainwallet "raycard/internal/core/domain/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

type TransactionRepoFake struct {
	parID map[string]*domainwallet.Transaction
}

func NewTransactionRepoFake() *TransactionRepoFake {
	return &TransactionRepoFake{parID: make(map[string]*domainwallet.Transaction)}
}

func (r *TransactionRepoFake) Create(_ context.Context, t *domainwallet.Transaction) error {
	r.parID[t.ID] = t
	return nil
}

func (r *TransactionRepoFake) FindByID(_ context.Context, id string) (*domainwallet.Transaction, error) {
	if t, ok := r.parID[id]; ok {
		return t, nil
	}
	return nil, domainwallet.ErrTransactionIntrouvable
}

func (r *TransactionRepoFake) FindByReferenceExterne(_ context.Context, referenceExterne string) (*domainwallet.Transaction, error) {
	for _, t := range r.parID {
		if t.ReferenceExterne == referenceExterne {
			return t, nil
		}
	}
	return nil, domainwallet.ErrTransactionIntrouvable
}

func (r *TransactionRepoFake) FindEnCoursByWalletID(_ context.Context, walletID string) (*domainwallet.Transaction, error) {
	for _, t := range r.parID {
		if t.WalletID != walletID {
			continue
		}
		if t.Statut == domainwallet.StatutTransactionEnAttente || t.Statut == domainwallet.StatutTransactionEnvoyee {
			return t, nil
		}
	}
	return nil, domainwallet.ErrTransactionIntrouvable
}

func (r *TransactionRepoFake) ListDisponiblesEcheance(_ context.Context, avant time.Time) ([]*domainwallet.Transaction, error) {
	var resultat []*domainwallet.Transaction
	for _, t := range r.parID {
		if t.Statut == domainwallet.StatutTransactionSucces && t.DisponibleLe != nil && !t.DisponibleLe.After(avant) {
			resultat = append(resultat, t)
		}
	}
	return resultat, nil
}

func (r *TransactionRepoFake) Update(_ context.Context, t *domainwallet.Transaction) error {
	if _, ok := r.parID[t.ID]; !ok {
		return domainwallet.ErrTransactionIntrouvable
	}
	r.parID[t.ID] = t
	return nil
}

// AgregateurPaiementFake simule l'agrégateur de paiement externe.
// ErreurInitiation, si non nil, est renvoyée par InitierCashIn et
// InitierCashOut (simule un échec réseau ou une erreur métier de
// l'agrégateur). EvenementARenvoyer est ce que renvoie
// ConstruireEvenementWebhook.
type AgregateurPaiementFake struct {
	ReferenceGeneree     string
	ErreurInitiation     error
	EvenementARenvoyer   *outputwallet.EvenementWebhook
	ErreurWebhook        error
	AppelsInitierCashIn  int
	AppelsInitierCashOut int
}

func (a *AgregateurPaiementFake) InitierCashIn(_ context.Context, _ outputwallet.InitierCashInParams) (*outputwallet.InitierCashInResultat, error) {
	a.AppelsInitierCashIn++
	if a.ErreurInitiation != nil {
		return nil, a.ErreurInitiation
	}
	reference := a.ReferenceGeneree
	if reference == "" {
		reference = "ref-fake-1"
	}
	return &outputwallet.InitierCashInResultat{ReferenceExterne: reference}, nil
}

func (a *AgregateurPaiementFake) InitierCashOut(_ context.Context, _ outputwallet.InitierCashOutParams) (*outputwallet.InitierCashOutResultat, error) {
	a.AppelsInitierCashOut++
	if a.ErreurInitiation != nil {
		return nil, a.ErreurInitiation
	}
	reference := a.ReferenceGeneree
	if reference == "" {
		reference = "ref-fake-1"
	}
	return &outputwallet.InitierCashOutResultat{ReferenceExterne: reference}, nil
}

func (a *AgregateurPaiementFake) ConstruireEvenementWebhook(_ []byte, _ string) (*outputwallet.EvenementWebhook, error) {
	if a.ErreurWebhook != nil {
		return nil, a.ErreurWebhook
	}
	if a.EvenementARenvoyer != nil {
		return a.EvenementARenvoyer, nil
	}
	return nil, errors.New("AgregateurPaiementFake: aucun évènement configuré")
}
