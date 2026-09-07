package carte_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appcarte "raycard/internal/application/carte"
	domaincarte "raycard/internal/core/domain/carte"
	domaincommun "raycard/internal/core/domain/commun"
	domainkyc "raycard/internal/core/domain/kyc"
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
// (requis pour émettre une carte). cardCustomers, si non nil et tier2
// vrai, enrôle aussitôt l'utilisateur comme porteur de carte déjà validé
// — la quasi-totalité des tests de ce fichier portent sur le
// débit/l'appel agrégateur, pas sur le KYC porteur de carte lui-même
// (voir TestCarteService_CreerCarte_PorteurNonEnrole et
// TestCarteService_SoumettrePorteurCarte_* pour ces cas précis, qui
// passent nil ici).
func nouvelUtilisateurTest(t *testing.T, utilisateurs *testcommun.UtilisateurRepoFake, tier2 bool, cardCustomers *testcarte.CardCustomerRepoFake) *domaincommun.Utilisateur {
	t.Helper()
	u, err := domaincommun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	u.ID = utilisateurID
	require.NoError(t, u.ValiderKycTier1())
	if tier2 {
		require.NoError(t, u.PasserAuTier2())
	}
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	if tier2 && cardCustomers != nil {
		porteur, err := domaincarte.NouveauCardCustomer(u.ID)
		require.NoError(t, err)
		require.NoError(t, porteur.MarquerEnrole("cust-fake-1", time.Now().UTC()))
		require.NoError(t, cardCustomers.Create(context.Background(), porteur))
	}
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
	cardCustomers := testcarte.NewCardCustomerRepoFake()
	dossiersKyc := testcarte.NewDossierKycRepoFake()
	documentsKyc := testcarte.NewDocumentKycRepoFake()
	service := appcarte.NewCarteService(
		utilisateurs, wallets, transactions, cartes, depenses, cardCustomers,
		dossiersKyc, documentsKyc, testcommun.StockageFichierFake{},
		agregateur, notifieur, auditLog, testcommun.TxManagerFake{},
	)
	return &carteServiceComplet{
		CarteUseCase:      service,
		AdminCarteUseCase: service.(inputcarte.AdminCarteUseCase),
		CardCustomers:     cardCustomers,
		DossiersKyc:        dossiersKyc,
		DocumentsKyc:       documentsKyc,
	}
}

// carteServiceComplet expose les deux visages de carteService (client et
// back-office) : NewCarteService ne renvoie que inputcarte.CarteUseCase,
// il faut une assertion de type pour accéder à AdminCarteUseCase (voir
// cmd/api/main.go, même principe). Les fakes KYC/porteur de carte sont
// exposés pour les tests qui ont besoin d'un contrôle fin (voir
// nouvelUtilisateurTest et TestCarteService_SoumettrePorteurCarte_*).
type carteServiceComplet struct {
	inputcarte.CarteUseCase
	inputcarte.AdminCarteUseCase
	CardCustomers *testcarte.CardCustomerRepoFake
	DossiersKyc   *testcarte.DossierKycRepoFake
	DocumentsKyc  *testcarte.DocumentKycRepoFake
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

func TestCarteService_CreerCarte_PorteurNonSoumis(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	// Tier 2 mais jamais passé par SoumettrePorteurCarte : aucun dossier
	// de porteur de carte n'existe (cardCustomers = nil ci-dessous).
	nouvelUtilisateurTest(t, utilisateurs, true, nil)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	_, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrCardCustomerNonEnrole)
	assert.Equal(t, 0, agregateur.AppelsCreerCarte, "jamais d'appel agrégateur sans porteur enrôlé")

	walletInchange, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(20000), walletInchange.SoldeDisponibleCentimes, "aucun débit tant que le porteur n'est pas enrôlé")
}

func TestCarteService_CreerCarte_PorteurEnAttente(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, nil)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	// Dossier soumis mais pas encore enrole (revue humaine en cours côté
	// agrégateur) : jamais NouveauCardCustomer + MarquerEnrole ici.
	porteur, err := domaincarte.NouveauCardCustomer(utilisateurID)
	require.NoError(t, err)
	require.NoError(t, service.CardCustomers.Create(context.Background(), porteur))

	_, err = service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrCardCustomerNonEnrole)
}

func TestCarteService_CreerCarte_PorteurRejete(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, nil)
	nouveauWalletTest(t, wallets)

	porteur, err := domaincarte.NouveauCardCustomer(utilisateurID)
	require.NoError(t, err)
	porteur.MarquerRejete("document illisible", true, time.Now().UTC())
	require.NoError(t, service.CardCustomers.Create(context.Background(), porteur))

	_, err = service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.ErrorIs(t, err, domaincarte.ErrCardCustomerRejete)
	assert.Contains(t, err.Error(), "document illisible")
}

