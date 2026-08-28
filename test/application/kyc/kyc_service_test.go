package kyc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appkyc "raycard/internal/application/kyc"
	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
	testcommun "raycard/test/application/commun"
)

// --- faux repository en mémoire, propre au module kyc ---

type dossierKycRepoFake struct {
	parID                   map[string]*kyc.DossierKyc
	enAttenteParUtilisateur map[string]*kyc.DossierKyc
}

func nouveauDossierKycRepoFake() *dossierKycRepoFake {
	return &dossierKycRepoFake{
		parID:                   make(map[string]*kyc.DossierKyc),
		enAttenteParUtilisateur: make(map[string]*kyc.DossierKyc),
	}
}

func (r *dossierKycRepoFake) Create(_ context.Context, d *kyc.DossierKyc) error {
	r.parID[d.ID] = d
	if d.Statut == kyc.StatutDossierEnAttente {
		r.enAttenteParUtilisateur[d.UtilisateurID] = d
	}
	return nil
}

func (r *dossierKycRepoFake) FindByID(_ context.Context, id string) (*kyc.DossierKyc, error) {
	if d, ok := r.parID[id]; ok {
		return d, nil
	}
	return nil, kyc.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) FindEnAttenteByUtilisateurID(_ context.Context, utilisateurID string) (*kyc.DossierKyc, error) {
	if d, ok := r.enAttenteParUtilisateur[utilisateurID]; ok {
		return d, nil
	}
	return nil, kyc.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) FindDernierByUtilisateurID(_ context.Context, utilisateurID string) (*kyc.DossierKyc, error) {
	var dernier *kyc.DossierKyc
	for _, d := range r.parID {
		if d.UtilisateurID == utilisateurID {
			if dernier == nil || d.CreatedAt.After(dernier.CreatedAt) {
				dernier = d
			}
		}
	}
	if dernier != nil {
		return dernier, nil
	}
	return nil, kyc.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) ListEnAttente(_ context.Context) ([]*kyc.DossierKyc, error) {
	var resultat []*kyc.DossierKyc
	for _, d := range r.parID {
		if d.Statut == kyc.StatutDossierEnAttente {
			resultat = append(resultat, d)
		}
	}
	return resultat, nil
}

func (r *dossierKycRepoFake) ListAll(_ context.Context) ([]*kyc.DossierKyc, error) {
	var resultat []*kyc.DossierKyc
	for _, d := range r.parID {
		resultat = append(resultat, d)
	}
	return resultat, nil
}

func (r *dossierKycRepoFake) Update(_ context.Context, d *kyc.DossierKyc) error {
	if _, ok := r.parID[d.ID]; !ok {
		return kyc.ErrDossierKycIntrouvable
	}
	r.parID[d.ID] = d
	if d.Statut != kyc.StatutDossierEnAttente {
		delete(r.enAttenteParUtilisateur, d.UtilisateurID)
	}
	return nil
}

type documentKycRepoFake struct {
	parDossierID map[string][]*kyc.DocumentKyc
}

func nouveauDocumentKycRepoFake() *documentKycRepoFake {
	return &documentKycRepoFake{parDossierID: make(map[string][]*kyc.DocumentKyc)}
}

func (r *documentKycRepoFake) Create(_ context.Context, d *kyc.DocumentKyc) error {
	r.parDossierID[d.DossierKycID] = append(r.parDossierID[d.DossierKycID], d)
	return nil
}

func (r *documentKycRepoFake) FindByID(_ context.Context, id string) (*kyc.DocumentKyc, error) {
	for _, documents := range r.parDossierID {
		for _, d := range documents {
			if d.ID == id {
				return d, nil
			}
		}
	}
	return nil, kyc.ErrDocumentKycIntrouvable
}

func (r *documentKycRepoFake) ListByDossierKycID(_ context.Context, dossierKycID string) ([]*kyc.DocumentKyc, error) {
	return r.parDossierID[dossierKycID], nil
}

// ocrExtracteurFake simule Tesseract sans dépendre du binaire réel.
type ocrExtracteurFake struct{}

func (ocrExtracteurFake) ExtraireTexte(_ context.Context, _ string) (string, error) {
	return "NOM: KONE\nPRENOM: AWA", nil
}

// jpegFactice : préfixe suffisant pour que http.DetectContentType
// reconnaisse un JPEG (FF D8 FF), sans avoir besoin d'une vraie image.
var jpegFactice = []byte{0xFF, 0xD8, 0xFF, 0x00}

