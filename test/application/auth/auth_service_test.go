package auth_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	appauth "raycard/internal/application/auth"
	authdomain "raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
	authinput "raycard/internal/core/ports/input/auth"
	authoutput "raycard/internal/core/ports/output/auth"
	testcommun "raycard/test/application/commun"
)

// --- faux TokenGenerator, RefreshTokenRepository, TokenReinitialisationRepository, TicketConnexionRepository, Notifieur et GoogleAuthProvider, sans dépendance JWT/DB/email/Google ---

type tokenGeneratorFake struct{}

func (tokenGeneratorFake) GenererAccessToken(claims authoutput.Claims) (string, time.Time, error) {
	return "access-" + claims.UtilisateurID, time.Now().Add(15 * time.Minute), nil
}

func (tokenGeneratorFake) ValiderAccessToken(token string) (authoutput.Claims, error) {
	utilisateurID, trouve := strings.CutPrefix(token, "access-")
	if !trouve {
		return authoutput.Claims{}, authdomain.ErrTokenInvalide
	}
	return authoutput.Claims{UtilisateurID: utilisateurID}, nil
}

type refreshTokenRepoFake struct {
	parHash map[string]*authdomain.RefreshToken
	parID   map[string]*authdomain.RefreshToken
}

func nouveauRefreshTokenRepoFake() *refreshTokenRepoFake {
	return &refreshTokenRepoFake{
		parHash: make(map[string]*authdomain.RefreshToken),
		parID:   make(map[string]*authdomain.RefreshToken),
	}
}

func (r *refreshTokenRepoFake) Create(_ context.Context, rt *authdomain.RefreshToken) error {
	r.parHash[rt.TokenHash] = rt
	r.parID[rt.ID] = rt
	return nil
}

func (r *refreshTokenRepoFake) FindByHash(_ context.Context, tokenHash string) (*authdomain.RefreshToken, error) {
	rt, ok := r.parHash[tokenHash]
	if !ok {
		return nil, authdomain.ErrTokenInvalide
	}
	// Copie défensive : une vraie lecture DB renvoie toujours un objet
	// indépendant, jamais une référence mutable vers le stockage interne.
	copie := *rt
	return &copie, nil
}

func (r *refreshTokenRepoFake) Revoke(_ context.Context, id string) error {
	rt, ok := r.parID[id]
	if !ok || rt.RevokedAt != nil {
		// Reflète le comportement du repository Postgres réel (clause
		// WHERE revoked_at IS NULL) : révoquer deux fois est une erreur.
		return authdomain.ErrTokenInvalide
	}
	rt.Revoquer()
	return nil
}

func (r *refreshTokenRepoFake) RevokeAllForUtilisateur(_ context.Context, utilisateurID string) error {
	for _, rt := range r.parID {
		if rt.UtilisateurID == utilisateurID && rt.RevokedAt == nil {
			rt.Revoquer()
		}
	}
	return nil
}

type tokenReinitialisationRepoFake struct {
	parHash map[string]*authdomain.TokenReinitialisation
	parID   map[string]*authdomain.TokenReinitialisation
}

func nouveauTokenReinitialisationRepoFake() *tokenReinitialisationRepoFake {
	return &tokenReinitialisationRepoFake{
		parHash: make(map[string]*authdomain.TokenReinitialisation),
		parID:   make(map[string]*authdomain.TokenReinitialisation),
	}
}

func (r *tokenReinitialisationRepoFake) Create(_ context.Context, t *authdomain.TokenReinitialisation) error {
	r.parHash[t.TokenHash] = t
	r.parID[t.ID] = t
	return nil
}

func (r *tokenReinitialisationRepoFake) FindByHash(_ context.Context, tokenHash string) (*authdomain.TokenReinitialisation, error) {
	t, ok := r.parHash[tokenHash]
	if !ok {
		return nil, authdomain.ErrTokenInvalide
	}
	// Copie défensive : une vraie lecture DB renvoie toujours un objet
	// indépendant, jamais une référence mutable vers le stockage interne.
	copie := *t
	return &copie, nil
}

func (r *tokenReinitialisationRepoFake) MarquerUtilise(_ context.Context, id string) error {
	t, ok := r.parID[id]
	if !ok || t.UtiliseAt != nil {
		return authdomain.ErrTokenInvalide
	}
	t.MarquerUtilise()
	return nil
}

