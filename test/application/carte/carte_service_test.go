package carte_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appcarte "raycard/internal/application/carte"
	domaincarte "raycard/internal/core/domain/carte"
	domaincommun "raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputcarte "raycard/internal/core/ports/input/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
	testcarte "raycard/test/application/carte"
	testcommun "raycard/test/application/commun"
	testwallet "raycard/test/application/wallet"
)

const utilisateurID = "user-1"

// nouvelUtilisateurTest crée un utilisateur au palier donné et l'enregistre
// dans le fake. tier2 détermine si l'utilisateur est passé au Tier 2
// (requis pour émettre une carte).
func nouvelUtilisateurTest(t *testing.T, utilisateurs *testcommun.UtilisateurRepoFake, tier2 bool) *domaincommun.Utilisateur {
	t.Helper()
	u, err := domaincommun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	u.ID = utilisateurID
	require.NoError(t, u.ValiderKycTier1())
	if tier2 {
		require.NoError(t, u.PasserAuTier2())
	}
	require.NoError(t, utilisateurs.Create(context.Background(), u))
	return u
}

func nouveauWalletTest(t *testing.T, wallets *testcommun.WalletRepoFake) *domaincommun.Wallet {
	t.Helper()
	w, err := domaincommun.NouveauWallet(utilisateurID, "CI", "XOF", 1_000_000)
	require.NoError(t, err)
	require.NoError(t, wallets.Create(context.Background(), w))
	return w
}

func crediterDisponible(t *testing.T, wallets *testcommun.WalletRepoFake, w *domaincommun.Wallet, montant int64) {
	t.Helper()
	require.NoError(t, w.Crediter(montant))
	require.NoError(t, wallets.UpdateSolde(context.Background(), w))
}

func nouveauService(
	utilisateurs *testcommun.UtilisateurRepoFake,
	wallets *testcommun.WalletRepoFake,
	transactions *testwallet.TransactionRepoFake,
	cartes *testcarte.CarteRepoFake,
	depenses *testcarte.DepenseCarteRepoFake,
	agregateur *testcarte.AgregateurCarteFake,
	notifieur *testcommun.NotifieurFake,
	auditLog *testcommun.AuditLogRepoFake,
) *carteServiceComplet {
	service := appcarte.NewCarteService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog, testcommun.TxManagerFake{})
	return &carteServiceComplet{
		CarteUseCase:      service,
		AdminCarteUseCase: service.(inputcarte.AdminCarteUseCase),
	}
}

// carteServiceComplet expose les deux visages de carteService (client et
// back-office) : NewCarteService ne renvoie que inputcarte.CarteUseCase,
// il faut une assertion de type pour accéder à AdminCarteUseCase (voir
// cmd/api/main.go, même principe).
type carteServiceComplet struct {
	inputcarte.CarteUseCase
	inputcarte.AdminCarteUseCase
}

func TestCarteService_CreerCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-123"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	assert.Equal(t, domaincarte.StatutCarteActive, carteCreee.Statut)
	assert.Equal(t, "card-123", carteCreee.IDExterne)
	assert.Equal(t, int64(10000), carteCreee.MontantChargeCentimes)
	assert.Equal(t, 1, agregateur.AppelsCreerCarte)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_CreerCarte_KycTierInsuffisant(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, false) // Tier 1 seulement
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrKycTierInsuffisant)
	assert.Equal(t, 0, agregateur.AppelsCreerCarte, "l'agrégateur ne doit jamais être appelé si le palier est insuffisant")
}

func TestCarteService_CreerCarte_WalletGele(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, &testcarte.AgregateurCarteFake{}, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)
	w.Statut = domaincommun.StatutWalletGele

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrWalletGele)
}

func TestCarteService_CreerCarte_SoldeInsuffisant(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, &testcarte.AgregateurCarteFake{}, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 1000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrSoldeInsuffisant)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_CreerCarte_TransactionDejaEnCours(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, &testcarte.AgregateurCarteFake{}, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	// Une transaction wallet (recharge, retrait...) déjà en cours doit
	// aussi bloquer l'émission de carte : un seul mouvement de fonds à la
	// fois par wallet.
	enCours, err := domainwallet.NouvelleTransactionRecharge(w.ID, utilisateurID, "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)
	require.NoError(t, transactions.Create(context.Background(), enCours))

	_, err = service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domainwallet.ErrTransactionDejaEnCours)
}

