// Package auth regroupe les interfaces (ports entrants) exposées par la
// couche application au transport pour l'authentification et le cycle
// de vie de la session.
package auth

import (
	"context"
	"time"

	"raycard/internal/core/domain/commun"
)

// ConnexionRequest transporte les identifiants bruts depuis le transport.
type ConnexionRequest struct {
	Email      string
	MotDePasse string
}

// SessionResultat regroupe l'access token et le refresh token émis lors
// d'une connexion ou d'un rafraîchissement. RefreshToken est la valeur
// brute (à usage unique côté client) — jamais celle stockée en base.
type SessionResultat struct {
	AccessToken          string
	AccessTokenExpireAt  time.Time
	RefreshToken         string
	RefreshTokenExpireAt time.Time
}

// ConnexionResultat est émis après vérification du mot de passe : la 2FA
// étant obligatoire pour tous les comptes (client comme admin), aucun
// access/refresh token n'est encore délivré ici — Ticket doit être
// présenté avec le code reçu par email, voir AuthUseCase.VerifierCode2FA.
type ConnexionResultat struct {
	Ticket        string
	ExpireDansSec int
}

// ConnexionGoogleRequest transporte l'ID token émis par Google, ainsi
// que téléphone/pays — nécessaires seulement si aucun compte n'existe
// encore pour cet utilisateur (création à la volée), ignorés sinon.
type ConnexionGoogleRequest struct {
	IDToken   string
	Telephone string
	PaysCode  string
}

// MetadonneesConnexion capture le contexte de la requête au moment de
// la vérification du second facteur, pour les notifications de
// sécurité (email de nouvelle connexion). Purement informatif, jamais
// utilisé pour une décision de sécurité.
type MetadonneesConnexion struct {
	AdresseIP    string
	AppareilInfo string
}

// EnregistrerAppareilRequest transporte la clé publique générée par
// l'appareil (après déverrouillage biométrique) à associer à
// l'utilisateur authentifié.
type EnregistrerAppareilRequest struct {
	ClePublique string // encodée en base64 (Ed25519, 32 octets)
	NomAppareil string
}

// AppareilResultat représente un appareil enregistré pour la connexion
// par empreinte, tel que renvoyé au client (jamais la clé publique :
// le client la connaît déjà, inutile de la renvoyer).
type AppareilResultat struct {
	ID          string
	NomAppareil string
	CreatedAt   time.Time
}

// ChallengeEmpreinteResultat est le nonce que le client doit signer
// avec la clé privée de l'appareil désigné pour obtenir une session.
type ChallengeEmpreinteResultat struct {
	ChallengeID   string
	Challenge     string
	ExpireDansSec int
}

// VerifierEmpreinteRequest transporte la signature du challenge,
// produite par la clé privée de l'appareil après déverrouillage
// biométrique.
type VerifierEmpreinteRequest struct {
	ChallengeID string
	Signature   string // encodée en base64
}

// ModifierProfilRequest transporte les champs de profil auto-gérables
// par l'utilisateur. L'email n'en fait pas partie : il suit son propre
// circuit de vérification (voir DemanderChangementEmail).
type ModifierProfilRequest struct {
	Nom    string
	Prenom string
}