type ticketConnexionRepoFake struct {
	parHash map[string]*authdomain.TicketConnexion
	parID   map[string]*authdomain.TicketConnexion
}

func nouveauTicketConnexionRepoFake() *ticketConnexionRepoFake {
	return &ticketConnexionRepoFake{
		parHash: make(map[string]*authdomain.TicketConnexion),
		parID:   make(map[string]*authdomain.TicketConnexion),
	}
}

func (r *ticketConnexionRepoFake) Create(_ context.Context, t *authdomain.TicketConnexion) error {
	r.parHash[t.TicketHash] = t
	r.parID[t.ID] = t
	return nil
}

func (r *ticketConnexionRepoFake) FindByHash(_ context.Context, ticketHash string) (*authdomain.TicketConnexion, error) {
	t, ok := r.parHash[ticketHash]
	if !ok {
		return nil, authdomain.ErrTokenInvalide
	}
	// Copie défensive : une vraie lecture DB renvoie toujours un objet
	// indépendant, jamais une référence mutable vers le stockage interne.
	copie := *t
	return &copie, nil
}

func (r *ticketConnexionRepoFake) Update(_ context.Context, t *authdomain.TicketConnexion) error {
	if _, ok := r.parID[t.ID]; !ok {
		return authdomain.ErrTokenInvalide
	}
	r.parID[t.ID] = t
	r.parHash[t.TicketHash] = t
	return nil
}

type emailEnvoye struct {
	destinataire, sujet, corps string
}

type notifieurFake struct {
	emailsEnvoyes []emailEnvoye
}

func (n *notifieurFake) EnvoyerEmail(_ context.Context, destinataire, sujet, corpsHTML string) error {
	n.emailsEnvoyes = append(n.emailsEnvoyes, emailEnvoye{destinataire, sujet, corpsHTML})
	return nil
}

// googleAuthProviderFake : identite/erreur sont mutables après
// construction, pour que chaque test configure la réponse attendue de
// "Google" avant d'appeler ConnexionGoogle.
type googleAuthProviderFake struct {
	identite authoutput.IdentiteGoogle
	erreur   error
}

func (g *googleAuthProviderFake) VerifierIDToken(_ context.Context, _ string) (authoutput.IdentiteGoogle, error) {
	if g.erreur != nil {
		return authoutput.IdentiteGoogle{}, g.erreur
	}
	return g.identite, nil
}

type authServiceFakes struct {
	utilisateurs           *testcommun.UtilisateurRepoFake
	wallets                *testcommun.WalletRepoFake
	refreshTokens          *refreshTokenRepoFake
	tokensReinitialisation *tokenReinitialisationRepoFake
	ticketsConnexion       *ticketConnexionRepoFake
	notifieur              *notifieurFake
	googleAuthProvider     *googleAuthProviderFake
}

func setupAuthService() (authinput.AuthUseCase, *authServiceFakes) {
	fakes := &authServiceFakes{
		utilisateurs:           testcommun.NewUtilisateurRepoFake(),
		wallets:                testcommun.NewWalletRepoFake(),
		refreshTokens:          nouveauRefreshTokenRepoFake(),
		tokensReinitialisation: nouveauTokenReinitialisationRepoFake(),
		ticketsConnexion:       nouveauTicketConnexionRepoFake(),
		notifieur:              &notifieurFake{},
		googleAuthProvider:     &googleAuthProviderFake{},
	}
	service := appauth.NewAuthService(
		fakes.utilisateurs, fakes.wallets, testcommun.NewReglesKycRepoFake(),
		fakes.refreshTokens, fakes.tokensReinitialisation, fakes.ticketsConnexion,
		tokenGeneratorFake{}, fakes.notifieur, fakes.googleAuthProvider, testcommun.TxManagerFake{},
	)
	return service, fakes
}

func creerUtilisateurTest(t *testing.T, repo *testcommun.UtilisateurRepoFake, email, motDePasse string) *commun.Utilisateur {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	require.NoError(t, err)

	u, err := commun.NouveauUtilisateur("Koné", "Awa", email, "+2250700000000", "CI", string(hash))
	require.NoError(t, err)
	require.NoError(t, repo.Create(context.Background(), u))
	return u
}