func TestCarteService_CreerCarte_ErreurAgregateur_DebitResteApplique(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{ErreurEmission: errors.New("panne réseau")}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrEmissionEchouee)

	// Le débit reste appliqué : jamais remboursé automatiquement sur une
	// erreur ambiguë (même politique que pour un retrait).
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), walletMisAJour.SoldeDisponibleCentimes)

	enCours, err := transactions.FindEnCoursByWalletID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, domainwallet.StatutTransactionEnAttente, enCours.Statut)
	assert.Equal(t, domainwallet.TypeTransactionFinancementCarte, enCours.Type)
}

func TestCarteService_ListerCartes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-list-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	liste, err := service.ListerCartes(context.Background(), utilisateurID)
	require.NoError(t, err)
	require.Len(t, liste, 1)
	assert.Equal(t, "Carte courses", liste[0].Label)
}

func TestCarteService_ObtenirCarte(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-obtenir-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	carteObtenue, err := service.ObtenirCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, carteCreee.ID, carteObtenue.ID)
	assert.Equal(t, "Carte courses", carteObtenue.Label)

	t.Run("carte introuvable", func(t *testing.T) {
		_, err := service.ObtenirCarte(context.Background(), utilisateurID, "carte-inexistante")
		assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	})

	t.Run("carte d'un autre utilisateur", func(t *testing.T) {
		_, err := service.ObtenirCarte(context.Background(), "un-autre-utilisateur", carteCreee.ID)
		assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	})
}

func TestCarteService_SynchroniserSoldes_DetecteUneDepense(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-sync-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)
	require.Equal(t, int64(10000), carteCreee.SoldeCentimes)

	// L'utilisateur a dépensé 3000 sur la carte depuis l'émission : le
	// solde observé chez l'agrégateur a baissé d'autant.
	agregateur.SoldesParIDExterne = map[string]int64{"card-sync-1": 7000}

	n, err := service.SynchroniserSoldes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	carteMiseAJour, err := cartes.FindByID(context.Background(), carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(7000), carteMiseAJour.SoldeCentimes)

	liste, err := service.ListerDepenses(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	require.Len(t, liste, 1)
	assert.Equal(t, int64(3000), liste[0].MontantCentimes)
	assert.Equal(t, int64(10000), liste[0].SoldeAvantCentimes)
	assert.Equal(t, int64(7000), liste[0].SoldeApresCentimes)

	// Cashback : 3000 * 0,02% = 0,6, arrondi à 1 centime, crédité
	// immédiatement sur le wallet (20000 crédités - 10000 débités pour la
	// carte + 1 de cashback).
	assert.Equal(t, int64(1), liste[0].CashbackCentimes)
	walletMisAJour, err := wallets.FindByUtilisateurID(context.Background(), utilisateurID)
	require.NoError(t, err)
	assert.Equal(t, int64(10001), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_SynchroniserSoldes_SoldeStable_AucuneDepense(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-sync-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldesParIDExterne = map[string]int64{"card-sync-2": 10000}

	n, err := service.SynchroniserSoldes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)

	liste, err := service.ListerDepenses(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Empty(t, liste)
}

func TestCarteService_ListerDepenses_AutreUtilisateur_Introuvable(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-sync-3"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.ListerDepenses(context.Background(), "un-autre-utilisateur", carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
}

func TestCarteService_SynchroniserSoldes_DetecteGelDecideParAgregateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-sync-gel"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	// Cartevo gèle la carte de son côté (ex: fraude suspectée), sans
	// aucune action initiée depuis RAYCARD.
	agregateur.SoldesParIDExterne = map[string]int64{"card-sync-gel": 10000}
	agregateur.StatutsParIDExterne = map[string]domaincarte.StatutCarte{"card-sync-gel": domaincarte.StatutCarteGelee}

	n, err := service.SynchroniserSoldes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n, "un gel n'est pas une dépense")

	carteMiseAJour, err := cartes.FindByID(context.Background(), carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteGelee, carteMiseAJour.Statut)

	// Une fois gelée, la carte ne doit plus jamais être sondée
	// automatiquement (seule une action explicite de dégel la remet en
	// rotation) : le second passage ne doit même pas rappeler l'agrégateur.
	appelsAvant := agregateur.AppelsObtenirEtat
	n, err = service.SynchroniserSoldes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, appelsAvant, agregateur.AppelsObtenirEtat)
}