// AuthUseCase orchestre l'authentification et le cycle de vie de la
// session (access + refresh token, rotation, révocation).
type AuthUseCase interface {
	// Connexion vérifie les identifiants et déclenche le second facteur
	// (envoi d'un code par email) : elle ne renvoie jamais directement de
	// session, la 2FA est obligatoire pour tout le monde.
	Connexion(ctx context.Context, req ConnexionRequest) (*ConnexionResultat, error)

	// VerifierCode2FA échange un ticket de connexion valide et son code
	// contre une session complète (access + refresh token). Envoie une
	// notification de connexion réussie, et une alerte si les tentatives
	// s'épuisent (voir application/auth/auth_service.go).
	VerifierCode2FA(ctx context.Context, ticket, code string, metadonnees MetadonneesConnexion) (*SessionResultat, error)

	// ConnexionGoogle authentifie (ou crée) un utilisateur à partir d'un
	// ID token Google déjà vérifié côté client, puis déclenche le même
	// second facteur que Connexion : ne renvoie jamais directement de
	// session, la 2FA est obligatoire pour tout le monde, y compris via
	// Google.
	ConnexionGoogle(ctx context.Context, req ConnexionGoogleRequest) (*ConnexionResultat, error)

	RafraichirToken(ctx context.Context, refreshToken string) (*SessionResultat, error)
	Deconnexion(ctx context.Context, refreshToken string) error

	// DemanderReinitialisation envoie un code de réinitialisation par
	// email si un compte existe pour cet email. Ne renvoie jamais d'erreur
	// pour un email inconnu (évite l'énumération de comptes) : c'est à
	// l'appelant HTTP de toujours répondre le même succès générique.
	DemanderReinitialisation(ctx context.Context, email string) error

	// Reinitialiser change le mot de passe si le code fourni est valide,
	// et révoque toutes les sessions actives de l'utilisateur, y compris
	// les appareils enregistrés pour la connexion par empreinte.
	// adresseIP alimente VerrouReinitialisation : contrairement à
	// MetadonneesConnexion (purement informatif), elle influence ici
	// directement une décision de sécurité (blocage après trop
	// d'échecs) — voir domain/auth.VerrouReinitialisation.
	Reinitialiser(ctx context.Context, token, nouveauMotDePasse, adresseIP string) error

	// EnregistrerAppareil associe une clé publique d'appareil à
	// l'utilisateur authentifié donné, pour une future connexion par
	// empreinte sur cet appareil.
	EnregistrerAppareil(ctx context.Context, utilisateurID string, req EnregistrerAppareilRequest) (*AppareilResultat, error)

	// RevoquerAppareil invalide définitivement un appareil enregistré
	// (perte, vol, ou l'utilisateur ne veut plus l'utiliser).
	RevoquerAppareil(ctx context.Context, utilisateurID, appareilID string) error

	// DemanderChallengeEmpreinte émet un nonce à signer par la clé privée
	// de l'appareil désigné, pour prouver sa possession sans transmettre
	// l'empreinte elle-même.
	DemanderChallengeEmpreinte(ctx context.Context, appareilID string) (*ChallengeEmpreinteResultat, error)

	// ConnexionEmpreinte échange un challenge signé contre une session
	// complète : contrairement à Connexion et ConnexionGoogle, aucun
	// second facteur par email n'est requis ici — l'appareil (clé privée
	// en zone sécurisée) et l'empreinte qui l'a déverrouillée constituent
	// déjà deux facteurs.
	ConnexionEmpreinte(ctx context.Context, req VerifierEmpreinteRequest, metadonnees MetadonneesConnexion) (*SessionResultat, error)

	// ObtenirProfil retourne le profil de l'utilisateur authentifié, sans
	// le modifier — contrairement à ModifierProfil ci-dessous, aucun
	// endpoint de lecture seule n'existait avant : le client n'avait
	// aucun moyen de connaître nom/prénom/email/kyc_tier après une
	// simple connexion (les flux de connexion ne renvoient qu'une
	// session, jamais le profil).
	ObtenirProfil(ctx context.Context, utilisateurID string) (*commun.Utilisateur, error)

	// ModifierProfil met à jour le nom et le prénom de l'utilisateur
	// authentifié.
	ModifierProfil(ctx context.Context, utilisateurID string, req ModifierProfilRequest) (*commun.Utilisateur, error)

	// ModifierPhotoProfil stocke la nouvelle photo de profil de
	// l'utilisateur authentifié.
	ModifierPhotoProfil(ctx context.Context, utilisateurID, nomFichier string, contenu []byte) (*commun.Utilisateur, error)

	// ObtenirPhotoProfil relit la photo de profil stockée (voir
	// StockageFichier.Lire) — sans cet endpoint, PhotoProfil n'était
	// qu'un chemin de fichier sur le disque du serveur, jamais
	// accessible par le client (aucune route statique ne l'exposait).
	// commun.ErrPhotoProfilAbsente si l'utilisateur n'en a pas encore.
	ObtenirPhotoProfil(ctx context.Context, utilisateurID string) (contenu []byte, contentType string, err error)

	// ChangerMotDePasse change le mot de passe de l'utilisateur
	// authentifié, après vérification du mot de passe actuel, et révoque
	// toutes les autres sessions actives.
	ChangerMotDePasse(ctx context.Context, utilisateurID, motDePasseActuel, nouveauMotDePasse string) error

	// DemanderChangementEmail envoie un code de confirmation au NOUVEL
	// email : le changement ne prend effet qu'après ConfirmerChangementEmail,
	// pour ne jamais risquer de perdre l'accès au compte sur une simple
	// faute de frappe ou une session compromise.
	DemanderChangementEmail(ctx context.Context, utilisateurID, nouvelEmail string) error

	// ConfirmerChangementEmail applique le changement d'email si le code
	// reçu au nouvel email est valide, et notifie l'ancien email du
	// changement.
	ConfirmerChangementEmail(ctx context.Context, code string) (*commun.Utilisateur, error)
}