func extraireCodeOTP(t *testing.T, corps string) string {
	t.Helper()
	code := regexp.MustCompile(`\d{6}`).FindString(corps)
	require.NotEmpty(t, code, "le corps de l'email doit contenir un code à 6 chiffres")
	return code
}

// connecterAvec2FA exécute les deux étapes de connexion (mot de passe
// puis code reçu par email) et renvoie la session obtenue. Sert de
// point d'entrée pour les tests d'autres flux (rafraîchissement,
// déconnexion...) qui ont juste besoin d'une session valide.
func connecterAvec2FA(t *testing.T, service authinput.AuthUseCase, notifieur *notifieurFake, email, motDePasse string) *authinput.SessionResultat {
	t.Helper()

	resultatConnexion, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: email, MotDePasse: motDePasse})
	require.NoError(t, err)
	require.NotEmpty(t, resultatConnexion.Ticket)

	require.NotEmpty(t, notifieur.emailsEnvoyes)
	dernierEmail := notifieur.emailsEnvoyes[len(notifieur.emailsEnvoyes)-1]
	code := extraireCodeOTP(t, dernierEmail.corps)

	session, err := service.VerifierCode2FA(context.Background(), resultatConnexion.Ticket, code, authinput.MetadonneesConnexion{})
	require.NoError(t, err)
	return session
}

func TestAuthService_Connexion_Succes(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	// Aucun token de session à cette étape : juste un ticket, la 2FA
	// n'est pas encore validée.
	assert.NotEmpty(t, resultat.Ticket)
	assert.Equal(t, 600, resultat.ExpireDansSec)

	require.Len(t, fakes.notifieur.emailsEnvoyes, 1)
	assert.Equal(t, "awa@example.com", fakes.notifieur.emailsEnvoyes[0].destinataire)
	assert.Len(t, fakes.ticketsConnexion.parHash, 1)
}

func TestAuthService_Connexion_MotDePasseIncorrect(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	_, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "mauvais-mot-de-passe"})
	assert.ErrorIs(t, err, authdomain.ErrIdentifiantsInvalides)
	assert.Empty(t, fakes.notifieur.emailsEnvoyes, "aucun code ne doit être envoyé si le mot de passe est incorrect")
}

func TestAuthService_Connexion_UtilisateurInconnu(t *testing.T) {
	service, _ := setupAuthService()

	_, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "inconnu@example.com", MotDePasse: "peu-importe"})
	// Ne doit jamais fuiter commun.ErrUtilisateurIntrouvable : même erreur
	// générique que pour un mot de passe incorrect.
	assert.ErrorIs(t, err, authdomain.ErrIdentifiantsInvalides)
}

func TestAuthService_VerifierCode2FA_Succes(t *testing.T) {
	service, fakes := setupAuthService()
	u := creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	session := connecterAvec2FA(t, service, fakes.notifieur, "awa@example.com", "motdepasse123")

	assert.Equal(t, "access-"+u.ID, session.AccessToken)
	assert.NotEmpty(t, session.RefreshToken)
	assert.Len(t, fakes.refreshTokens.parHash, 1)
}

func TestAuthService_VerifierCode2FA_MauvaisCode(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	_, err = service.VerifierCode2FA(context.Background(), resultat.Ticket, "000000", authinput.MetadonneesConnexion{})
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)

	// Une tentative a été consommée, mais le ticket reste utilisable
	// pour les tentatives restantes.
	var ticket *authdomain.TicketConnexion
	for _, tk := range fakes.ticketsConnexion.parID {
		ticket = tk
	}
	require.NotNil(t, ticket)
	assert.Equal(t, 4, ticket.TentativesRestantes)

	// Le bon code, lui, fonctionne toujours.
	dernierEmail := fakes.notifieur.emailsEnvoyes[len(fakes.notifieur.emailsEnvoyes)-1]
	bonCode := extraireCodeOTP(t, dernierEmail.corps)
	session, err := service.VerifierCode2FA(context.Background(), resultat.Ticket, bonCode, authinput.MetadonneesConnexion{})
	require.NoError(t, err)
	assert.NotEmpty(t, session.AccessToken)
}