func TestCarteService_GelerCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-gel-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	carteGelee, err := service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteGelee, carteGelee.Statut)
	assert.Equal(t, 1, agregateur.AppelsGeler)

	// Persisté.
	carteRelue, err := cartes.FindByID(context.Background(), carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteGelee, carteRelue.Statut)
}

func TestCarteService_GelerCarte_DejaGelee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-gel-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrTransitionCarteInvalide)
	assert.Equal(t, 1, agregateur.AppelsGeler, "la validation locale doit bloquer avant tout second appel réseau")
}

func TestCarteService_GelerCarte_AutreUtilisateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-gel-3"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), "un-autre-utilisateur", carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	assert.Equal(t, 0, agregateur.AppelsGeler)
}

func TestCarteService_DegelerCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-degel-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	carteDegelee, err := service.DegelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteActive, carteDegelee.Statut)
	assert.Equal(t, 1, agregateur.AppelsDegeler)

	// Redevient éligible au sondage : ProchaineVerificationAt réinitialisé.
	agregateur.SoldesParIDExterne = map[string]int64{"card-degel-1": 10000}
	n, err := service.SynchroniserSoldes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, 1, agregateur.AppelsObtenirEtat)
}

func TestCarteService_DegelerCarte_PasGelee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-degel-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.DegelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrTransitionCarteInvalide)
}

func TestCarteService_RechargerCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-topup-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldeApresRecharge = 15000
	carteRechargee, err := service.RechargerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 5000,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(15000), carteRechargee.SoldeCentimes)
	assert.Equal(t, int64(15000), carteRechargee.MontantChargeCentimes)
	assert.Equal(t, 1, agregateur.AppelsRecharger)

	// Le débit du wallet a bien eu lieu (10000 à la création + 5000 ici).
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), walletMisAJour.SoldeDisponibleCentimes)

	// La carte reste disponible pour une seconde recharge (pas bloquée par
	// FindEnCoursByWalletID, la transaction précédente est déjà Succes).
	agregateur.SoldeApresRecharge = 17000
	_, err = service.RechargerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 2000,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, agregateur.AppelsRecharger)
}

func TestCarteService_RechargerCarte_CarteGelee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-topup-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	_, err = service.RechargerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 2000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrTransitionCarteInvalide)
	assert.Equal(t, 0, agregateur.AppelsRecharger, "aucun appel réseau si la carte n'est pas active")
}

func TestCarteService_RechargerCarte_SoldeInsuffisant(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-topup-3"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 10000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.RechargerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 5000,
	})
	assert.ErrorIs(t, err, domaincommun.ErrSoldeInsuffisant)
	assert.Equal(t, 0, agregateur.AppelsRecharger)
}

func TestCarteService_RechargerCarte_ErreurAgregateur_DebitResteApplique(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-topup-4"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.ErreurRecharge = errors.New("panne réseau")
	_, err = service.RechargerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 2000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrRechargeEchouee)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(8000), walletMisAJour.SoldeDisponibleCentimes, "le débit reste appliqué malgré l'échec de l'agrégateur")

	enCours, err := transactions.FindEnCoursByWalletID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, domainwallet.TypeTransactionFinancementCarte, enCours.Type)
}

func TestCarteService_RechargerCarte_AutreUtilisateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-topup-5"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.RechargerCarte(context.Background(), "un-autre-utilisateur", carteCreee.ID, inputcarte.RechargerCarteRequest{
		MontantCentimes: 2000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	assert.Equal(t, 0, agregateur.AppelsRecharger)
}

