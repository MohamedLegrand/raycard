package wallet_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appwallet "raycard/internal/application/wallet"
	domaincommun "raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputwallet "raycard/internal/core/ports/input/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
	testcommun "raycard/test/application/commun"
	testwallet "raycard/test/application/wallet"
)

const utilisateurID = "user-1"

func nouveauWalletTest(t *testing.T, wallets *testcommun.WalletRepoFake) *domaincommun.Wallet {
	t.Helper()
	w, err := domaincommun.NouveauWallet(utilisateurID, "CI", "XOF", 1_000_000)
	require.NoError(t, err)
	require.NoError(t, wallets.Create(context.Background(), w))
	return w
}

func nouveauService(wallets *testcommun.WalletRepoFake, transactions *testwallet.TransactionRepoFake, agregateur *testwallet.AgregateurPaiementFake) inputwallet.WalletUseCase {
	return appwallet.NewWalletService(wallets, transactions, agregateur, testcommun.TxManagerFake{})
}

func TestWalletService_InitierRecharge_Succes(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-123"}
	service := nouveauService(wallets, transactions, agregateur)
	nouveauWalletTest(t, wallets)

	transaction, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur:       "ORANGE",
		Telephone:       "+2250700000000",
		MontantCentimes: 5000,
	})
	require.NoError(t, err)

	assert.Equal(t, domainwallet.StatutTransactionEnvoyee, transaction.Statut)
	assert.Equal(t, "ref-123", transaction.ReferenceExterne)
	assert.Equal(t, 1, agregateur.AppelsInitierCashIn)
}

func TestWalletService_InitierRecharge_WalletIntrouvable(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	service := nouveauService(wallets, transactions, &testwallet.AgregateurPaiementFake{})

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrWalletIntrouvable)
}

func TestWalletService_InitierRecharge_WalletGele(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	service := nouveauService(wallets, transactions, &testwallet.AgregateurPaiementFake{})
	w := nouveauWalletTest(t, wallets)
	w.Statut = domaincommun.StatutWalletGele

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrWalletGele)
}

func TestWalletService_InitierRecharge_DejaEnCours(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	req := inputwallet.InitierRechargeRequest{Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000}
	_, err := service.InitierRecharge(context.Background(), utilisateurID, req)
	require.NoError(t, err)

	_, err = service.InitierRecharge(context.Background(), utilisateurID, req)
	assert.ErrorIs(t, err, domainwallet.ErrTransactionDejaEnCours)
	assert.Equal(t, 1, agregateur.AppelsInitierCashIn, "le second appel ne doit jamais atteindre l'agrégateur")
	_ = w
}

func TestWalletService_InitierRecharge_ErreurAgregateur(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ErreurInitiation: errors.New("panne réseau")}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.Error(t, err)

	// La transaction reste EnAttente : jamais rappelée automatiquement, mais
	// elle bloque bien une nouvelle tentative tant qu'elle n'est pas résolue.
	enCours, err := transactions.FindEnCoursByWalletID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionEnAttente, enCours.Statut)
}

func TestWalletService_TraiterWebhook_PaiementReussi(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-succes"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type:             outputwallet.EvenementPaiementReussi,
		ReferenceExterne: "ref-succes",
		FraisCentimes:    75,
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	transaction, err := transactions.FindByReferenceExterne(context.Background(), "ref-succes")
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionSucces, transaction.Statut)
	assert.Equal(t, int64(75), transaction.FraisCentimes)
	require.NotNil(t, transaction.DisponibleLe)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(4925), walletMisAJour.SoldeEnAttenteCentimes)
	assert.Equal(t, int64(0), walletMisAJour.SoldeDisponibleCentimes)
}

func TestWalletService_TraiterWebhook_PaiementEchoue(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-echec"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type:             outputwallet.EvenementPaiementEchoue,
		ReferenceExterne: "ref-echec",
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	transaction, err := transactions.FindByReferenceExterne(context.Background(), "ref-echec")
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionEchouee, transaction.Statut)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), walletMisAJour.SoldeTotalCentimes())
}

func TestWalletService_TraiterWebhook_Rejoue(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-rejoue"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type: outputwallet.EvenementPaiementReussi, ReferenceExterne: "ref-rejoue", FraisCentimes: 75,
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	// Webhook rejoué (retry côté agrégateur) : ne doit pas créditer une
	// seconde fois.
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(4925), walletMisAJour.SoldeEnAttenteCentimes)
}

func TestWalletService_TraiterWebhook_SignatureInvalide(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ErreurWebhook: domainwallet.ErrWebhookSignatureInvalide}
	service := nouveauService(wallets, transactions, agregateur)

	err := service.TraiterWebhook(context.Background(), []byte(`{}`), "signature-invalide")
	assert.ErrorIs(t, err, domainwallet.ErrWebhookSignatureInvalide)
}

func TestWalletService_BasculerFondsEcheus(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-echu"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type: outputwallet.EvenementPaiementReussi, ReferenceExterne: "ref-echu", FraisCentimes: 75,
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	// Force l'échéance dans le passé (le webhook vient de la fixer à +48h).
	transaction, err := transactions.FindByReferenceExterne(context.Background(), "ref-echu")
	require.NoError(t, err)
	passe := time.Now().UTC().Add(-time.Minute)
	transaction.DisponibleLe = &passe
	require.NoError(t, transactions.Update(context.Background(), transaction))

	n, err := service.BasculerFondsEcheus(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), walletMisAJour.SoldeEnAttenteCentimes)
	assert.Equal(t, int64(4925), walletMisAJour.SoldeDisponibleCentimes)

	transactionApres, err := transactions.FindByID(context.Background(), transaction.ID)
	require.NoError(t, err)
	assert.Nil(t, transactionApres.DisponibleLe)
}