func setupService() (inputkyc.KycUseCase, *testcommun.UtilisateurRepoFake, *testcommun.WalletRepoFake, *dossierKycRepoFake, *documentKycRepoFake) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	regles := testcommun.NewReglesKycRepoFake()
	dossiers := nouveauDossierKycRepoFake()
	documents := nouveauDocumentKycRepoFake()
	service := appkyc.NewKycService(
		utilisateurs, wallets, regles, dossiers, documents,
		testcommun.StockageFichierFake{}, ocrExtracteurFake{}, testcommun.TxManagerFake{},
	)
	return service, utilisateurs, wallets, dossiers, documents
}

func requeteValide() inputkyc.InscriptionRequest {
	return inputkyc.InscriptionRequest{
		Nom:        "Koné",
		Prenom:     "Awa",
		Email:      "awa@example.com",
		Telephone:  "+2250700000000",
		PaysCode:   "CI",
		MotDePasse: "motdepasse123",
	}
}

func TestKycService_Inscrire_Succes(t *testing.T) {
	service, utilisateurs, wallets, _, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	assert.Equal(t, commun.KycTier1, resultat.Utilisateur.KycTier)
	assert.Equal(t, commun.KycStatutVerifie, resultat.Utilisateur.KycStatut)
	assert.NotEqual(t, "motdepasse123", resultat.Utilisateur.MotDePasseHash, "le mot de passe doit être haché")

	assert.Equal(t, "XOF", resultat.Wallet.Devise)
	assert.Equal(t, int64(200000), resultat.Wallet.PlafondSoldeCentimes)
	assert.Equal(t, resultat.Utilisateur.ID, resultat.Wallet.UtilisateurID)

	// Persistance effective via les deux repositories, dans la même
	// "transaction" (fake).
	_, err = utilisateurs.FindByEmail(context.Background(), "awa@example.com")
	require.NoError(t, err)
	_, err = wallets.FindByUtilisateurID(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)
}

func TestKycService_Inscrire_EmailDejaUtilise(t *testing.T) {
	service, _, _, _, _ := setupService()

	_, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	_, err = service.Inscrire(context.Background(), requeteValide())
	assert.ErrorIs(t, err, commun.ErrEmailDejaUtilise)
}

func TestKycService_Inscrire_TelephoneDejaUtilise(t *testing.T) {
	service, _, _, _, _ := setupService()

	_, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	req := requeteValide()
	req.Email = "autre@example.com"
	_, err = service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, commun.ErrTelephoneDejaUtilise)
}

func TestKycService_Inscrire_PaysNonSupporte(t *testing.T) {
	service, _, _, _, _ := setupService()

	req := requeteValide()
	req.Email = "autre@example.com"
	req.Telephone = "+221700000000"
	req.PaysCode = "FR" // pas de règle Tier 1 pour FR dans le fake

	_, err := service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, commun.ErrPaysNonSupporte)
}

func TestKycService_DemanderTier2_Succes(t *testing.T) {
	service, _, _, dossiers, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	dossier, err := service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	assert.Equal(t, kyc.StatutDossierEnAttente, dossier.Statut)
	assert.Equal(t, commun.KycTier2, dossier.TierDemande)
	assert.Len(t, dossiers.parID, 1)
}

func TestKycService_DemanderTier2_DejaEnAttente(t *testing.T) {
	service, _, _, _, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	assert.ErrorIs(t, err, kyc.ErrDossierKycDejaEnAttente)
}

func TestKycService_DemanderTier2_PasEncoreTier1(t *testing.T) {
	service, utilisateurs, _, _, _ := setupService()

	// Un utilisateur qui n'a pas encore de wallet/tier (créé directement,
	// sans passer par Inscrire) ne peut pas demander le Tier 2.
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	_, err = service.DemanderTier2(context.Background(), u.ID)
	assert.ErrorIs(t, err, commun.ErrTransitionKycInvalide)
}

// creerUtilisateurEtDossier inscrit un utilisateur puis lui crée un
// dossier de passage de palier en attente, comme préalable commun aux
// tests de téléversement de document.
func creerUtilisateurEtDossier(t *testing.T, service inputkyc.KycUseCase) (utilisateurID, dossierID string) {
	t.Helper()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	dossier, err := service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	return resultat.Utilisateur.ID, dossier.ID
}