func TestCarteService_AnnulerCarte_AvecSoldeRestant_Rembourse(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldeRestantAnnule = 6000
	carteAnnulee, err := service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteAnnulee, carteAnnulee.Statut)
	assert.Equal(t, int64(0), carteAnnulee.SoldeCentimes)
	assert.Equal(t, 1, agregateur.AppelsAnnuler)

	// 20000 - 10000 (financement) + 6000 (remboursement) = 16000
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(16000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_AnnulerCarte_SansSoldeRestant_AucunRemboursement(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldeRestantAnnule = 0
	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	// Aucun remboursement : le solde reste à 10000 (20000 - 10000 financement).
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_AnnulerCarte_DepuisGelee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-3"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	agregateur.SoldeRestantAnnule = 10000
	carteAnnulee, err := service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteAnnulee, carteAnnulee.Statut)
}

func TestCarteService_AnnulerCarte_DejaAnnulee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-4"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldeRestantAnnule = 10000
	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrTransitionCarteInvalide)
	assert.Equal(t, 1, agregateur.AppelsAnnuler, "aucun second appel réseau pour une carte déjà annulée")
}

func TestCarteService_AnnulerCarte_ErreurAgregateur_RienNeChange(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-5"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.ErreurAnnulation = errors.New("panne réseau")
	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrAnnulationEchouee)

	// La carte n'a pas bougé : l'appel à l'agrégateur a échoué avant toute
	// mutation locale.
	carteInchangee, err := cartes.FindByID(context.Background(), carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteActive, carteInchangee.Statut)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(10000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_AnnulerCarte_AutreUtilisateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-6"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.AnnulerCarte(context.Background(), "un-autre-utilisateur", carteCreee.ID)
	assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	assert.Equal(t, 0, agregateur.AppelsAnnuler)
}

func TestCarteService_GelerCarteAdmin_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-admin-gel-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	// Contrairement à GelerCarte, aucune vérification de propriétaire :
	// "un-autre-utilisateur" n'est jamais le propriétaire, l'action réussit
	// quand même.
	carteGelee, err := service.GelerCarteAdmin(context.Background(), "admin-1", carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteGelee, carteGelee.Statut)
	assert.Equal(t, 1, agregateur.AppelsGeler)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "admin-1", auditLog.Entrees[0].AdminID)
	assert.Equal(t, "carte_gelee_admin", auditLog.Entrees[0].Action)
	assert.Equal(t, carteCreee.ID, auditLog.Entrees[0].CibleID)
}

func TestCarteService_DegelerCarteAdmin_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-admin-degel-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)
	_, err = service.GelerCarteAdmin(context.Background(), "admin-1", carteCreee.ID)
	require.NoError(t, err)

	carteDegelee, err := service.DegelerCarteAdmin(context.Background(), "admin-1", carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteActive, carteDegelee.Statut)

	require.Len(t, auditLog.Entrees, 2)
	assert.Equal(t, "carte_degelee_admin", auditLog.Entrees[1].Action)
}

func TestCarteService_AnnulerCarteAdmin_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-admin-annuler-1", SoldeRestantAnnule: 4000}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	carteAnnulee, err := service.AnnulerCarteAdmin(context.Background(), "admin-1", carteCreee.ID)
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteAnnulee, carteAnnulee.Statut)

	// Remboursement appliqué comme pour l'annulation client (même logique
	// partagée, voir carteService.annulerCarte).
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(14000), walletMisAJour.SoldeDisponibleCentimes)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "carte_annulee_admin", auditLog.Entrees[0].Action)
}

func TestCarteService_ListerCartesAdmin_FiltreParUtilisateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-admin-liste-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	toutes, err := service.ListerCartesAdmin(context.Background(), outputcarte.FiltreCartes{})
	require.NoError(t, err)
	assert.Len(t, toutes, 1)

	filtrees, err := service.ListerCartesAdmin(context.Background(), outputcarte.FiltreCartes{UtilisateurID: "un-autre-utilisateur"})
	require.NoError(t, err)
	assert.Empty(t, filtrees)
}