func crediterDisponible(t *testing.T, wallets *testcommun.WalletRepoFake, w *domaincommun.Wallet, montant int64) {
	t.Helper()
	require.NoError(t, w.Crediter(montant))
	require.NoError(t, wallets.UpdateSolde(context.Background(), w))
}

func TestWalletService_InitierRetrait_Succes(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-1"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 10000)

	transaction, err := service.InitierRetrait(context.Background(), utilisateurID, inputwallet.InitierRetraitRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 4000,
	})
	require.NoError(t, err)

	assert.Equal(t, domainwallet.StatutTransactionEnvoyee, transaction.Statut)
	assert.Equal(t, domainwallet.TypeTransactionRetrait, transaction.Type)
	assert.Equal(t, 1, agregateur.AppelsInitierCashOut)

	// Le débit est appliqué immédiatement, avant toute confirmation.
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestWalletService_InitierRetrait_SoldeInsuffisant(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	service := nouveauService(wallets, transactions, &testwallet.AgregateurPaiementFake{})
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 1000)

	_, err := service.InitierRetrait(context.Background(), utilisateurID, inputwallet.InitierRetraitRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrSoldeInsuffisant)

	// Rien n'a dû être débité ni aucune transaction créée.
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), walletMisAJour.SoldeDisponibleCentimes)
	_, err = transactions.FindEnCoursByWalletID(context.Background(), w.ID)
	assert.ErrorIs(t, err, domainwallet.ErrTransactionIntrouvable)
}

func TestWalletService_InitierRetrait_ErreurAgregateur_DebitResteApplique(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ErreurInitiation: errors.New("panne réseau")}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 10000)

	_, err := service.InitierRetrait(context.Background(), utilisateurID, inputwallet.InitierRetraitRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 4000,
	})
	require.Error(t, err)

	// Le débit reste appliqué : jamais remboursé automatiquement sur une
	// erreur ambiguë (voir le commentaire dans InitierRetrait).
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), walletMisAJour.SoldeDisponibleCentimes)

	enCours, err := transactions.FindEnCoursByWalletID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionEnAttente, enCours.Statut)
}

func TestWalletService_TraiterWebhook_RetraitReussi_NeTouchePasLeWallet(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-succes"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 10000)

	_, err := service.InitierRetrait(context.Background(), utilisateurID, inputwallet.InitierRetraitRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 4000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type: outputwallet.EvenementPaiementReussi, ReferenceExterne: "ref-cashout-succes", FraisCentimes: 40,
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	transaction, err := transactions.FindByReferenceExterne(context.Background(), "ref-cashout-succes")
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionSucces, transaction.Statut)
	assert.Equal(t, int64(40), transaction.FraisCentimes)

	// Le solde ne bouge plus : il a déjà été débité à l'initiation.
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestWalletService_TraiterWebhook_RetraitEchoue_Rembourse(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-echec"}
	service := nouveauService(wallets, transactions, agregateur)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 10000)

	_, err := service.InitierRetrait(context.Background(), utilisateurID, inputwallet.InitierRetraitRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 4000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type: outputwallet.EvenementPaiementEchoue, ReferenceExterne: "ref-cashout-echec",
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	transaction, err := transactions.FindByReferenceExterne(context.Background(), "ref-cashout-echec")
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionEchouee, transaction.Statut)

	// Le montant débité à l'initiation est intégralement remboursé.
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestWalletService_ListerTransactions(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	agregateur := &testwallet.AgregateurPaiementFake{}
	service := nouveauService(wallets, transactions, agregateur)
	nouveauWalletTest(t, wallets)

	agregateur.ReferenceGeneree = "ref-histo-1"
	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	agregateur.EvenementARenvoyer = &outputwallet.EvenementWebhook{
		Type: outputwallet.EvenementPaiementEchoue, ReferenceExterne: "ref-histo-1",
	}
	require.NoError(t, service.TraiterWebhook(context.Background(), []byte(`{}`), "signature"))

	agregateur.ReferenceGeneree = "ref-histo-2"
	agregateur.EvenementARenvoyer = nil
	_, err = service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "MTN", Telephone: "+2250700000001", MontantCentimes: 3000,
	})
	require.NoError(t, err)

	liste, err := service.ListerTransactions(context.Background(), utilisateurID)
	require.NoError(t, err)
	require.Len(t, liste, 2)
	// Les deux transactions appartiennent bien au même wallet.
	references := []string{liste[0].ReferenceExterne, liste[1].ReferenceExterne}
	assert.ElementsMatch(t, []string{"ref-histo-1", "ref-histo-2"}, references)
}

func TestWalletService_ListerTransactions_WalletIntrouvable(t *testing.T) {
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	service := nouveauService(wallets, transactions, &testwallet.AgregateurPaiementFake{})

	_, err := service.ListerTransactions(context.Background(), utilisateurID)
	assert.ErrorIs(t, err, domaincommun.ErrWalletIntrouvable)
}
