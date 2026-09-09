package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authdomain "raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
	inputauth "raycard/internal/core/ports/input/auth"
	apihttp "raycard/internal/transport/http"
	handlersauth "raycard/internal/transport/http/handlers/auth"
	testauth "raycard/test/transport/http/auth"
	"raycard/test/transport/http/harnais"
)

func nouvelleAppAuth(t *testing.T, authUseCase inputauth.AuthUseCase) (*fiber.App, string) {
	t.Helper()
	validate := validator.New()
	h := apihttp.Handlers{Auth: handlersauth.NewAuthHandler(authUseCase, validate)}
	app, tokenGenerator := harnais.NouvelleApp(h)
	tokenClient := harnais.Token(t, tokenGenerator, "user-1", commun.RoleClient)
	return app, tokenClient
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

func requeteMultipart(t *testing.T, chemin, champFichier, nomFichier string, contenuFichier []byte, token string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if nomFichier != "" {
		fw, err := w.CreateFormFile(champFichier, nomFichier)
		require.NoError(t, err)
		_, err = fw.Write(contenuFichier)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, chemin, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
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

func nouvelUtilisateurTest(t *testing.T) *commun.Utilisateur {
	t.Helper()
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	return u
}

// --- Connexion : POST /api/v1/auth/connexion ---

func TestAuthHandler_Connexion_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ConnexionResultat: &inputauth.ConnexionResultat{Ticket: "ticket-1", ExpireDansSec: 300}}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion", map[string]any{
		"email": "awa@example.com", "mot_de_passe": "motdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "awa@example.com", fake.DerniereReqConnexion.Email)
	body := corpsJSON(t, resp)
	assert.Equal(t, "ticket-1", body["ticket"])
}

func TestAuthHandler_Connexion_CorpsInvalide_EmailInvalide(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion", map[string]any{
		"email": "pas-un-email", "mot_de_passe": "motdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_Connexion_IdentifiantsInvalides(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ConnexionErr: authdomain.ErrIdentifiantsInvalides}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion", map[string]any{
		"email": "awa@example.com", "mot_de_passe": "mauvais",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- VerifierCode2FA : POST /api/v1/auth/connexion/verifier-code ---

func TestAuthHandler_VerifierCode2FA_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{VerifierCode2FAResultat: &inputauth.SessionResultat{AccessToken: "at-1", RefreshToken: "rt-1"}}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion/verifier-code", map[string]any{
		"ticket": "ticket-1", "code": "042951",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ticket-1", fake.DernierTicket)
	assert.Equal(t, "042951", fake.DernierCode)
}

func TestAuthHandler_VerifierCode2FA_CorpsInvalide_CodePasSixChiffres(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion/verifier-code", map[string]any{
		"ticket": "ticket-1", "code": "42",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_VerifierCode2FA_TicketInvalide(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{VerifierCode2FAErr: authdomain.ErrTokenInvalide}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/connexion/verifier-code", map[string]any{
		"ticket": "ticket-expire", "code": "042951",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Rafraichir : POST /api/v1/auth/rafraichir ---

func TestAuthHandler_Rafraichir_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{RafraichirTokenResultat: &inputauth.SessionResultat{AccessToken: "at-2"}}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/rafraichir", map[string]any{"refresh_token": "rt-1"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "rt-1", fake.DernierRefreshToken)
}

func TestAuthHandler_Rafraichir_TokenInvalide(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{RafraichirTokenErr: authdomain.ErrTokenInvalide}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/rafraichir", map[string]any{"refresh_token": "rt-invalide"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Deconnexion : POST /api/v1/auth/deconnexion (non authentifiée par JWT) ---

func TestAuthHandler_Deconnexion_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/deconnexion", map[string]any{"refresh_token": "rt-1"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Aucun JWT exigé (voir router.go) : la révocation du refresh token
	// suffit à prouver la légitimité de la déconnexion.
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "rt-1", fake.DernierRefreshToken)
}

// --- DemanderReinitialisation / Reinitialiser ---

func TestAuthHandler_DemanderReinitialisation_ToujoursSucces(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/mot-de-passe-oublie", map[string]any{"email": "inconnu@example.com"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "inconnu@example.com", fake.DernierEmail)
}

func TestAuthHandler_Reinitialiser_TokenInvalide(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ReinitialiserErr: authdomain.ErrTokenInvalide}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/reinitialiser-mot-de-passe", map[string]any{
		"token": "042951", "nouveau_mot_de_passe": "nouveaumotdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- EnregistrerAppareil / RevoquerAppareil ---

func TestAuthHandler_EnregistrerAppareil_SansToken(t *testing.T) {
	app, _ := nouvelleAppAuth(t, &testauth.AuthUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/empreinte/appareils", map[string]any{
		"cle_publique": "TUNvd0JRWURLMlZ3QXlFQQ==", "nom_appareil": "iPhone d'Awa",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_EnregistrerAppareil_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{EnregistrerAppareilResultat: &inputauth.AppareilResultat{ID: "appareil-1", NomAppareil: "iPhone d'Awa"}}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/empreinte/appareils", map[string]any{
		"cle_publique": "TUNvd0JRWURLMlZ3QXlFQQ==", "nom_appareil": "iPhone d'Awa",
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestAuthHandler_EnregistrerAppareil_CorpsInvalide_ClePasBase64(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/empreinte/appareils", map[string]any{
		"cle_publique": "pas-du-base64!!!", "nom_appareil": "iPhone d'Awa",
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_RevoquerAppareil_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodDelete, "/api/v1/auth/empreinte/appareils/appareil-1", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "appareil-1", fake.DernierAppareilID)
}

func TestAuthHandler_RevoquerAppareil_Introuvable(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{RevoquerAppareilErr: authdomain.ErrCleAppareilIntrouvable}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodDelete, "/api/v1/auth/empreinte/appareils/appareil-inconnu", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- DemanderChallengeEmpreinte / ConnexionEmpreinte ---

func TestAuthHandler_DemanderChallengeEmpreinte_CorpsInvalide_AppareilIDPasUUID(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/empreinte/challenge", map[string]any{"appareil_id": "pas-un-uuid"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ConnexionEmpreinte_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ConnexionEmpreinteResultat: &inputauth.SessionResultat{AccessToken: "at-3"}}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/empreinte/verifier", map[string]any{
		"challenge_id": "3fa2c1e4-9b5d-4a2e-8c1a-0e2f6a7b8c9d", "signature": "TUNvd0JRWURLMlZ3QXlFQQ==",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- Profil : GET/PUT /api/v1/auth/profil ---

func TestAuthHandler_ObtenirProfil_SansToken(t *testing.T) {
	app, _ := nouvelleAppAuth(t, &testauth.AuthUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/auth/profil", nil, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_ObtenirProfil_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ObtenirProfilResultat: nouvelUtilisateurTest(t)}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodGet, "/api/v1/auth/profil", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestAuthHandler_ModifierProfil_CorpsInvalide_NomTropCourt(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPut, "/api/v1/auth/profil", map[string]any{"nom": "K", "prenom": "Awa"}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ModifierProfil_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ModifierProfilResultat: nouvelUtilisateurTest(t)}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPut, "/api/v1/auth/profil", map[string]any{"nom": "Koné", "prenom": "Awa"}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- Photo de profil : POST/GET /api/v1/auth/profil/photo ---

func TestAuthHandler_ModifierPhotoProfil_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ModifierPhotoProfilResultat: nouvelUtilisateurTest(t)}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteMultipart(t, "/api/v1/auth/profil/photo", "photo", "photo.jpg", []byte{0xFF, 0xD8, 0xFF}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestAuthHandler_ModifierPhotoProfil_FichierManquant(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteMultipart(t, "/api/v1/auth/profil/photo", "photo", "", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAuthHandler_ObtenirPhotoProfil_Absente(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ObtenirPhotoProfilErr: commun.ErrPhotoProfilAbsente}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodGet, "/api/v1/auth/profil/photo", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- ChangerMotDePasse / DemanderChangementEmail / ConfirmerChangementEmail ---

func TestAuthHandler_ChangerMotDePasse_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/profil/mot-de-passe", map[string]any{
		"mot_de_passe_actuel": "ancienmotdepasse123", "nouveau_mot_de_passe": "nouveaumotdepasse456",
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestAuthHandler_ChangerMotDePasse_MotDePasseActuelIncorrect(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ChangerMotDePasseErr: authdomain.ErrIdentifiantsInvalides}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/profil/mot-de-passe", map[string]any{
		"mot_de_passe_actuel": "mauvais", "nouveau_mot_de_passe": "nouveaumotdepasse456",
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_DemanderChangementEmail_EmailDejaUtilise(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{DemanderChangementEmailErr: commun.ErrEmailDejaUtilise}
	app, tokenClient := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/profil/email", map[string]any{"nouvel_email": "prise@example.com"}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestAuthHandler_ConfirmerChangementEmail_Succes(t *testing.T) {
	fake := &testauth.AuthUseCaseFake{ConfirmerChangementEmailResultat: nouvelUtilisateurTest(t)}
	app, _ := nouvelleAppAuth(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/profil/email/confirmer", map[string]any{"code": "042951"}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
