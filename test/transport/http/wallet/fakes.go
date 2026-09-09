// Package wallet fournit des fakes des use cases wallet
// (inputwallet.WalletUseCase, inputwallet.AdminWalletUseCase) pour les
// tests HTTP de la couche transport — même principe que
// test/transport/http/carte.
package wallet

import (
	"context"

	"raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputwallet "raycard/internal/core/ports/input/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

type WalletUseCaseFake struct {
	ObtenirWalletResultat *commun.Wallet
	ObtenirWalletErr      error

	ListerTransactionsResultat []*domainwallet.Transaction
	ListerTransactionsErr      error

	InitierRechargeResultat *domainwallet.Transaction
	InitierRechargeErr      error
	DerniereReqRecharge     inputwallet.InitierRechargeRequest

	InitierRetraitResultat *domainwallet.Transaction
	InitierRetraitErr      error
	DerniereReqRetrait     inputwallet.InitierRetraitRequest

	TraiterWebhookErr   error
	DernierCorpsWebhook []byte
	DerniereSignature   string

	BasculerFondsEcheusResultat int
	BasculerFondsEcheusErr      error

	DernierUtilisateurID string
}

func (f *WalletUseCaseFake) ObtenirWallet(_ context.Context, utilisateurID string) (*commun.Wallet, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirWalletResultat, f.ObtenirWalletErr
}

func (f *WalletUseCaseFake) ListerTransactions(_ context.Context, utilisateurID string) ([]*domainwallet.Transaction, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ListerTransactionsResultat, f.ListerTransactionsErr
}

func (f *WalletUseCaseFake) InitierRecharge(_ context.Context, utilisateurID string, req inputwallet.InitierRechargeRequest) (*domainwallet.Transaction, error) {
	f.DernierUtilisateurID = utilisateurID
	f.DerniereReqRecharge = req
	return f.InitierRechargeResultat, f.InitierRechargeErr
}

func (f *WalletUseCaseFake) InitierRetrait(_ context.Context, utilisateurID string, req inputwallet.InitierRetraitRequest) (*domainwallet.Transaction, error) {
	f.DernierUtilisateurID = utilisateurID
	f.DerniereReqRetrait = req
	return f.InitierRetraitResultat, f.InitierRetraitErr
}

func (f *WalletUseCaseFake) TraiterWebhook(_ context.Context, corps []byte, signature string) error {
	f.DernierCorpsWebhook = corps
	f.DerniereSignature = signature
	return f.TraiterWebhookErr
}

func (f *WalletUseCaseFake) BasculerFondsEcheus(_ context.Context) (int, error) {
	return f.BasculerFondsEcheusResultat, f.BasculerFondsEcheusErr
}

// AdminWalletUseCaseFake implémente inputwallet.AdminWalletUseCase.
type AdminWalletUseCaseFake struct {
	GelerWalletAdminResultat *commun.Wallet
	GelerWalletAdminErr      error

	DegelerWalletAdminResultat *commun.Wallet
	DegelerWalletAdminErr      error

	ListerTransactionsAdminResultat []*domainwallet.Transaction
	ListerTransactionsAdminErr      error
	DernierFiltre                   outputwallet.FiltreTransactions

	DernierAdminID  string
	DernierWalletID string
}

func (f *AdminWalletUseCaseFake) GelerWalletAdmin(_ context.Context, adminID, walletID string) (*commun.Wallet, error) {
	f.DernierAdminID, f.DernierWalletID = adminID, walletID
	return f.GelerWalletAdminResultat, f.GelerWalletAdminErr
}

func (f *AdminWalletUseCaseFake) DegelerWalletAdmin(_ context.Context, adminID, walletID string) (*commun.Wallet, error) {
	f.DernierAdminID, f.DernierWalletID = adminID, walletID
	return f.DegelerWalletAdminResultat, f.DegelerWalletAdminErr
}

func (f *AdminWalletUseCaseFake) ListerTransactionsAdmin(_ context.Context, filtre outputwallet.FiltreTransactions) ([]*domainwallet.Transaction, error) {
	f.DernierFiltre = filtre
	return f.ListerTransactionsAdminResultat, f.ListerTransactionsAdminErr
}
