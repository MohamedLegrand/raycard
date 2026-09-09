package admin_test

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

	domaincommun "raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	apihttp "raycard/internal/transport/http"
	handlersadmin "raycard/internal/transport/http/handlers/admin"
	testadmin "raycard/test/transport/http/admin"
	"raycard/test/transport/http/harnais"
)

func nouvelleAppAdmin(t *testing.T, adminUseCase inputadmin.AdminUseCase) (app *fiber.App, tokenClient, tokenAdmin, tokenSuperAdmin string) {
	t.Helper()
	validate := validator.New()
	h := apihttp.Handlers{Admin: handlersadmin.NewAdminHandler(adminUseCase, validate)}
	app, tokenGenerator := harnais.NouvelleApp(h)
	tokenClient = harnais.Token(t, tokenGenerator, "user-1", domaincommun.RoleClient)
	tokenAdmin = harnais.Token(t, tokenGenerator, "admin-1", domaincommun.RoleAdmin)
	tokenSuperAdmin = harnais.Token(t, tokenGenerator, "super-admin-1", domaincommun.RoleSuperAdmin)
	return app, tokenClient, tokenAdmin, tokenSuperAdmin
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

func nouvelUtilisateurTest(t *testing.T) *domaincommun.Utilisateur {
	t.Helper()
	u, err := domaincommun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	return u
}

// --- ListerUtilisateurs : GET /api/v1/backoffice/utilisateurs ---

func TestAdminHandler_ListerUtilisateurs_RefuseClientNonAdmin(t *testing.T) {
	app, tokenClient, _, _ := nouvelleAppAdmin(t, &testadmin.AdminUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/utilisateurs", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminHandler_ListerUtilisateurs_Succes(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{ListerUtilisateursResultat: []*domaincommun.Utilisateur{nouvelUtilisateurTest(t)}}
	app, _, tokenAdmin, _ := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/utilisateurs?q=awa", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "awa", fake.DernierFiltreUtilisateurs.Recherche)
}

// --- ObtenirUtilisateur : GET /api/v1/backoffice/utilisateurs/:id ---

func TestAdminHandler_ObtenirUtilisateur_Introuvable(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{ObtenirUtilisateurErr: domaincommun.ErrUtilisateurIntrouvable}
	app, _, tokenAdmin, _ := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/utilisateurs/user-inconnu", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAdminHandler_ObtenirUtilisateur_Succes(t *testing.T) {
	u := nouvelUtilisateurTest(t)
	fake := &testadmin.AdminUseCaseFake{ObtenirUtilisateurResultat: &inputadmin.UtilisateurDetail{Utilisateur: u}}
	app, _, tokenAdmin, _ := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/utilisateurs/"+u.ID, nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, u.ID, fake.DernierUtilisateurID)
}

// --- ChangerRole : PUT /api/v1/backoffice/utilisateurs/:id/role (super_admin uniquement) ---

func TestAdminHandler_ChangerRole_RefuseAdminSimple(t *testing.T) {
	app, _, tokenAdmin, _ := nouvelleAppAdmin(t, &testadmin.AdminUseCaseFake{})

	req := requeteJSON(t, http.MethodPut, "/api/v1/backoffice/utilisateurs/user-1/role", map[string]any{
		"role": "admin",
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	// RequireSuperAdmin : un admin ordinaire n'y suffit pas, contrairement
	// aux autres routes back-office (voir router.go).
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminHandler_ChangerRole_CorpsInvalide_RoleInconnu(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodPut, "/api/v1/backoffice/utilisateurs/user-1/role", map[string]any{
		"role": "role-inexistant",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, fake.DernierRoleDemande)
}

func TestAdminHandler_ChangerRole_Succes(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{ChangerRoleUtilisateurResultat: nouvelUtilisateurTest(t)}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodPut, "/api/v1/backoffice/utilisateurs/user-1/role", map[string]any{
		"role": "admin",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "super-admin-1", fake.DernierAdminID)
	assert.Equal(t, domaincommun.RoleAdmin, fake.DernierRoleDemande)
}

func TestAdminHandler_ChangerRole_AutoModification(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{ChangerRoleUtilisateurErr: domaincommun.ErrAutoModificationRole}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodPut, "/api/v1/backoffice/utilisateurs/super-admin-1/role", map[string]any{
		"role": "utilisateur",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// --- CreerAdministrateur : POST /api/v1/backoffice/utilisateurs (super_admin uniquement) ---

func TestAdminHandler_CreerAdministrateur_RefuseAdminSimple(t *testing.T) {
	app, _, tokenAdmin, _ := nouvelleAppAdmin(t, &testadmin.AdminUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/utilisateurs", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com", "telephone": "+2250700000000",
		"pays_code": "CI", "mot_de_passe": "motdepasse123", "role": "admin",
	}, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminHandler_CreerAdministrateur_CorpsInvalide_RoleUtilisateurRefuse(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	// "utilisateur" est valide pour ChangerRole mais pas ici (voir
	// admindto.CreerAdministrateurRequestDTO : cet endpoint ne crée jamais
	// de compte client).
	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/utilisateurs", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com", "telephone": "+2250700000000",
		"pays_code": "CI", "mot_de_passe": "motdepasse123", "role": "utilisateur",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAdminHandler_CreerAdministrateur_Succes(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{CreerAdministrateurResultat: nouvelUtilisateurTest(t)}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/utilisateurs", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com", "telephone": "+2250700000000",
		"pays_code": "CI", "mot_de_passe": "motdepasse123", "role": "admin",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "super-admin-1", fake.DernierAdminID)
	assert.Equal(t, "awa@example.com", fake.DerniereReqCreerAdmin.Email)
}

func TestAdminHandler_CreerAdministrateur_EmailDejaUtilise(t *testing.T) {
	fake := &testadmin.AdminUseCaseFake{CreerAdministrateurErr: domaincommun.ErrEmailDejaUtilise}
	app, _, _, tokenSuperAdmin := nouvelleAppAdmin(t, fake)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/utilisateurs", map[string]any{
		"nom": "Koné", "prenom": "Awa", "email": "awa@example.com", "telephone": "+2250700000000",
		"pays_code": "CI", "mot_de_passe": "motdepasse123", "role": "admin",
	}, tokenSuperAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}
