package wallet

import (
	"context"

	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

// AdminWalletUseCase orchestre les actions back-office sur les wallets et
// la consultation système des transactions — séparé de WalletUseCase pour
// ne jamais confondre un accès client et un accès administrateur, même si
// l'implémentation est partagée (voir walletService, qui implémente les
// deux interfaces).
type AdminWalletUseCase interface {
	// GelerWalletAdmin et DegelerWalletAdmin agissent sur n'importe quel
	// wallet du système et écrivent une entrée d'audit.
	GelerWalletAdmin(ctx context.Context, adminID, walletID string) (*commun.Wallet, error)
	DegelerWalletAdmin(ctx context.Context, adminID, walletID string) (*commun.Wallet, error)

	// ListerTransactionsAdmin liste les transactions tous wallets
	// confondus (voir ListerTransactions pour l'équivalent client).
	ListerTransactionsAdmin(ctx context.Context, filtre outputwallet.FiltreTransactions) ([]*wallet.Transaction, error)
}
