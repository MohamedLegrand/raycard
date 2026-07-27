package application_test

import (
	"context"
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

// --- faux TokenGenerator et RefreshTokenRepository, sans dépendance JWT/DB ---

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
	if rt, ok := r.parHash[tokenHash]; ok {
		return rt, nil
	}
	return nil, domain.ErrTokenInvalide
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

func setupAuthService() (input.AuthUseCase, *utilisateurRepoFake, *refreshTokenRepoFake) {
	utilisateurs := nouveauUtilisateurRepoFake()
	refreshTokens := nouveauRefreshTokenRepoFake()
	service := application.NewAuthService(utilisateurs, refreshTokens, tokenGeneratorFake{})
	return service, utilisateurs, refreshTokens
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

func TestAuthService_Connexion_Succes(t *testing.T) {
	service, utilisateurs, refreshTokens := setupAuthService()
	u := creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	resultat, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "motdepasse123"})
	require.NoError(t, err)

	assert.Equal(t, "access-"+u.ID, resultat.AccessToken)
	assert.NotEmpty(t, resultat.RefreshToken)

	// Le refresh token a bien été persisté (sous forme hachée).
	assert.Len(t, refreshTokens.parHash, 1)
}

func TestAuthService_Connexion_MotDePasseIncorrect(t *testing.T) {
	service, utilisateurs, _ := setupAuthService()
	creerUtilisateurTest(t, utilisateurs, "awa@example.com", "motdepasse123")

	_, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "awa@example.com", MotDePasse: "mauvais-mot-de-passe"})
	assert.ErrorIs(t, err, domain.ErrIdentifiantsInvalides)
}

func TestAuthService_Connexion_UtilisateurInconnu(t *testing.T) {
	service, _, _ := setupAuthService()

	_, err := service.Connexion(context.Background(), input.ConnexionRequest{Email: "inconnu@example.com", MotDePasse: "peu-importe"})
	// Ne doit jamais fuiter domain.ErrUtilisateurIntrouvable : même erreur
	// générique que pour un mot de passe incorrect.
	assert.ErrorIs(t, err, domain.ErrIdentifiantsInvalides)
}

func TestAuthService_RafraichirToken_Succes(t *testing.T) {
	service, utilisateurs, _ := setupAuthService()
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
	service, _, _ := setupAuthService()

	_, err := service.RafraichirToken(context.Background(), "token-qui-n-existe-pas")
	assert.ErrorIs(t, err, domain.ErrTokenInvalide)
}

func TestAuthService_Deconnexion(t *testing.T) {
	service, utilisateurs, _ := setupAuthService()
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
