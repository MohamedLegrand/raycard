package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/bcrypt"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/core/ports/output"
)

const (
	dureeRefreshToken          = 30 * 24 * time.Hour
	dureeTokenReinitialisation = 15 * time.Minute
	sujetEmailReinitialisation = "Réinitialisation de votre mot de passe RAYCARD"

	dureeTicketConnexion         = 10 * time.Minute
	tentativesMaxCodeConnexion   = 5
	sujetEmailConnexion          = "Votre code de connexion RAYCARD"
	sujetEmailNouvelleConnexion  = "Nouvelle connexion à votre compte RAYCARD"
	sujetEmailTentativesEpuisees = "Alerte de sécurité RAYCARD : tentatives de connexion échouées"
)

type authService struct {
	utilisateurs           output.UtilisateurRepository
	wallets                output.WalletRepository
	reglesKyc              output.ReglesKycRepository
	refreshTokens          output.RefreshTokenRepository
	tokensReinitialisation output.TokenReinitialisationRepository
	ticketsConnexion       output.TicketConnexionRepository
	tokenGenerator         output.TokenGenerator
	notifieur              output.Notifieur
	googleAuthProvider     output.GoogleAuthProvider
	txManager              output.TxManager
}

// NewAuthService construit l'implémentation de input.AuthUseCase.
func NewAuthService(
	utilisateurs output.UtilisateurRepository,
	wallets output.WalletRepository,
	reglesKyc output.ReglesKycRepository,
	refreshTokens output.RefreshTokenRepository,
	tokensReinitialisation output.TokenReinitialisationRepository,
	ticketsConnexion output.TicketConnexionRepository,
	tokenGenerator output.TokenGenerator,
	notifieur output.Notifieur,
	googleAuthProvider output.GoogleAuthProvider,
	txManager output.TxManager,
) input.AuthUseCase {
	return &authService{
		utilisateurs:           utilisateurs,
		wallets:                wallets,
		reglesKyc:              reglesKyc,
		refreshTokens:          refreshTokens,
		tokensReinitialisation: tokensReinitialisation,
		ticketsConnexion:       ticketsConnexion,
		tokenGenerator:         tokenGenerator,
		notifieur:              notifieur,
		googleAuthProvider:     googleAuthProvider,
		txManager:              txManager,
	}
}

// Connexion vérifie le mot de passe puis déclenche systématiquement le
// second facteur : aucune session n'est émise ici, quel que soit le
// rôle de l'utilisateur (2FA obligatoire pour tout le monde, sans
// exception).
func (s *authService) Connexion(ctx context.Context, req input.ConnexionRequest) (*input.ConnexionResultat, error) {
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

	return s.demarrerTicketConnexion(ctx, utilisateur)
}

// demarrerTicketConnexion émet le ticket + code du second facteur, une
// fois l'identité déjà établie (mot de passe vérifié, ou identité
// Google vérifiée) — commun à Connexion et ConnexionGoogle : la 2FA est
// obligatoire pour tout le monde, quel que soit le chemin emprunté.
func (s *authService) demarrerTicketConnexion(ctx context.Context, utilisateur *domain.Utilisateur) (*input.ConnexionResultat, error) {
	ticketBrut, err := genererSecretAleatoire()
	if err != nil {
		return nil, fmt.Errorf("génération ticket connexion: %w", err)
	}
	code, err := genererCodeOTP()
	if err != nil {
		return nil, fmt.Errorf("génération code: %w", err)
	}

	ticket, err := domain.NouveauTicketConnexion(
		utilisateur.ID, hacherToken(ticketBrut), hacherToken(code), tentativesMaxCodeConnexion, dureeTicketConnexion,
	)
	if err != nil {
		return nil, err
	}
	if err := s.ticketsConnexion.Create(ctx, ticket); err != nil {
		return nil, fmt.Errorf("persistance ticket connexion: %w", err)
	}

	corps := fmt.Sprintf(
		"<p>Voici votre code de connexion RAYCARD : <strong>%s</strong></p>"+
			"<p>Ce code expire dans 10 minutes. Si vous n'êtes pas à l'origine de cette connexion, sécurisez immédiatement votre compte.</p>",
		code,
	)
	if err := s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailConnexion, corps); err != nil {
		return nil, fmt.Errorf("envoi email connexion: %w", err)
	}

	return &input.ConnexionResultat{
		Ticket:        ticketBrut,
		ExpireDansSec: int(dureeTicketConnexion.Seconds()),
	}, nil
}

