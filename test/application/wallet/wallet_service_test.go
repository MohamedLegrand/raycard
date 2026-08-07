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

func nouveauWalletTest(t *testing.T, utilisateurs *testcommun.UtilisateurRepoFake, wallets *testcommun.WalletRepoFake) *domaincommun.Wallet {
	t.Helper()
	require.NoError(t, utilisateurs.Create(context.Background(), &domaincommun.Utilisateur{
		ID: utilisateurID, Email: "user-1@example.com",
	}))
	w, err := domaincommun.NouveauWallet(utilisateurID, "CI", "XOF", 1_000_000)
	require.NoError(t, err)
	require.NoError(t, wallets.Create(context.Background(), w))
	return w
}

func nouveauService(utilisateurs *testcommun.UtilisateurRepoFake, wallets *testcommun.WalletRepoFake, transactions *testwallet.TransactionRepoFake, agregateur *testwallet.AgregateurPaiementFake, notifieur *testcommun.NotifieurFake, auditLog *testcommun.AuditLogRepoFake) *walletServiceComplet {
	service := appwallet.NewWalletService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog, testcommun.TxManagerFake{})
	return &walletServiceComplet{
		WalletUseCase:      service,
		AdminWalletUseCase: service.(inputwallet.AdminWalletUseCase),
	}
}

// walletServiceComplet expose les deux visages de walletService (client
// et back-office) : NewWalletService ne renvoie que
// inputwallet.WalletUseCase, il faut une assertion de type pour accéder
// à AdminWalletUseCase (voir cmd/api/main.go, même principe).
type walletServiceComplet struct {
	inputwallet.WalletUseCase
	inputwallet.AdminWalletUseCase
}

func TestWalletService_InitierRecharge_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-123"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrWalletIntrouvable)
}

func TestWalletService_InitierRecharge_WalletGele(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
	w.Statut = domaincommun.StatutWalletGele

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrWalletGele)
}

func TestWalletService_InitierRecharge_DejaEnCours(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

	req := inputwallet.InitierRechargeRequest{Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000}
	_, err := service.InitierRecharge(context.Background(), utilisateurID, req)
	require.NoError(t, err)

	_, err = service.InitierRecharge(context.Background(), utilisateurID, req)
	assert.ErrorIs(t, err, domainwallet.ErrTransactionDejaEnCours)
	assert.Equal(t, 1, agregateur.AppelsInitierCashIn, "le second appel ne doit jamais atteindre l'agrégateur")
	_ = w
}

func TestWalletService_InitierRecharge_ErreurAgregateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ErreurInitiation: errors.New("panne réseau")}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-succes"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-echec"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-rejoue"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ErreurWebhook: domainwallet.ErrWebhookSignatureInvalide}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)

	err := service.TraiterWebhook(context.Background(), []byte(`{}`), "signature-invalide")
	assert.ErrorIs(t, err, domainwallet.ErrWebhookSignatureInvalide)
}

func TestWalletService_BasculerFondsEcheus(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-echu"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-1"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ErreurInitiation: errors.New("panne réseau")}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-succes"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-cashout-echec"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)
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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	nouveauWalletTest(t, utilisateurs, wallets)

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
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)

	_, err := service.ListerTransactions(context.Background(), utilisateurID)
	assert.ErrorIs(t, err, domaincommun.ErrWalletIntrouvable)
}

func TestWalletService_GelerWalletAdmin_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

	walletGele, err := service.GelerWalletAdmin(context.Background(), "admin-1", w.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincommun.StatutWalletGele, walletGele.Statut)

	// Persisté.
	walletRelu, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincommun.StatutWalletGele, walletRelu.Statut)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "admin-1", auditLog.Entrees[0].AdminID)
	assert.Equal(t, "wallet_gele_admin", auditLog.Entrees[0].Action)
	assert.Equal(t, w.ID, auditLog.Entrees[0].CibleID)
}

func TestWalletService_GelerWalletAdmin_DejaGele(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

	_, err := service.GelerWalletAdmin(context.Background(), "admin-1", w.ID)
	require.NoError(t, err)

	_, err = service.GelerWalletAdmin(context.Background(), "admin-1", w.ID)
	assert.ErrorIs(t, err, domaincommun.ErrTransitionWalletInvalide)
}

func TestWalletService_DegelerWalletAdmin_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, &testwallet.AgregateurPaiementFake{}, notifieur, auditLog)
	w := nouveauWalletTest(t, utilisateurs, wallets)

	_, err := service.GelerWalletAdmin(context.Background(), "admin-1", w.ID)
	require.NoError(t, err)

	walletDegele, err := service.DegelerWalletAdmin(context.Background(), "admin-1", w.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincommun.StatutWalletActif, walletDegele.Statut)

	require.Len(t, auditLog.Entrees, 2)
	assert.Equal(t, "wallet_degele_admin", auditLog.Entrees[1].Action)
}

func TestWalletService_ListerTransactionsAdmin_Filtres(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testwallet.AgregateurPaiementFake{ReferenceGeneree: "ref-admin-liste-1"}
	service := nouveauService(utilisateurs, wallets, transactions, agregateur, notifieur, auditLog)
	nouveauWalletTest(t, utilisateurs, wallets)

	_, err := service.InitierRecharge(context.Background(), utilisateurID, inputwallet.InitierRechargeRequest{
		Operateur: "ORANGE", Telephone: "+2250700000000", MontantCentimes: 5000,
	})
	require.NoError(t, err)

	toutes, err := service.ListerTransactionsAdmin(context.Background(), outputwallet.FiltreTransactions{})
	require.NoError(t, err)
	assert.Len(t, toutes, 1)

	filtrees, err := service.ListerTransactionsAdmin(context.Background(), outputwallet.FiltreTransactions{UtilisateurID: "un-autre-utilisateur"})
	require.NoError(t, err)
	assert.Empty(t, filtrees)
}