func TestAuthService_VerifierCode2FA_TentativesEpuisees(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	// 5 tentatives maximum : les 5 échecs épuisent le ticket.
	for i := 0; i < 5; i++ {
		_, err := service.VerifierCode2FA(context.Background(), resultat.Ticket, "000000", authinput.MetadonneesConnexion{})
		assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
	}

	// La 5e tentative épuisée déclenche une alerte de sécurité, en plus
	// de l'email initial contenant le code.
	require.Len(t, fakes.notifieur.emailsEnvoyes, 2)
	alerte := fakes.notifieur.emailsEnvoyes[1]
	assert.Equal(t, "awa@example.com", alerte.destinataire)
	assert.Contains(t, alerte.sujet, "Alerte de sécurité")

	// Même le bon code ne fonctionne plus : il faut recommencer depuis
	// Connexion. Le code reste celui du tout premier email (aucun autre
	// n'en contient un nouveau).
	bonCode := extraireCodeOTP(t, fakes.notifieur.emailsEnvoyes[0].corps)
	_, err = service.VerifierCode2FA(context.Background(), resultat.Ticket, bonCode, authinput.MetadonneesConnexion{})
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

func TestAuthService_VerifierCode2FA_NotifieConnexionReussie(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)
	code := extraireCodeOTP(t, fakes.notifieur.emailsEnvoyes[0].corps)

	metadonnees := authinput.MetadonneesConnexion{AdresseIP: "203.0.113.42", AppareilInfo: "TestAgent/1.0"}
	_, err = service.VerifierCode2FA(context.Background(), resultat.Ticket, code, metadonnees)
	require.NoError(t, err)

	require.Len(t, fakes.notifieur.emailsEnvoyes, 2, "email du code + confirmation de connexion réussie")
	confirmation := fakes.notifieur.emailsEnvoyes[1]
	assert.Equal(t, "awa@example.com", confirmation.destinataire)
	assert.Contains(t, confirmation.corps, "203.0.113.42")
	assert.Contains(t, confirmation.corps, "TestAgent/1.0")
}

