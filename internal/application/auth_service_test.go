package application_test

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"raycard/internal/application"
	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/core/ports/output"
)

// --- faux TokenGenerator, RefreshTokenRepository, TokenReinitialisationRepository et Notifieur, sans dépendance JWT/DB/email ---

type tokenGeneratorFake struct{}

func (tokenGeneratorFake) GenererAccessToken(claims output.Claims) (string, time.Time, error) {
	return "access-" + claims.UtilisateurID, time.Now().Add(15 * time.Minute), nil
}

func (tokenGeneratorFake) ValiderAccessToken(token string) (output.Claims, error) {
	utilisateurID, trouve := strings.CutPrefix(token, "access-")
	if !trouve {
		return output.Claims{}, domain.ErrTokenInvalide
	}
	return output.Claims{UtilisateurID: utilisateurID}, nil
}

type refreshTokenRepoFake struct {
	parHash map[string]*domain.RefreshToken
	parID   map[string]*domain.RefreshToken
}

func nouveauRefreshTokenRepoFake() *refreshTokenRepoFake {
	return &refreshTokenRepoFake{
		parHash: make(map[string]*domain.RefreshToken),
		parID:   make(map[string]*domain.RefreshToken),
	}
}

func (r *refreshTokenRepoFake) Create(_ context.Context, rt *domain.RefreshToken) error {
	r.parHash[rt.TokenHash] = rt
	r.parID[rt.ID] = rt
	return nil
}

func (r *refreshTokenRepoFake) FindByHash(_ context.Context, tokenHash string) (*domain.RefreshToken, error) {
	rt, ok := r.parHash[tokenHash]
	if !ok {
		return nil, domain.ErrTokenInvalide
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
		return domain.ErrTokenInvalide
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
	parHash map[string]*domain.TokenReinitialisation
	parID   map[string]*domain.TokenReinitialisation
}

func nouveauTokenReinitialisationRepoFake() *tokenReinitialisationRepoFake {
	return &tokenReinitialisationRepoFake{
		parHash: make(map[string]*domain.TokenReinitialisation),
		parID:   make(map[string]*domain.TokenReinitialisation),
	}
}

func (r *tokenReinitialisationRepoFake) Create(_ context.Context, t *domain.TokenReinitialisation) error {
	r.parHash[t.TokenHash] = t
	r.parID[t.ID] = t
	return nil
}

func (r *tokenReinitialisationRepoFake) FindByHash(_ context.Context, tokenHash string) (*domain.TokenReinitialisation, error) {
	t, ok := r.parHash[tokenHash]
	if !ok {
		return nil, domain.ErrTokenInvalide
	}
	// Copie défensive : une vraie lecture DB renvoie toujours un objet
	// indépendant, jamais une référence mutable vers le stockage interne.
	copie := *t
	return &copie, nil
}

func (r *tokenReinitialisationRepoFake) MarquerUtilise(_ context.Context, id string) error {
	t, ok := r.parID[id]
	if !ok || t.UtiliseAt != nil {
		return domain.ErrTokenInvalide
	}
	t.MarquerUtilise()
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

func setupAuthService() (input.AuthUseCase, *utilisateurRepoFake, *refreshTokenRepoFake, *tokenReinitialisationRepoFake, *notifieurFake) {
	utilisateurs := nouveauUtilisateurRepoFake()
	refreshTokens := nouveauRefreshTokenRepoFake()
	tokensReinitialisation := nouveauTokenReinitialisationRepoFake()
	notifieur := &notifieurFake{}
	service := application.NewAuthService(utilisateurs, refreshTokens, tokensReinitialisation, tokenGeneratorFake{}, notifieur, txManagerFake{})
	return service, utilisateurs, refreshTokens, tokensReinitialisation, notifieur
}

func creerUtilisateurTest(t *testing.T, repo *utilisateurRepoFake, email, motDePasse string) *domain.Utilisateur {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(motDePasse), bcrypt.DefaultCost)
	require.NoError(t, err)

	u, err := domain.NouveauUtilisateur("Koné", "Awa", email, "+2250700000000", "CI", string(hash))
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

func TestAuthService_Connexion_Succes(t *testing.T) {
	service, utilisateurs, refreshTokens, _, _ := setupAuthService()
	u := creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	assert.Equal(t, "access-"+u.ID, resultat.AccessToken)
	assert.NotEmpty(t, resultat.RefreshToken)

	// Le refresh token a bien été persisté (sous forme hachée).
	assert.Len(t, refreshTokens.parHash, 1)
}

func TestAuthService_Connexion_MotDePasseIncorrect(t *testing.T) {
	service, utilisateurs, _, _, _ := setupAuthService()
	creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	_, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "mauvais-mot-de-passe"})
	assert.ErrorIs(t, err, domain.ErrIdentifiantsInvalides)
}

