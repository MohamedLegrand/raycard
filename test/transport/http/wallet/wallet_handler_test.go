package wallet_test

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

	"raycard/internal/core/domain/commun"
	domainwallet "raycard/internal/core/domain/wallet"
	inputwallet "raycard/internal/core/ports/input/wallet"
	apihttp "raycard/internal/transport/http"
	handlerswallet "raycard/internal/transport/http/handlers/wallet"
	"raycard/test/transport/http/harnais"
	testwallet "raycard/test/transport/http/wallet"
)

func nouvelleAppWallet(t *testing.T, walletUseCase inputwallet.WalletUseCase, adminWalletUseCase inputwallet.AdminWalletUseCase) (*fiber.App, string, string) {
	t.Helper()
	validate := validator.New()
	h := apihttp.Handlers{
		Wallet:      handlerswallet.NewWalletHandler(walletUseCase, validate),
		AdminWallet: handlerswallet.NewAdminWalletHandler(adminWalletUseCase),
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
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out
}

func nouveauWalletTest(t *testing.T) *commun.Wallet {
	t.Helper()
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 1_000_000)
	require.NoError(t, err)
	return w
}

func nouvelleTransactionTest(t *testing.T) *domainwallet.Transaction {
	t.Helper()
	tx, err := domainwallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)
	return tx
}

// --- ObtenirWallet : GET /api/v1/wallet ---

func TestWalletHandler_ObtenirWallet_SansToken(t *testing.T) {
	app, _, _ := nouvelleAppWallet(t, &testwallet.WalletUseCaseFake{}, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/wallet", nil, "")
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWalletHandler_ObtenirWallet_Succes(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{ObtenirWalletResultat: nouveauWalletTest(t)}
	app, tokenClient, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodGet, "/api/v1/wallet", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user-1", fake.DernierUtilisateurID)
	body := corpsJSON(t, resp)
	assert.Equal(t, "XOF", body["devise"])
}

// --- InitierRecharge : POST /api/v1/wallet/topup ---

func TestWalletHandler_InitierRecharge_CorpsInvalide_TelephoneMalforme(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{}
	app, tokenClient, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/wallet/topup", map[string]any{
		"operateur": "ORANGE", "telephone": "0700000000", "montant_centimes": 5000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, fake.DerniereReqRecharge.Operateur, "le use case ne doit jamais être appelé si la validation échoue")
}

func TestWalletHandler_InitierRecharge_Succes(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{InitierRechargeResultat: nouvelleTransactionTest(t)}
	app, tokenClient, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/wallet/topup", map[string]any{
		"operateur": "ORANGE", "telephone": "+2250700000000", "montant_centimes": 5000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, int64(5000), fake.DerniereReqRecharge.MontantCentimes)
}

func TestWalletHandler_InitierRecharge_TransactionDejaEnCours(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{InitierRechargeErr: domainwallet.ErrTransactionDejaEnCours}
	app, tokenClient, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/wallet/topup", map[string]any{
		"operateur": "ORANGE", "telephone": "+2250700000000", "montant_centimes": 5000,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// --- InitierRetrait : POST /api/v1/wallet/cashout ---

func TestWalletHandler_InitierRetrait_SoldeInsuffisant(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{InitierRetraitErr: commun.ErrSoldeInsuffisant}
	app, tokenClient, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/wallet/cashout", map[string]any{
		"operateur": "ORANGE", "telephone": "+2250700000000", "montant_centimes": 999999999,
	}, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// --- WebhookHrPay : POST /api/v1/webhooks/hrpay (non authentifiée par JWT) ---

func TestWalletHandler_WebhookHrPay_SansToken_PasBloque(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{}
	app, _, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/hrpay", bytes.NewReader([]byte(`{"type":"payment.succeeded"}`)))
	req.Header.Set("X-Hub-Signature", "signature-de-test")
	resp, err := app.Test(req)
	require.NoError(t, err)

	// Contrairement aux autres routes, aucune authentification JWT n'est
	// exigée ici : l'authenticité vient de la signature HMAC, vérifiée par
	// le use case lui-même (voir WalletHandler.WebhookHrPay).
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "signature-de-test", fake.DerniereSignature)
}

func TestWalletHandler_WebhookHrPay_SignatureInvalide(t *testing.T) {
	fake := &testwallet.WalletUseCaseFake{TraiterWebhookErr: domainwallet.ErrWebhookSignatureInvalide}
	app, _, _ := nouvelleAppWallet(t, fake, &testwallet.AdminWalletUseCaseFake{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/hrpay", bytes.NewReader([]byte(`{}`)))
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// --- Routes back-office : /api/v1/backoffice/wallets/... ---

func TestAdminWalletHandler_GelerWallet_RefuseClientNonAdmin(t *testing.T) {
	app, tokenClient, _ := nouvelleAppWallet(t, &testwallet.WalletUseCaseFake{}, &testwallet.AdminWalletUseCaseFake{})

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/wallets/wallet-1/gel", nil, tokenClient)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminWalletHandler_GelerWallet_Succes(t *testing.T) {
	fakeAdmin := &testwallet.AdminWalletUseCaseFake{GelerWalletAdminResultat: nouveauWalletTest(t)}
	app, _, tokenAdmin := nouvelleAppWallet(t, &testwallet.WalletUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodPost, "/api/v1/backoffice/wallets/wallet-1/gel", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "admin-1", fakeAdmin.DernierAdminID)
	assert.Equal(t, "wallet-1", fakeAdmin.DernierWalletID)
}

func TestAdminWalletHandler_ListerTransactions_Succes(t *testing.T) {
	fakeAdmin := &testwallet.AdminWalletUseCaseFake{ListerTransactionsAdminResultat: []*domainwallet.Transaction{nouvelleTransactionTest(t)}}
	app, _, tokenAdmin := nouvelleAppWallet(t, &testwallet.WalletUseCaseFake{}, fakeAdmin)

	req := requeteJSON(t, http.MethodGet, "/api/v1/backoffice/transactions?statut=succes", nil, tokenAdmin)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "succes", fakeAdmin.DernierFiltre.Statut)
}