// VerifierCode2FA échange un ticket de connexion valide et son code
// contre une session complète. Un code incorrect consomme une
// tentative ; à zéro, le ticket est définitivement mort (il faut
// recommencer depuis Connexion) et une alerte de sécurité est envoyée.
// Une connexion réussie déclenche elle aussi une notification.
func (s *authService) VerifierCode2FA(ctx context.Context, ticketBrut, code string, metadonnees input.MetadonneesConnexion) (*input.SessionResultat, error) {
	ticket, err := s.ticketsConnexion.FindByHash(ctx, hacherToken(ticketBrut))
	if err != nil {
		if errors.Is(err, domain.ErrTokenInvalide) {
			return nil, domain.ErrTokenInvalide
		}
		return nil, fmt.Errorf("recherche ticket connexion: %w", err)
	}
	if !ticket.EstValide() {
		return nil, domain.ErrTokenInvalide
	}

	// Comparaison en temps constant : le code n'a que 10^6 combinaisons
	// possibles, une comparaison naïve (==) laisserait fuir un signal de
	// timing exploitable sur un secret aussi court.
	if subtle.ConstantTimeCompare([]byte(hacherToken(code)), []byte(ticket.CodeHash)) != 1 {
		ticket.EnregistrerEchec()
		if err := s.ticketsConnexion.Update(ctx, ticket); err != nil {
			return nil, fmt.Errorf("mise à jour ticket connexion: %w", err)
		}
		if ticket.TentativesRestantes == 0 {
			// Signal fort : quelqu'un connaît déjà le mot de passe et essaie
			// de deviner le code. Best-effort — un échec d'envoi ne doit pas
			// masquer l'erreur d'authentification déjà décidée.
			_ = s.alerterTentativesEpuisees(ctx, ticket.UtilisateurID)
		}
		return nil, domain.ErrTokenInvalide
	}

	utilisateur, err := s.utilisateurs.FindByID(ctx, ticket.UtilisateurID)
	if err != nil {
		return nil, fmt.Errorf("recherche utilisateur: %w", err)
	}

	ticket.Consommer()
	if err := s.ticketsConnexion.Update(ctx, ticket); err != nil {
		return nil, fmt.Errorf("consommation ticket connexion: %w", err)
	}

	session, err := s.emettreSession(ctx, utilisateur)
	if err != nil {
		return nil, err
	}

	// Best-effort, même logique : ne bloque jamais une connexion réussie.
	_ = s.notifierConnexionReussie(ctx, utilisateur, metadonnees)

	return session, nil
}

// notifierConnexionReussie envoie un email de confirmation à chaque
// connexion effective — permet à l'utilisateur de repérer rapidement un
// accès qu'il n'a pas initié.
func (s *authService) notifierConnexionReussie(ctx context.Context, utilisateur *domain.Utilisateur, metadonnees input.MetadonneesConnexion) error {
	corps := fmt.Sprintf(
		"<p>Une connexion à votre compte RAYCARD vient d'avoir lieu.</p>"+
			"<p>Adresse IP : %s<br>Appareil : %s</p>"+
			"<p>Si ce n'est pas vous, réinitialisez votre mot de passe immédiatement.</p>",
		valeurOuDefaut(metadonnees.AdresseIP, "inconnue"),
		valeurOuDefaut(metadonnees.AppareilInfo, "inconnu"),
	)
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailNouvelleConnexion, corps)
}

// alerterTentativesEpuisees prévient l'utilisateur que les 5 tentatives
// de code ont été consommées sans succès — le mot de passe seul ne
// suffisant pas à se connecter, ceci indique que quelqu'un d'autre le
// connaît probablement.
func (s *authService) alerterTentativesEpuisees(ctx context.Context, utilisateurID string) error {
	utilisateur, err := s.utilisateurs.FindByID(ctx, utilisateurID)
	if err != nil {
		return fmt.Errorf("recherche utilisateur pour alerte: %w", err)
	}

	corps := "<p>Les 5 tentatives de code pour une connexion à votre compte RAYCARD ont échoué.</p>" +
		"<p>Cela signifie que votre mot de passe est probablement connu de quelqu'un d'autre. " +
		"Si ce n'est pas vous qui avez tenté de vous connecter, réinitialisez votre mot de passe immédiatement.</p>"
	return s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailTentativesEpuisees, corps)
}

