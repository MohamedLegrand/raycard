// Package google implémente output.GoogleAuthProvider en vérifiant la
// signature d'un ID token directement contre les clés publiques
// publiées par Google (JWKS), avec golang-jwt (déjà une dépendance du
// projet) — plutôt que le client Google Cloud complet
// (google.golang.org/api), qui embarque gRPC et OpenTelemetry pour un
// besoin qui ne demande qu'une vérification de signature JWT.
package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"raycard/internal/core/ports/output"
)

const urlClesGoogle = "https://www.googleapis.com/oauth2/v3/certs"

// dureeCacheCles : Google fait tourner ses clés peu souvent : un cache
// d'une heure évite un aller-retour réseau à chaque connexion Google
// sans risquer de garder une clé révoquée trop longtemps.
const dureeCacheCles = time.Hour

var issuersValides = map[string]bool{
	"https://accounts.google.com": true,
	"accounts.google.com":         true,
}

type VerificateurToken struct {
	clientID string
	client   *http.Client

	mu         sync.Mutex
	cles       map[string]*rsa.PublicKey
	clesExpire time.Time
}

func NewVerificateurToken(clientID string) *VerificateurToken {
	return &VerificateurToken{
		clientID: clientID,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *VerificateurToken) VerifierIDToken(ctx context.Context, idTokenBrut string) (output.IdentiteGoogle, error) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(idTokenBrut, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("id token sans kid")
		}
		return v.cle(ctx, kid)
	})
	if err != nil || !token.Valid {
		return output.IdentiteGoogle{}, fmt.Errorf("id token google invalide: %w", err)
	}

	iss, _ := claims["iss"].(string)
	if !issuersValides[iss] {
		return output.IdentiteGoogle{}, errors.New("émetteur id token invalide")
	}
	if aud, _ := claims["aud"].(string); aud != v.clientID {
		return output.IdentiteGoogle{}, errors.New("audience id token invalide")
	}

	sub, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	if sub == "" || email == "" {
		return output.IdentiteGoogle{}, errors.New("id token incomplet")
	}
	emailVerifie, _ := claims["email_verified"].(bool)
	prenom, _ := claims["given_name"].(string)
	nom, _ := claims["family_name"].(string)

	return output.IdentiteGoogle{
		GoogleID:     sub,
		Email:        email,
		EmailVerifie: emailVerifie,
		Nom:          nom,
		Prenom:       prenom,
	}, nil
}

func (v *VerificateurToken) cle(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if time.Now().Before(v.clesExpire) {
		if cle, ok := v.cles[kid]; ok {
			return cle, nil
		}
	}

	if err := v.rafraichirCles(ctx); err != nil {
		return nil, err
	}
	cle, ok := v.cles[kid]
	if !ok {
		return nil, fmt.Errorf("clé google introuvable pour kid=%s", kid)
	}
	return cle, nil
}

type jwks struct {
	Keys []struct {
		Kid string `json:"kid"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *VerificateurToken) rafraichirCles(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlClesGoogle, nil)
	if err != nil {
		return fmt.Errorf("construction requête clés google: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("récupération clés google: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("récupération clés google: statut %d", resp.StatusCode)
	}

	var ensemble jwks
	if err := json.NewDecoder(resp.Body).Decode(&ensemble); err != nil {
		return fmt.Errorf("décodage clés google: %w", err)
	}

	cles := make(map[string]*rsa.PublicKey, len(ensemble.Keys))
	for _, k := range ensemble.Keys {
		pub, err := construireCleRSA(k.N, k.E)
		if err != nil {
			continue
		}
		cles[k.Kid] = pub
	}

	v.cles = cles
	v.clesExpire = time.Now().Add(dureeCacheCles)
	return nil
}

func construireCleRSA(nBase64, eBase64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nBase64)
	if err != nil {
		return nil, fmt.Errorf("décodage module clé: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eBase64)
	if err != nil {
		return nil, fmt.Errorf("décodage exposant clé: %w", err)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(new(big.Int).SetBytes(eBytes).Int64()),
	}, nil
}