// TestCarteService_CreerCarte_FinancementAutomatiqueApresSoldeInsuffisant
// vérifie le comportement borné décrit sur
// carteService.creerCarteAvecFinancementAutomatique : un premier échec
// ErrCardWalletInsuffisant déclenche un financement puis une seule
// nouvelle tentative, jamais une boucle.
func TestCarteService_CreerCarte_FinancementAutomatiqueApresSoldeInsuffisant(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{
		IDExterneGenere:   "card-financement-1",
		ErreurEmission:    domaincarte.ErrCardWalletInsuffisant,
		EchecsAvantSucces: 1,
	}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)
	assert.Equal(t, "card-financement-1", carteCreee.IDExterne)
	assert.Equal(t, 2, agregateur.AppelsCreerCarte, "un échec puis une réussite après financement")
	assert.Equal(t, 1, agregateur.AppelsAlimenter)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "portefeuille_cartes_alimente_auto", auditLog.Entrees[0].Action)
}

func TestCarteService_SoumettrePorteurCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneCustomer: "cust-soumis-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, nil)

	dossier, err := domainkyc.NouveauDossierKyc(utilisateurID)
	require.NoError(t, err)
	dossier.Statut = domainkyc.StatutDossierApprouve
	require.NoError(t, service.DossiersKyc.Create(context.Background(), dossier))
	recto, err := domainkyc.NouveauDocumentKyc(utilisateurID, dossier.ID, domainkyc.TypeDocumentRectoPieceIdentite, "recto.jpg", "/faux/recto.jpg", "")
	require.NoError(t, err)
	verso, err := domainkyc.NouveauDocumentKyc(utilisateurID, dossier.ID, domainkyc.TypeDocumentVersoPieceIdentite, "verso.jpg", "/faux/verso.jpg", "")
	require.NoError(t, err)
	require.NoError(t, service.DocumentsKyc.Create(context.Background(), recto))
	require.NoError(t, service.DocumentsKyc.Create(context.Background(), verso))

	porteur, err := service.SoumettrePorteurCarte(context.Background(), utilisateurID, inputcarte.SoumettrePorteurCarteRequest{
		PaysNomComplet: "Cameroon", PaysCodeISO: "CM", IndicatifPays: "+237", TelephoneLocal: "690001234",
		Rue: "Rue 1", Ville: "Douala", Region: "Littoral", CodePostal: "00237",
		NumeroIdentification: "123456789", TypeDocument: "NIN", DateNaissance: "1990-04-12",
	})
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCardCustomerEnAttente, porteur.Statut)
	assert.Equal(t, 1, agregateur.AppelsSoumettre)

	relu, err := service.ObtenirStatutPorteurCarte(context.Background(), utilisateurID)
	require.NoError(t, err)
	assert.Equal(t, porteur.ID, relu.ID)
}

func TestCarteService_SoumettrePorteurCarte_DejaSoumis(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)

	_, err := service.SoumettrePorteurCarte(context.Background(), utilisateurID, inputcarte.SoumettrePorteurCarteRequest{})
	assert.ErrorIs(t, err, domaincarte.ErrCardCustomerDejaSoumis)
	assert.Equal(t, 0, agregateur.AppelsSoumettre)
}

func TestCarteService_SoumettrePorteurCarte_SansDossierKycApprouve(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, nil)
	// Aucun dossier KYC créé du tout dans service.DossiersKyc.

	_, err := service.SoumettrePorteurCarte(context.Background(), utilisateurID, inputcarte.SoumettrePorteurCarteRequest{})
	assert.Error(t, err)
	assert.Equal(t, 0, agregateur.AppelsSoumettre, "jamais d'appel agrégateur sans pièces d'identité disponibles")
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

	nouvelUtilisateurTest(t, utilisateurs, false, nil) // Tier 1 seulement
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

