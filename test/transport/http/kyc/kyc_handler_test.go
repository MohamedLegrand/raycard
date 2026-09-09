package kyc_test

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

	"raycard/internal/core/domain/commun"
	domainkyc "raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
	apihttp "raycard/internal/transport/http"
	handlerskyc "raycard/internal/transport/http/handlers/kyc"
	"raycard/test/transport/http/harnais"
	testkyc "raycard/test/transport/http/kyc"
)

func nouvelleAppKyc(t *testing.T, kycUseCase inputkyc.KycUseCase, adminKycUseCase inputkyc.AdminKycUseCase) (*fiber.App, string, string) {
	t.Helper()
	validate := validator.New()
	h := apihttp.Handlers{
		Kyc:      handlerskyc.NewKycHandler(kycUseCase, validate),
		AdminKyc: handlerskyc.NewAdminKycHandler(adminKycUseCase, validate),
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

func nouveauDossierTest(t *testing.T) *domainkyc.DossierKyc {
	t.Helper()
	d, err := domainkyc.NouveauDossierKyc("user-1")
	require.NoError(t, err)
	return d
}

// --- Inscrire : POST /api/v1/auth/inscription (publique) ---

func TestKycHandler_Inscrire_Succes(t *testing.T) {
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	w, err := commun.NouveauWallet(u.ID, "CI", "XOF", 1_000_000)
	require.NoError(t, err)
	fake := &testkyc.KycUseCaseFake{InscrireResultat: &inputkyc.InscriptionResultat{Utilisateur: u, Wallet: w}}
	app, _, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/inscription", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com",
		"telephone": "+2250700000000", "pays_code": "CI", "mot_de_passe": "motdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := corpsJSON(t, resp)
	assert.Equal(t, "awa@example.com", body["utilisateur"].(map[string]any)["email"])
}

func TestKycHandler_Inscrire_CorpsInvalide_EmailInvalide(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{}
	app, _, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/inscription", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "pas-un-email",
		"telephone": "+2250700000000", "pays_code": "CI", "mot_de_passe": "motdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, fake.DerniereReqInscrire.Email, "le use case ne doit jamais être appelé si la validation échoue")
}

func TestKycHandler_Inscrire_EmailDejaUtilise(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{InscrireErr: commun.ErrEmailDejaUtilise}
	app, _, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/auth/inscription", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com",
		"telephone": "+2250700000000", "pays_code": "CI", "mot_de_passe": "motdepasse123",
	}, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// --- DemanderTier2 : POST /api/v1/kyc/demande-tier2 ---

func TestKycHandler_DemanderTier2_SansToken(t *testing.T) {
	app, _, _ := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/kyc/demande-tier2", nil, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestKycHandler_DemanderTier2_Succes(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{DemanderTier2Resultat: nouveauDossierTest(t)}
	app, tokenClient, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/kyc/demande-tier2", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestKycHandler_DemanderTier2_DejaEnAttente(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{DemanderTier2Err: domainkyc.ErrDossierKycDejaEnAttente}
	app, tokenClient, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/kyc/demande-tier2", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// --- ObtenirDossierCourant : GET /api/v1/kyc/dossier-courant ---

func TestKycHandler_ObtenirDossierCourant_Introuvable(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{ObtenirDossierCourantErr: domainkyc.ErrDossierKycIntrouvable}
	app, tokenClient, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/kyc/dossier-courant", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- TeleverserDocument : POST /api/v1/kyc/documents (multipart) ---

func requeteMultipart(t *testing.T, chemin string, champs map[string]string, nomFichier string, contenuFichier []byte, token string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range champs {
		require.NoError(t, w.WriteField(k, v))
	}
	if nomFichier != "" {
		fw, err := w.CreateFormFile("document", nomFichier)
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

func TestKycHandler_TeleverserDocument_Succes(t *testing.T) {
	doc, err := domainkyc.NouveauDocumentKyc("user-1", "dossier-1", domainkyc.TypeDocumentRectoPieceIdentite, "recto.jpg", "/chemin/recto.jpg", "")
	require.NoError(t, err)
	fake := &testkyc.KycUseCaseFake{TeleverserDocumentResultat: doc}
	app, tokenClient, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteMultipart(t, "/api/v1/kyc/documents",
		map[string]string{"dossier_id": "dossier-1", "type_document": "recto_piece_identite"},
		"recto.jpg", []byte("contenu-image-factice"), tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "dossier-1", fake.DerniereReqTeleverser.DossierKycID)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
}

func TestKycHandler_TeleverserDocument_FichierManquant(t *testing.T) {
	fake := &testkyc.KycUseCaseFake{}
	app, tokenClient, _ := nouvelleAppKyc(t, fake, &testkyc.AdminKycUseCaseFake{})

	req := requeteMultipart(t, "/api/v1/kyc/documents",
		map[string]string{"dossier_id": "dossier-1", "type_document": "recto_piece_identite"},
		"", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Routes back-office : /api/v1/backoffice/kyc/... ---

func TestAdminKycHandler_ListerDossiersEnAttente_RefuseClientNonAdmin(t *testing.T) {
	app, tokenClient, _ := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, &testkyc.AdminKycUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/kyc/dossiers", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminKycHandler_ListerDossiersEnAttente_Succes(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{ListerDossiersEnAttenteResultat: []*domainkyc.DossierKyc{nouveauDossierTest(t)}}
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/kyc/dossiers", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdminKycHandler_Approuver_Succes(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{}
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/kyc/dossiers/dossier-1/approuver", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "admin-1", fakeAdmin.DernierAdminID)
	assert.Equal(t, "dossier-1", fakeAdmin.DernierDossierID)
}

func TestAdminKycHandler_Approuver_Introuvable(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{ApprouverDossierErr: domainkyc.ErrDossierKycIntrouvable}
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/kyc/dossiers/dossier-inconnu/approuver", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAdminKycHandler_Rejeter_CorpsInvalide_MotifManquant(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{}
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/kyc/dossiers/dossier-1/rejeter", map[string]any{
		"motif": "",
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, fakeAdmin.DernierMotifRejet)
}

func TestAdminKycHandler_Rejeter_Succes(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{}
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/kyc/dossiers/dossier-1/rejeter", map[string]any{
		"motif": "pièce illisible",
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "pièce illisible", fakeAdmin.DernierMotifRejet)
}

func TestAdminKycHandler_RecupererDocument_Succes(t *testing.T) {
	fakeAdmin := &testkyc.AdminKycUseCaseFake{RecupererDocumentContenu: []byte{0xFF, 0xD8, 0xFF}} // en-tête JPEG
	app, _, tokenAdmin := nouvelleAppKyc(t, &testkyc.KycUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/kyc/documents/doc-1", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "doc-1", fakeAdmin.DernierDocumentID)
}