func TestKycService_TeleverserDocument_Succes(t *testing.T) {
	service, _, _, _, documents := setupService()
	utilisateurID, dossierID := creerUtilisateurEtDossier(t, service)

	document, err := service.TeleverserDocument(context.Background(), utilisateurID, inputkyc.TeleverserDocumentRequest{
		DossierKycID: dossierID,
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	require.NoError(t, err)

	assert.NotEmpty(t, document.ID)
	assert.Equal(t, "cni.jpg", document.NomFichier)
	assert.Equal(t, kyc.TypeDocumentRectoPieceIdentite, document.TypeDocument)
	assert.Equal(t, "NOM: KONE\nPRENOM: AWA", document.TexteExtrait)
	assert.Contains(t, document.CheminFichier, "cni.jpg")

	stockes, err := documents.ListByDossierKycID(context.Background(), dossierID)
	require.NoError(t, err)
	assert.Len(t, stockes, 1)
}

func TestKycService_TeleverserDocument_FormatInvalide(t *testing.T) {
	service, _, _, _, _ := setupService()

	// Le format est vérifié avant même la recherche du dossier : un
	// dossier bidon suffit ici.
	_, err := service.TeleverserDocument(context.Background(), "user-1", inputkyc.TeleverserDocumentRequest{
		DossierKycID: "dossier-peu-importe",
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.txt",
		Contenu:      []byte("pas une image"),
	})
	assert.ErrorIs(t, err, kyc.ErrFormatDocumentInvalide)
}

func TestKycService_TeleverserDocument_DossierIntrouvable(t *testing.T) {
	service, _, _, _, _ := setupService()

	_, err := service.TeleverserDocument(context.Background(), "user-1", inputkyc.TeleverserDocumentRequest{
		DossierKycID: "dossier-inconnu",
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	assert.ErrorIs(t, err, kyc.ErrDossierKycIntrouvable)
}

func TestKycService_TeleverserDocument_DossierDunAutreUtilisateur(t *testing.T) {
	service, _, _, _, _ := setupService()
	_, dossierID := creerUtilisateurEtDossier(t, service)

	_, err := service.TeleverserDocument(context.Background(), "un-autre-utilisateur", inputkyc.TeleverserDocumentRequest{
		DossierKycID: dossierID,
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	assert.ErrorIs(t, err, kyc.ErrDossierKycIntrouvable, "ne doit pas confirmer l'existence d'un dossier qui ne nous appartient pas")
}

func TestKycService_TeleverserDocument_DossierNonModifiable(t *testing.T) {
	service, _, _, dossiers, _ := setupService()
	utilisateurID, dossierID := creerUtilisateurEtDossier(t, service)

	// Simule un dossier déjà approuvé : plus question d'y ajouter une pièce.
	dossier, err := dossiers.FindByID(context.Background(), dossierID)
	require.NoError(t, err)
	require.NoError(t, dossier.Approuver("admin-1"))
	require.NoError(t, dossiers.Update(context.Background(), dossier))

	_, err = service.TeleverserDocument(context.Background(), utilisateurID, inputkyc.TeleverserDocumentRequest{
		DossierKycID: dossierID,
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	assert.ErrorIs(t, err, kyc.ErrDossierKycNonModifiable)
}

func TestKycService_TeleverserDocument_TypeDocumentInvalide(t *testing.T) {
	service, _, _, _, _ := setupService()
	utilisateurID, dossierID := creerUtilisateurEtDossier(t, service)

	_, err := service.TeleverserDocument(context.Background(), utilisateurID, inputkyc.TeleverserDocumentRequest{
		DossierKycID: dossierID,
		TypeDocument: kyc.TypeDocument("type-inconnu"),
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
}

// ocrEnErreurFake simule une panne du moteur OCR (ex: tesseract non
// installé) : le document doit quand même être stocké, l'administrateur
// pourra toujours l'examiner visuellement pendant la revue manuelle.
type ocrEnErreurFake struct{}

func (ocrEnErreurFake) ExtraireTexte(_ context.Context, _ string) (string, error) {
	return "", errors.New("tesseract: exécutable introuvable")
}

func TestKycService_TeleverserDocument_OcrEnEchec_DocumentQuandMemeStocke(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	documents := nouveauDocumentKycRepoFake()
	service := appkyc.NewKycService(
		utilisateurs, wallets, testcommun.NewReglesKycRepoFake(), nouveauDossierKycRepoFake(), documents,
		testcommun.StockageFichierFake{}, ocrEnErreurFake{}, testcommun.TxManagerFake{},
	)
	utilisateurID, dossierID := creerUtilisateurEtDossier(t, service)

	document, err := service.TeleverserDocument(context.Background(), utilisateurID, inputkyc.TeleverserDocumentRequest{
		DossierKycID: dossierID,
		TypeDocument: kyc.TypeDocumentRectoPieceIdentite,
		NomFichier:   "cni.jpg",
		Contenu:      jpegFactice,
	})
	require.NoError(t, err)
	assert.Empty(t, document.TexteExtrait)
}

func TestKycService_ObtenirDossierCourant_Succes(t *testing.T) {
	service, _, _, _, _ := setupService()
	utilisateurID, dossierID := creerUtilisateurEtDossier(t, service)

	dossier, err := service.ObtenirDossierCourant(context.Background(), utilisateurID)
	require.NoError(t, err)
	assert.Equal(t, dossierID, dossier.ID)
}