// TestCarteService_AnnulerCarte_ConvertitEnXAFAvantRemboursement couvre un
// bug corrigé : le solde restant renvoyé par AnnulerCarte est en USD (voir
// le commentaire sur carte.Carte.Devise), et était auparavant crédité tel
// quel dans le wallet XAF de l'utilisateur, sans reconversion — un
// TauxConversionInverse différent de 1 ici (comme un vrai taux XAF/USD)
// prouve que le montant crédité est bien reconverti, jamais le nombre brut
// de centimes de dollar renvoyé par l'agrégateur.
func TestCarteService_AnnulerCarte_ConvertitEnXAFAvantRemboursement(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-annuler-conversion"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	// 100 "centimes de dollar" restants, reconvertis à un taux de 6.49 (un
	// taux XAF/USD réaliste, à cette échelle) : sans la reconversion, le
	// wallet ne serait crédité que de 100 (comme s'il s'agissait déjà de
	// XAF).
	agregateur.SoldeRestantAnnule = 100
	agregateur.TauxConversionInverse = 6.49
	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	// 20000 - 10000 (financement) + 100*6.49 arrondi à 649 (remboursement
	// reconverti) = 10649
	assert.Equal(t, int64(10_649), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_RetirerCarte_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-retrait-1"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	// TauxConversion (XAF->USD) et TauxConversionInverse (USD->XAF) à 1 :
	// un retrait de 3000 XAF redemande 3000 "USD" à l'agrégateur, qui les
	// crédite en totalité (pas de frais simulé), reconvertis 1:1.
	agregateur.SoldeApresRetrait = 7000
	carteApresRetrait, err := service.RetirerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RetirerCarteRequest{
		MontantCentimes: 3000,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(7000), carteApresRetrait.SoldeCentimes)
	// Contrairement à Recharger, MontantChargeCentimes ne bouge jamais.
	assert.Equal(t, int64(10000), carteApresRetrait.MontantChargeCentimes)
	assert.Equal(t, domaincarte.StatutCarteActive, carteApresRetrait.Statut)
	assert.Equal(t, 1, agregateur.AppelsRetirer)

	// 20000 - 10000 (financement) + 3000 (retrait recrédité) = 13000
	walletMisAJour, err := wallets.FindByID(context.Background(), w.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(13000), walletMisAJour.SoldeDisponibleCentimes)
}

func TestCarteService_RetirerCarte_SoldeCarteInsuffisant(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-retrait-2"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.RetirerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RetirerCarteRequest{
		MontantCentimes: 50000, // largement au-delà du solde carte connu (10000)
	})
	assert.ErrorIs(t, err, domaincarte.ErrSoldeCarteInsuffisant)
	assert.Equal(t, 0, agregateur.AppelsRetirer, "aucun appel réseau si le solde carte connu est visiblement insuffisant")
}

func TestCarteService_RetirerCarte_CarteAnnulee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-retrait-3"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	agregateur.SoldeRestantAnnule = 0
	_, err = service.AnnulerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	_, err = service.RetirerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RetirerCarteRequest{
		MontantCentimes: 1000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrTransitionCarteInvalide)
	assert.Equal(t, 0, agregateur.AppelsRetirer)
}

func TestCarteService_RetirerCarte_DepuisGelee(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-retrait-4"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.GelerCarte(context.Background(), utilisateurID, carteCreee.ID)
	require.NoError(t, err)

	// Contrairement à RechargerCarte (réservé aux cartes actives), un
	// retrait reste possible sur une carte gelée : geler bloque la
	// dépense, pas la récupération du solde déjà présent.
	agregateur.SoldeApresRetrait = 8000
	carteApresRetrait, err := service.RetirerCarte(context.Background(), utilisateurID, carteCreee.ID, inputcarte.RetirerCarteRequest{
		MontantCentimes: 2000,
	})
	require.NoError(t, err)
	assert.Equal(t, domaincarte.StatutCarteGelee, carteApresRetrait.Statut)
	assert.Equal(t, int64(8000), carteApresRetrait.SoldeCentimes)
}

func TestCarteService_RetirerCarte_AutreUtilisateur(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	transactions := testwallet.NewTransactionRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	depenses := testcarte.NewDepenseCarteRepoFake()
	notifieur := &testcommun.NotifieurFake{}
	auditLog := &testcommun.AuditLogRepoFake{}
	agregateur := &testcarte.AgregateurCarteFake{IDExterneGenere: "card-retrait-5"}
	service := nouveauService(utilisateurs, wallets, transactions, cartes, depenses, agregateur, notifieur, auditLog)

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
	w := nouveauWalletTest(t, wallets)
	crediterDisponible(t, wallets, w, 20000)

	carteCreee, err := service.CreerCarte(context.Background(), utilisateurID, inputcarte.CreerCarteRequest{
		Label: "Carte courses", MontantCentimes: 10000,
	})
	require.NoError(t, err)

	_, err = service.RetirerCarte(context.Background(), "un-autre-utilisateur", carteCreee.ID, inputcarte.RetirerCarteRequest{
		MontantCentimes: 1000,
	})
	assert.ErrorIs(t, err, domaincarte.ErrCarteIntrouvable)
	assert.Equal(t, 0, agregateur.AppelsRetirer)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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

	nouvelUtilisateurTest(t, utilisateurs, true, service.CardCustomers)
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
