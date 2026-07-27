package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/core/ports/output"
)

const (
	dureeRefreshToken = 30 * 24 * time.Hour
)

type authService struct {
	utilisateurs   output.UtilisateurRepository
	refreshTokens  output.RefreshTokenRepository
	tokenGenerator output.TokenGenerator
}

// NewAuthService construit l'implémentation de input.AuthUseCase.
func NewAuthService(
	utilisateurs output.UtilisateurRepository,
	refreshTokens output.RefreshTokenRepository,
	tokenGenerator output.TokenGenerator,
) input.AuthUseCase {
	return &authService{
		utilisateurs:   utilisateurs,
		refreshTokens:  refreshTokens,
		tokenGenerator: tokenGenerator,
	}
}

func (s *authService) Connexion(ctx context.Context, req input.ConnexionRequest) (*input.SessionResultat, error) {
	utilisateur, err := s.utilisateurs.FindByEmail(ctx, req.Email)
	if errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		return nil, domain.ErrIdentifiantsInvalides
	}
	if err != nil {
		return nil, fmt.Errorf("recherche utilisateur: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(utilisateur.MotDePasseHash), []byte(req.MotDePasse)); err != nil {
		return nil, domain.ErrIdentifiantsInvalides
	}

	return s.emettreSession(ctx, utilisateur.ID)
}

func (s *authService) RafraichirToken(ctx context.Context, refreshToken string) (*input.SessionResultat, error) {
	rt, err := s.refreshTokens.FindByHash(ctx, hacherToken(refreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrTokenInvalide) {
			return nil, domain.ErrTokenInvalide
		}
		return nil, fmt.Errorf("recherche refresh token: %w", err)
	}
	if !rt.EstValide() {
		return nil, domain.ErrTokenInvalide
	}

	// Rotation : ce refresh token est à usage unique, il est révoqué dès
	// qu'il sert à émettre une nouvelle session.
	if err := s.refreshTokens.Revoke(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("révocation refresh token: %w", err)
	}

	return s.emettreSession(ctx, rt.UtilisateurID)
}

func (s *authService) Deconnexion(ctx context.Context, refreshToken string) error {
	rt, err := s.refreshTokens.FindByHash(ctx, hacherToken(refreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrTokenInvalide) {
			return nil // déjà invalide/inconnu : déconnexion idempotente
		}
		return fmt.Errorf("recherche refresh token: %w", err)
	}

	if err := s.refreshTokens.Revoke(ctx, rt.ID); err != nil {
		if errors.Is(err, domain.ErrTokenInvalide) {
			return nil // déjà révoqué entre-temps : déconnexion idempotente
		}
		return fmt.Errorf("révocation refresh token: %w", err)
	}
	return nil
}

func (s *authService) emettreSession(ctx context.Context, utilisateurID string) (*input.SessionResultat, error) {
	accessToken, accessExpireAt, err := s.tokenGenerator.GenererAccessToken(utilisateurID)
	if err != nil {
		return nil, fmt.Errorf("génération access token: %w", err)
	}

	refreshTokenBrut, err := genererSecretAleatoire()
	if err != nil {
		return nil, fmt.Errorf("génération refresh token: %w", err)
	}

	rt, err := domain.NouveauRefreshToken(utilisateurID, hacherToken(refreshTokenBrut), dureeRefreshToken)
	if err != nil {
		return nil, err
	}
	if err := s.refreshTokens.Create(ctx, rt); err != nil {
		return nil, fmt.Errorf("persistance refresh token: %w", err)
	}

	return &input.SessionResultat{
		AccessToken:          accessToken,
		AccessTokenExpireAt:  accessExpireAt,
		RefreshToken:         refreshTokenBrut,
		RefreshTokenExpireAt: rt.ExpireAt,
	}, nil
}

// genererSecretAleatoire produit la valeur brute du refresh token :
// 32 octets de crypto/rand encodés en hexadécimal.
func genererSecretAleatoire() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hacherToken calcule l'empreinte SHA-256 stockée en base : jamais la
// valeur brute du refresh token n'est persistée.
func hacherToken(token string) string {
	somme := sha256.Sum256([]byte(token))
	return hex.EncodeToString(somme[:])
}
