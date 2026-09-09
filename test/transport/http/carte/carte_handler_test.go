package carte_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domaincarte "raycard/internal/core/domain/carte"
	"raycard/internal/core/domain/commun"
	inputcarte "raycard/internal/core/ports/input/carte"
	apihttp "raycard/internal/transport/http"
	handlerscarte "raycard/internal/transport/http/handlers/carte"
	testcarte "raycard/test/transport/http/carte"
	"raycard/test/transport/http/harnais"
)

// nouvelleAppCarte monte l'app avec les handlers Carte/AdminCarte réels,
// branchés aux fakes fournis — voir harnais.NouvelleApp.
func nouvelleAppCarte(t *testing.T, carteUseCase inputcarte.CarteUseCase, adminCarteUseCase inputcarte.AdminCarteUseCase) (*fiber.App, string, string) {
	t.Helper()
	validate := validator.New()
	h := apihttp.Handlers{
		Carte:      handlerscarte.NewCarteHandler(carteUseCase, validate),
		AdminCarte: handlerscarte.NewAdminCarteHandler(adminCarteUseCase, validate),
	}
	app, tokenGenerator := harnais.NouvelleApp(h)
	tokenClient := harnais.Token(t, tokenGenerator, "user-1", commun.RoleClient)
	tokenAdmin := harnais.Token(t, tokenGenerator, "admin-1", commun.RoleAdmin)
	return app, tokenClient, tokenAdmin
}

func requeteJSON(t *testing.T, methode, chemin string, corps any, token string) *http.Request {
	t.Helper()
	var lecteurCorps io.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		require.NoError(t, err)
		lecteurCorps = bytes.NewReader(b)
	}
	req := httptest.NewRequest(methode, chemin, lecteurCorps)
	if corps != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func corpsJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func nouvelleCarteTest(t *testing.T) *domaincarte.Carte {
	t.Helper()
	c, err := domaincarte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "USD", 10000)
	require.NoError(t, err)
	return c
}

// --- CreerCarte : POST /api/v1/cartes ---

func TestCarteHandler_CreerCarte_Succes(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{CreerCarteResultat: nouvelleCarteTest(t)}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes", map[string]any{
		"label": "Carte courses", "montant_centimes": 10000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := corpsJSON(t, resp)
	assert.Equal(t, "Carte courses", body["label"])
	// Le handler doit transmettre l'ID utilisateur du JWT, jamais un champ
	// du corps de requête (aucun champ utilisateur_id n'existe côté DTO).
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
	assert.Equal(t, int64(10000), fake.DerniereReqCreerCarte.MontantCentimes)
}

func TestCarteHandler_CreerCarte_CorpsInvalide_MontantManquant(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{CreerCarteResultat: nouvelleCarteTest(t)}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes", map[string]any{
		"label": "Carte courses",
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, fake.DernierUtilisateurID, "le use case ne doit jamais être appelé si la validation échoue")
}

func TestCarteHandler_CreerCarte_SansToken(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{CreerCarteResultat: nouvelleCarteTest(t)}
	app, _, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes", map[string]any{
		"label": "Carte courses", "montant_centimes": 10000,
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCarteHandler_CreerCarte_TokenInvalide(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{CreerCarteResultat: nouvelleCarteTest(t)}
	app, _, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes", map[string]any{
		"label": "Carte courses", "montant_centimes": 10000,
	}, "ceci-nest-pas-un-jwt")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCarteHandler_CreerCarte_ErreurDomaine_KycTierInsuffisant(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{CreerCarteErr: domaincarte.ErrKycTierInsuffisant}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes", map[string]any{
		"label": "Carte courses", "montant_centimes": 10000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	body := corpsJSON(t, resp)
	assert.Contains(t, body["erreur"], "palier KYC")
}

// --- ObtenirCarte : GET /api/v1/cartes/:id ---

func TestCarteHandler_ObtenirCarte_Introuvable(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{ObtenirCarteErr: domaincarte.ErrCarteIntrouvable}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/cartes/carte-inconnue", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "carte-inconnue", fake.DernierCarteID)
}

func TestCarteHandler_ObtenirCarte_Succes(t *testing.T) {
	c := nouvelleCarteTest(t)
	fake := &testcarte.CarteUseCaseFake{ObtenirCarteResultat: c}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/cartes/"+c.ID, nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := corpsJSON(t, resp)
	assert.Equal(t, c.ID, body["id"])
	assert.Equal(t, "active", body["statut"])
}