// valeurOuDefaut renvoie v s'il est renseigné, sinon defaut — évite un
// email affichant un champ vide quand la métadonnée n'a pas été
// transmise.
func valeurOuDefaut(v, defaut string) string {
	if v == "" {
		return defaut
	}
	return v
}

// ConnexionGoogle authentifie ou crée un utilisateur à partir d'un ID
// token Google (trois cas, dans l'ordre : compte déjà lié à ce compte
// Google ; compte existant avec cet email, liaison automatique si
// Google confirme l'email vérifié ; aucun compte, création), puis
// déclenche le même second facteur que Connexion.
func (s *authService) ConnexionGoogle(ctx context.Context, req input.ConnexionGoogleRequest) (*input.ConnexionResultat, error) {
	identite, err := s.googleAuthProvider.VerifierIDToken(ctx, req.IDToken)
	if err != nil {
		return nil, domain.ErrIdentifiantsInvalides
	}

	utilisateur, err := s.utilisateurs.FindByGoogleID(ctx, identite.GoogleID)
	if err == nil {
		return s.demarrerTicketConnexion(ctx, utilisateur)
	}
	if !errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		return nil, fmt.Errorf("recherche utilisateur par google id: %w", err)
	}

	utilisateur, err = s.utilisateurs.FindByEmail(ctx, identite.Email)
	if err == nil {
		// Sans cette garantie, n'importe qui pourrait créer un compte
		// Google avec l'email de quelqu'un d'autre et prendre le contrôle
		// de son compte RAYCARD existant.
		if !identite.EmailVerifie {
			return nil, domain.ErrIdentifiantsInvalides
		}
		if err := utilisateur.LierGoogleID(identite.GoogleID); err != nil {
			return nil, err
		}
		if err := s.utilisateurs.LierGoogleID(ctx, utilisateur); err != nil {
			return nil, fmt.Errorf("liaison compte google: %w", err)
		}
		return s.demarrerTicketConnexion(ctx, utilisateur)
	}
	if !errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		return nil, fmt.Errorf("recherche utilisateur par email: %w", err)
	}

	nouvelUtilisateur, err := s.creerUtilisateurGoogle(ctx, identite, req.Telephone, req.PaysCode)
	if err != nil {
		return nil, err
	}
	return s.demarrerTicketConnexion(ctx, nouvelUtilisateur)
}