func TestAuthService_VerifierCode2FA_TicketInvalide(t *testing.T) {
	service, _ := setupAuthService()

	_, err := service.VerifierCode2FA(context.Background(), "ticket-qui-n-existe-pas", "123456", authinput.MetadonneesConnexion{})
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

func TestAuthService_VerifierCode2FA_UsageUnique(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)
	code := extraireCodeOTP(t, fakes.notifieur.emailsEnvoyes[0].corps)

	_, err = service.VerifierCode2FA(context.Background(), resultat.Ticket, code, authinput.MetadonneesConnexion{})
	require.NoError(t, err)

	// Rejouer le même ticket + code échoue : usage unique.
	_, err = service.VerifierCode2FA(context.Background(), resultat.Ticket, code, authinput.MetadonneesConnexion{})
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

// connecterAvecGoogle2FA exécute ConnexionGoogle puis complète la 2FA
// obligatoire déclenchée en retour, exactement comme connecterAvec2FA
// pour la connexion par mot de passe.
func connecterAvecGoogle2FA(t *testing.T, service authinput.AuthUseCase, notifieur *notifieurFake, req authinput.ConnexionGoogleRequest) *authinput.SessionResultat {
	t.Helper()

	resultatConnexion, err := service.ConnexionGoogle(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resultatConnexion.Ticket)

	require.NotEmpty(t, notifieur.emailsEnvoyes)
	dernierEmail := notifieur.emailsEnvoyes[len(notifieur.emailsEnvoyes)-1]
	code := extraireCodeOTP(t, dernierEmail.corps)

	session, err := service.VerifierCode2FA(context.Background(), resultatConnexion.Ticket, code, authinput.MetadonneesConnexion{})
	require.NoError(t, err)
	return session
}

func TestAuthService_ConnexionGoogle_CompteDejaLie(t *testing.T) {
	service, fakes := setupAuthService()

	u, err := commun.NouvelUtilisateurGoogle("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "google-sub-123")
	require.NoError(t, err)
	require.NoError(t, u.ValiderKycTier1())
	require.NoError(t, fakes.utilisateurs.Create(context.Background(), u))

	fakes.googleAuthProvider.identite = authoutput.IdentiteGoogle{
		GoogleID: "google-sub-123", Email: "awa@example.com", EmailVerifie: true, Nom: "Koné", Prenom: "Awa",
	}

	session := connecterAvecGoogle2FA(t, service, fakes.notifieur, authinput.ConnexionGoogleRequest{IDToken: "peu-importe"})
	assert.Equal(t, "access-"+u.ID, session.AccessToken)
	// Code de connexion + confirmation de connexion réussie : la 2FA
	// s'applique aussi à la connexion Google.
	assert.Len(t, fakes.notifieur.emailsEnvoyes, 2)
}

func TestAuthService_ConnexionGoogle_LiaisonCompteExistant(t *testing.T) {
	service, fakes := setupAuthService()
	u := creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")
	require.Empty(t, u.GoogleID)

	fakes.googleAuthProvider.identite = authoutput.IdentiteGoogle{
		GoogleID: "google-sub-456", Email: "awa@example.com", EmailVerifie: true, Nom: "Koné", Prenom: "Awa",
	}

	session := connecterAvecGoogle2FA(t, service, fakes.notifieur, authinput.ConnexionGoogleRequest{IDToken: "peu-importe"})
	assert.Equal(t, "access-"+u.ID, session.AccessToken)

	utilisateurMaj, err := fakes.utilisateurs.FindByGoogleID(context.Background(), "google-sub-456")
	require.NoError(t, err)
	assert.Equal(t, u.ID, utilisateurMaj.ID)
}

func TestAuthService_ConnexionGoogle_LiaisonRefuseeEmailNonVerifie(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	fakes.googleAuthProvider.identite = authoutput.IdentiteGoogle{
		GoogleID: "google-sub-789", Email: "awa@example.com", EmailVerifie: false, Nom: "Koné", Prenom: "Awa",
	}

	_, err := service.ConnexionGoogle(context.Background(), authinput.ConnexionGoogleRequest{IDToken: "peu-importe"})
	assert.ErrorIs(t, err, authdomain.ErrIdentifiantsInvalides)

	_, err = fakes.utilisateurs.FindByGoogleID(context.Background(), "google-sub-789")
	assert.ErrorIs(t, err, commun.ErrUtilisateurIntrouvable, "aucune liaison n'a dû être faite")
}

func TestAuthService_ConnexionGoogle_NouveauCompte(t *testing.T) {
	service, fakes := setupAuthService()

	fakes.googleAuthProvider.identite = authoutput.IdentiteGoogle{
		GoogleID: "google-sub-999", Email: "nouveau@example.com", EmailVerifie: true, Nom: "Traoré", Prenom: "Ibrahim",
	}

	session := connecterAvecGoogle2FA(t, service, fakes.notifieur, authinput.ConnexionGoogleRequest{
		IDToken: "peu-importe", Telephone: "+2250700000099", PaysCode: "CI",
	})
	assert.NotEmpty(t, session.AccessToken)

	utilisateur, err := fakes.utilisateurs.FindByGoogleID(context.Background(), "google-sub-999")
	require.NoError(t, err)
	assert.Equal(t, commun.KycTier1, utilisateur.KycTier)
	assert.Empty(t, utilisateur.MotDePasseHash, "un compte Google pur n'a pas de mot de passe")

	_, err = fakes.wallets.FindByUtilisateurID(context.Background(), utilisateur.ID)
	require.NoError(t, err, "le wallet doit être créé comme pour une inscription classique")
}

func TestAuthService_ConnexionGoogle_NouveauCompte_TelephonePaysManquants(t *testing.T) {
	service, fakes := setupAuthService()

	fakes.googleAuthProvider.identite = authoutput.IdentiteGoogle{
		GoogleID: "google-sub-999", Email: "nouveau@example.com", EmailVerifie: true, Nom: "Traoré", Prenom: "Ibrahim",
	}

	_, err := service.ConnexionGoogle(context.Background(), authinput.ConnexionGoogleRequest{IDToken: "peu-importe"})
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
}

func TestAuthService_ConnexionGoogle_IDTokenInvalide(t *testing.T) {
	service, fakes := setupAuthService()
	fakes.googleAuthProvider.erreur = authdomain.ErrTokenInvalide

	_, err := service.ConnexionGoogle(context.Background(), authinput.ConnexionGoogleRequest{IDToken: "invalide"})
	assert.ErrorIs(t, err, authdomain.ErrIdentifiantsInvalides)
}

func TestAuthService_RafraichirToken_Succes(t *testing.T) {
	service, fakes := setupAuthService()
	u := creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	session1 := connecterAvec2FA(t, service, fakes.notifieur, "awa@example.com", "motdepasse123")

	session2, err := service.RafraichirToken(context.Background(), session1.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "access-"+u.ID, session2.AccessToken)
	assert.NotEqual(t, session1.RefreshToken, session2.RefreshToken, "rotation : un nouveau refresh token doit être émis")

	// L'ancien refresh token est révoqué (usage unique) : le réutiliser échoue.
	_, err = service.RafraichirToken(context.Background(), session1.RefreshToken)
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

func TestAuthService_RafraichirToken_Invalide(t *testing.T) {
	service, _ := setupAuthService()

	_, err := service.RafraichirToken(context.Background(), "token-qui-n-existe-pas")
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

func TestAuthService_Deconnexion(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	session := connecterAvec2FA(t, service, fakes.notifieur, "awa@example.com", "motdepasse123")

	require.NoError(t, service.Deconnexion(context.Background(), session.RefreshToken))

	// Le refresh token révoqué ne peut plus servir à rafraîchir la session.
	_, err := service.RafraichirToken(context.Background(), session.RefreshToken)
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)

	// Déconnexion idempotente : rejouer sur un token déjà révoqué ne casse pas.
	assert.NoError(t, service.Deconnexion(context.Background(), session.RefreshToken))
}

func TestAuthService_DemanderReinitialisation_Succes(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "motdepasse123")

	require.NoError(t, service.DemanderReinitialisation(context.Background(), "awa@example.com"))

	require.Len(t, fakes.notifieur.emailsEnvoyes, 1)
	assert.Equal(t, "awa@example.com", fakes.notifieur.emailsEnvoyes[0].destinataire)
	assert.Len(t, fakes.tokensReinitialisation.parHash, 1)
}

func TestAuthService_DemanderReinitialisation_EmailInconnu(t *testing.T) {
	service, fakes := setupAuthService()

	// Aucune erreur, et surtout aucun email envoyé : ne révèle jamais si
	// l'email existe (évite l'énumération de comptes).
	require.NoError(t, service.DemanderReinitialisation(context.Background(), "inconnu@example.com"))
	assert.Empty(t, fakes.notifieur.emailsEnvoyes)
}

func TestAuthService_Reinitialiser_Succes(t *testing.T) {
	service, fakes := setupAuthService()
	creerUtilisateurTest(t, fakes.utilisateurs, "awa@example.com", "ancienmotdepasse123")

	session := connecterAvec2FA(t, service, fakes.notifieur, "awa@example.com", "ancienmotdepasse123")

	require.NoError(t, service.DemanderReinitialisation(context.Background(), "awa@example.com"))
	dernierEmail := fakes.notifieur.emailsEnvoyes[len(fakes.notifieur.emailsEnvoyes)-1]
	code := extraireCodeOTP(t, dernierEmail.corps)

	require.NoError(t, service.Reinitialiser(context.Background(), code, "nouveaumotdepasse456"))

	_, err := service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "ancienmotdepasse123"})
	assert.ErrorIs(t, err, authdomain.ErrIdentifiantsInvalides, "l'ancien mot de passe ne doit plus fonctionner")

	_, err = service.Connexion(context.Background(), authinput.ConnexionRequest{Email: "awa@example.com", MotDePasse: "nouveaumotdepasse456"})
	assert.NoError(t, err)

	// Toutes les sessions actives (dont celle d'avant la réinitialisation)
	// doivent être révoquées.
	_, err = service.RafraichirToken(context.Background(), session.RefreshToken)
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)

	// Le code est à usage unique.
	err = service.Reinitialiser(context.Background(), code, "autremotdepasse789")
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}

func TestAuthService_Reinitialiser_TokenInvalide(t *testing.T) {
	service, _ := setupAuthService()

	err := service.Reinitialiser(context.Background(), "000000", "nouveaumotdepasse456")
	assert.ErrorIs(t, err, authdomain.ErrTokenInvalide)
}