// --- ListerCartes : GET /api/v1/cartes ---

func TestCarteHandler_ListerCartes_Succes(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{ListerCartesResultat: []*domaincarte.Carte{nouvelleCarteTest(t)}}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/cartes", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

// --- RechargerCarte : POST /api/v1/cartes/:id/topup ---

func TestCarteHandler_RechargerCarte_MontantInvalide(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{RechargerCarteResultat: nouvelleCarteTest(t)}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes/card-1/topup", map[string]any{
		"montant_centimes": 0,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- RetirerCarte : POST /api/v1/cartes/:id/retrait ---

func TestCarteHandler_RetirerCarte_Succes(t *testing.T) {
	c := nouvelleCarteTest(t)
	fake := &testcarte.CarteUseCaseFake{RetirerCarteResultat: c}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes/"+c.ID+"/retrait", map[string]any{
		"montant_centimes": 3000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int64(3000), fake.DerniereReqRetirer.MontantCentimes)
	assert.Equal(t, c.ID, fake.DernierCarteID)
}

func TestCarteHandler_RetirerCarte_SoldeCarteInsuffisant(t *testing.T) {
	fake := &testcarte.CarteUseCaseFake{RetirerCarteErr: domaincarte.ErrSoldeCarteInsuffisant}
	app, tokenClient, _ := nouvelleAppCarte(t, fake, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/cartes/card-1/retrait", map[string]any{
		"montant_centimes": 999999,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// --- Routes back-office : /api/v1/backoffice/cartes/... ---

func TestAdminCarteHandler_ListerCartes_RefuseClientNonAdmin(t *testing.T) {
	fakeAdmin := &testcarte.AdminCarteUseCaseFake{}
	app, tokenClient, _ := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/cartes", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminCarteHandler_ListerCartes_Succes(t *testing.T) {
	fakeAdmin := &testcarte.AdminCarteUseCaseFake{ListerCartesAdminResultat: []*domaincarte.Carte{nouvelleCarteTest(t)}}
	app, _, tokenAdmin := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/cartes", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminCarteHandler_ObtenirCardWallet_Succes(t *testing.T) {
	fakeAdmin := &testcarte.AdminCarteUseCaseFake{ObtenirCardWalletAdminResultat: 42000}
	app, _, tokenAdmin := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/cartes/portefeuille", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := corpsJSON(t, resp)
	assert.Equal(t, float64(42000), body["solde_usd_centimes"])
}

func TestAdminCarteHandler_AlimenterCardWallet_CorpsInvalide(t *testing.T) {
	fakeAdmin := &testcarte.AdminCarteUseCaseFake{}
	app, _, tokenAdmin := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/cartes/portefeuille/financer", map[string]any{
		"montant_usd_centimes": 0,
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int64(0), fakeAdmin.DernierMontantAlimenter, "le use case ne doit jamais être appelé si la validation échoue")
}

func TestAdminCarteHandler_AlimenterCardWallet_Succes(t *testing.T) {
	fakeAdmin := &testcarte.AdminCarteUseCaseFake{AlimenterCardWalletAdminResultat: 100000}
	app, _, tokenAdmin := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/cartes/portefeuille/financer", map[string]any{
		"montant_usd_centimes": 50000,
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "admin-1", fakeAdmin.DernierAdminID)
	assert.Equal(t, int64(50000), fakeAdmin.DernierMontantAlimenter)
	body := corpsJSON(t, resp)
	assert.Equal(t, float64(100000), body["solde_usd_centimes"])
}

// TestAdminCarteHandler_PasDeRouteAnnulation verrouille une décision
// produit : un admin ne peut jamais détruire la carte d'un client depuis
// le dashboard back-office, contrairement au client lui-même (voir
// POST /cartes/:id/annuler, propriétaire uniquement). Seul le gel/dégel
// (tous deux réversibles) reste disponible côté admin — voir le
// commentaire sur inputcarte.AdminCarteUseCase.
func TestAdminCarteHandler_PasDeRouteAnnulation(t *testing.T) {
	app, _, tokenAdmin := nouvelleAppCarte(t, &testcarte.CarteUseCaseFake{}, &testcarte.AdminCarteUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/cartes/card-1/annuler", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