func TestAuthService_Connexion_UtilisateurInconnu(t *testing.T) {
	service, _, _, _, _ := setupAuthService()

	_, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "inconnu@example.com", MotDePasse: "peu-importe"})
	// Ne doit jamais fuiter domain.ErrUtilisateurIntrouvable : même erreur
	// générique que pour un mot de passe incorrect.
	assert.ErrorIs(t, err, domain.ErrIdentifiantsInvalides)
}

func TestAuthService_RafraichirToken_Succes(t *testing.T) {
	service, utilisateurs, _, _, _ := setupAuthService()
	u := creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	session1, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	session2, err := service.RafraichirToken(context.Background(), session1.RefreshToken)
	require.NoError(t, err)
	assert.Equal(t, "access-"+u.ID, session2.AccessToken)
	assert.NotEqual(t, session1.RefreshToken, session2.RefreshToken, "rotation : un nouveau refresh token doit être émis")

	// L'ancien refresh token est révoqué (usage unique) : le réutiliser échoue.
	_, err = service.RafraichirToken(context.Background(), session1.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)
}

func TestAuthService_RafraichirToken_Invalide(t *testing.T) {
	service, _, _, _, _ := setupAuthService()

	_, err := service.RafraichirToken(context.Background(), "token-qui-n-existe-pas")
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)
}

func TestAuthService_Deconnexion(t *testing.T) {
	service, utilisateurs, _, _, _ := setupAuthService()
	creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	session, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	require.NoError(t, service.Deconnexion(context.Background(), session.RefreshToken))

	// Le refresh token révoqué ne peut plus servir à rafraîchir la session.
	_, err = service.RafraichirToken(context.Background(), session.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)

	// Déconnexion idempotente : rejouer sur un token déjà révoqué ne casse pas.
	assert.NoError(t, service.Deconnexion(context.Background(), session.RefreshToken))
}

func TestAuthService_DemanderReinitialisation_Succes(t *testing.T) {
	service, utilisateurs, _, tokensReinitialisation, notifieur := setupAuthService()
	creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	require.NoError(t, service.DemanderReinitialisation(context.Background(), "awa@example.com"))

	require.Len(t, notifieur.emailsEnvoyes, 1)
	assert.Equal(t, "awa@example.com", notifieur.emailsEnvoyes[0].destinataire)
	assert.Len(t, tokensReinitialisation.parHash, 1)
}

func TestAuthService_DemanderReinitialisation_EmailInconnu(t *testing.T) {
	service, _, _, _, notifieur := setupAuthService()

	// Aucune erreur, et surtout aucun email envoyé : ne révèle jamais si
	// l'email existe (évite l'énumération de comptes).
	require.NoError(t, service.DemanderReinitialisation(context.Background(), "inconnu@example.com"))
	assert.Empty(t, notifieur.emailsEnvoyes)
}

func TestAuthService_Reinitialiser_Succes(t *testing.T) {
	service, utilisateurs, _, _, notifieur := setupAuthService()
	creerUtilisateurTest(t, utilisateurs, "awa@example.com", "ancienmotdepasse123")

	session, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "ancienmotdepasse123"})
	require.NoError(t, err)

	require.NoError(t, service.DemanderReinitialisation(context.Background(), "awa@example.com"))
	require.Len(t, notifieur.emailsEnvoyes, 1)
	code := extraireCodeOTP(t, notifieur.emailsEnvoyes[0].corps)

	require.NoError(t, service.Reinitialiser(context.Background(), code, "nouveaumotdepasse456"))

	_, err = service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "ancienmotdepasse123"})
	assert.ErrorIs(t, err, domain.ErrIdentifiantsInvalides, "l'ancien mot de passe ne doit plus fonctionner")

	_, err = service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "nouveaumotdepasse456"})
	assert.NoError(t, err)

	// Toutes les sessions actives (dont celle d'avant la réinitialisation)
	// doivent être révoquées.
	_, err = service.RafraichirToken(context.Background(), session.RefreshToken)
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)

	// Le code est à usage unique.
	err = service.Reinitialiser(context.Background(), code, "autremotdepasse789")
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)
}

func TestAuthService_Reinitialiser_TokenInvalide(t *testing.T) {
	service, _, _, _, _ := setupAuthService()

	err := service.Reinitialiser(context.Background(), "000000", "nouveaumotdepasse456")
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)
}