// creerUtilisateurGoogle crée un nouvel utilisateur (et son wallet, KYC
// Tier 1 auto-validé) lors d'une toute première connexion Google — même
// logique que KycUseCase.Inscrire, adaptée à l'absence de mot de passe.
func (s *authService) creerUtilisateurGoogle(ctx context.Context, identite output.IdentiteGoogle, telephone, paysCode string) (*domain.Utilisateur, error) {
	if telephone == "" || paysCode == "" {
		return nil, domain.ErrDonneesInvalides
	}

	regle, err := s.reglesKyc.FindByPaysEtTier(ctx, paysCode, domain.KycTier1)
	if err != nil {
		return nil, err
	}

	utilisateur, err := domain.NouvelUtilisateurGoogle(identite.Nom, identite.Prenom, identite.Email, telephone, paysCode, identite.GoogleID)
	if err != nil {
		return nil, err
	}
	if err := utilisateur.ValiderKycTier1(); err != nil {
		return nil, err
	}

	wallet, err := domain.NouveauWallet(utilisateur.ID, utilisateur.PaysCode, regle.Devise, regle.PlafondSoldeCentimes)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.utilisateurs.Create(ctx, utilisateur); err != nil {
			return fmt.Errorf("création utilisateur: %w", err)
		}
		if err := s.wallets.Create(ctx, wallet); err != nil {
			return fmt.Errorf("création wallet: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return utilisateur, nil
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

	// Le rôle a pu changer depuis l'émission du refresh token : on relit
	// l'utilisateur plutôt que de faire confiance à une information figée.
	utilisateur, err := s.utilisateurs.FindByID(ctx, rt.UtilisateurID)
	if err != nil {
		if errors.Is(err, domain.ErrUtilisateurIntrouvable) {
			return nil, domain.ErrTokenInvalide
		}
		return nil, fmt.Errorf("recherche utilisateur: %w", err)
	}

	// Rotation : ce refresh token est à usage unique, il est révoqué dès
	// qu'il sert à émettre une nouvelle session.
	if err := s.refreshTokens.Revoke(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("révocation refresh token: %w", err)
	}

	return s.emettreSession(ctx, utilisateur)
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

func (s *authService) DemanderReinitialisation(ctx context.Context, email string) error {
	utilisateur, err := s.utilisateurs.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUtilisateurIntrouvable) {
			return nil // réponse générique : ne révèle jamais si l'email existe
		}
		return fmt.Errorf("recherche utilisateur: %w", err)
	}

	code, err := genererCodeOTP()
	if err != nil {
		return fmt.Errorf("génération code: %w", err)
	}

	token, err := domain.NouveauTokenReinitialisation(utilisateur.ID, hacherToken(code), dureeTokenReinitialisation)
	if err != nil {
		return err
	}
	if err := s.tokensReinitialisation.Create(ctx, token); err != nil {
		return fmt.Errorf("persistance token réinitialisation: %w", err)
	}

	corps := fmt.Sprintf(
		"<p>Voici votre code de réinitialisation RAYCARD : <strong>%s</strong></p>"+
			"<p>Ce code expire dans 15 minutes. Si vous n'êtes pas à l'origine de cette demande, ignorez cet email.</p>",
		code,
	)
	if err := s.notifieur.EnvoyerEmail(ctx, utilisateur.Email, sujetEmailReinitialisation, corps); err != nil {
		return fmt.Errorf("envoi email réinitialisation: %w", err)
	}

	return nil
}

func (s *authService) Reinitialiser(ctx context.Context, token, nouveauMotDePasse string) error {
	tr, err := s.tokensReinitialisation.FindByHash(ctx, hacherToken(token))
	if err != nil {
		if errors.Is(err, domain.ErrTokenInvalide) {
			return domain.ErrTokenInvalide
		}
		return fmt.Errorf("recherche token réinitialisation: %w", err)
	}
	if !tr.EstValide() {
		return domain.ErrTokenInvalide
	}

	utilisateur, err := s.utilisateurs.FindByID(ctx, tr.UtilisateurID)
	if err != nil {
		return fmt.Errorf("recherche utilisateur: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(nouveauMotDePasse), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hachage mot de passe: %w", err)
	}
	utilisateur.MotDePasseHash = string(hash)

	// L'état "utilisé" est possédé par le repository (comme pour
	// RefreshToken.Revoke) : on ne mute pas tr en mémoire avant l'appel,
	// pour ne pas fausser la vérification faite côté persistance.
	//
	// Le mot de passe, la consommation du token et la révocation de
	// toutes les sessions actives doivent réussir ensemble.
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.utilisateurs.UpdateMotDePasse(ctx, utilisateur); err != nil {
			return fmt.Errorf("mise à jour mot de passe: %w", err)
		}
		if err := s.tokensReinitialisation.MarquerUtilise(ctx, tr.ID); err != nil {
			return fmt.Errorf("consommation token réinitialisation: %w", err)
		}
		if err := s.refreshTokens.RevokeAllForUtilisateur(ctx, utilisateur.ID); err != nil {
			return fmt.Errorf("révocation sessions actives: %w", err)
		}
		return nil
	})
}

func (s *authService) emettreSession(ctx context.Context, utilisateur *domain.Utilisateur) (*input.SessionResultat, error) {
	accessToken, accessExpireAt, err := s.tokenGenerator.GenererAccessToken(output.Claims{
		UtilisateurID: utilisateur.ID,
		Role:          utilisateur.Role,
	})
	if err != nil {
		return nil, fmt.Errorf("génération access token: %w", err)
	}

	refreshTokenBrut, err := genererSecretAleatoire()
	if err != nil {
		return nil, fmt.Errorf("génération refresh token: %w", err)
	}

	rt, err := domain.NouveauRefreshToken(utilisateur.ID, hacherToken(refreshTokenBrut), dureeRefreshToken)
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

// genererCodeOTP produit un code à 6 chiffres (crypto/rand) pour la
// réinitialisation de mot de passe. Entropie volontairement plus faible
// que le refresh token (10^6 possibilités) car pensé pour être saisi à
// la main sur mobile — compensée par une durée de vie courte et un
// usage unique.
func genererCodeOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
