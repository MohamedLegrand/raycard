package commun

import (
	"regexp"
	"strings"
	"time"
)

// KycTier représente le palier de vérification d'identité atteint par
// l'utilisateur. Les plafonds associés à chaque palier ne sont jamais
// codés en dur : ils sont lus depuis la table regles_kyc_pays (voir
// RegleKyc), car ils varient par pays.
type KycTier int

const (
	KycTierAucun KycTier = iota
	KycTier1
	KycTier2
)

// KycStatut représente l'état du dossier KYC de l'utilisateur.
type KycStatut string

const (
	KycStatutEnAttente KycStatut = "en_attente"
	KycStatutVerifie   KycStatut = "verifie"
	KycStatutRejete    KycStatut = "rejete"
)

// RoleUtilisateur distingue un client d'un administrateur RAYCARD. Les
// deux partagent la même entité et le même mécanisme de connexion :
// seul ce champ change leurs permissions (voir middleware.RequireAdmin).
type RoleUtilisateur string

const (
	RoleClient RoleUtilisateur = "utilisateur"
	RoleAdmin  RoleUtilisateur = "admin"
)

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Utilisateur est l'entité centrale du domaine : un client RAYCARD,
// avec son statut KYC et son pays de rattachement (le multi-pays est
// une exigence dès la V1).
type Utilisateur struct {
	ID             string
	Nom            string
	Prenom         string
	Email          string
	Telephone      string
	PaysCode       string // ISO 3166-1 alpha-2, ex: "CI", "SN"
	MotDePasseHash string // vide pour un compte connecté uniquement via Google
	GoogleID       string // vide si aucun compte Google lié
	Role           RoleUtilisateur
	KycTier        KycTier
	KycStatut      KycStatut
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// EstAdmin indique si l'utilisateur a les permissions d'administration
// (accès au back-office).
func (u *Utilisateur) EstAdmin() bool {
	return u.Role == RoleAdmin
}

// nouvelUtilisateurBase valide et normalise les champs communs à toute
// création d'utilisateur, quelle que soit la méthode d'authentification
// (mot de passe ou Google) — c'est aux constructeurs spécifiques de
// renseigner MotDePasseHash ou GoogleID.
func nouvelUtilisateurBase(nom, prenom, email, telephone, paysCode string) (*Utilisateur, error) {
	nom = strings.TrimSpace(nom)
	prenom = strings.TrimSpace(prenom)
	email = strings.ToLower(strings.TrimSpace(email))
	telephone = strings.TrimSpace(telephone)
	paysCode = strings.ToUpper(strings.TrimSpace(paysCode))

	if nom == "" || prenom == "" {
		return nil, ErrDonneesInvalides
	}
	if !emailRegex.MatchString(email) {
		return nil, ErrDonneesInvalides
	}
	if telephone == "" {
		return nil, ErrDonneesInvalides
	}
	if len(paysCode) != 2 {
		return nil, ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()

	return &Utilisateur{
		ID:        NewID(),
		Nom:       nom,
		Prenom:    prenom,
		Email:     email,
		Telephone: telephone,
		PaysCode:  paysCode,
		Role:      RoleClient,
		KycTier:   KycTierAucun,
		KycStatut: KycStatutEnAttente,
		CreatedAt: maintenant,
		UpdatedAt: maintenant,
	}, nil
}

// NouveauUtilisateur construit un utilisateur valide, prêt à être
// persisté. Le mot de passe doit déjà être haché par l'appelant
// (l'application, jamais le domaine, ne connaît de bibliothèque de
// hachage).
func NouveauUtilisateur(nom, prenom, email, telephone, paysCode, motDePasseHash string) (*Utilisateur, error) {
	if motDePasseHash == "" {
		return nil, ErrDonneesInvalides
	}
	u, err := nouvelUtilisateurBase(nom, prenom, email, telephone, paysCode)
	if err != nil {
		return nil, err
	}
	u.MotDePasseHash = motDePasseHash
	return u, nil
}

// NouvelUtilisateurGoogle construit un utilisateur authentifié
// uniquement via Google : aucun mot de passe n'est défini, la connexion
// ne sera jamais possible par ce biais pour ce compte (voir
// AuthUseCase.ConnexionGoogle).
func NouvelUtilisateurGoogle(nom, prenom, email, telephone, paysCode, googleID string) (*Utilisateur, error) {
	if googleID == "" {
		return nil, ErrDonneesInvalides
	}
	u, err := nouvelUtilisateurBase(nom, prenom, email, telephone, paysCode)
	if err != nil {
		return nil, err
	}
	u.GoogleID = googleID
	return u, nil
}

// LierGoogleID associe un compte Google à un utilisateur existant (créé
// initialement par mot de passe). Voir AuthUseCase.ConnexionGoogle pour
// les conditions de sécurité entourant cette liaison (email vérifié par
// Google).
func (u *Utilisateur) LierGoogleID(googleID string) error {
	if googleID == "" {
		return ErrDonneesInvalides
	}
	u.GoogleID = googleID
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// ValiderKycTier1 fait passer l'utilisateur au palier 1 (KYC léger :
// identité déclarative, sans document). Dans la réglementation type
// UEMOA, ce palier peut être validé immédiatement à l'inscription.
func (u *Utilisateur) ValiderKycTier1() error {
	if u.KycStatut != KycStatutEnAttente {
		return ErrTransitionKycInvalide
	}
	u.KycTier = KycTier1
	u.KycStatut = KycStatutVerifie
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// PasserAuTier2 fait passer l'utilisateur au palier 2, à l'issue de
// l'approbation d'un DossierKyc par un administrateur. Contrairement au
// Tier 1, ce palier n'est jamais auto-validé : il exige une revue
// humaine (voir DossierKyc).
func (u *Utilisateur) PasserAuTier2() error {
	if u.KycTier != KycTier1 {
		return ErrTransitionKycInvalide
	}
	u.KycTier = KycTier2
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// NouvelAdministrateur construit un compte administrateur RAYCARD. Il
// partage l'entité Utilisateur (même mécanisme de connexion), mais le
// concept de palier KYC ne s'applique pas à lui : il est créé
// directement vérifié, sans wallet associé (voir cmd/creer-admin).
func NouvelAdministrateur(nom, prenom, email, telephone, paysCode, motDePasseHash string) (*Utilisateur, error) {
	admin, err := NouveauUtilisateur(nom, prenom, email, telephone, paysCode, motDePasseHash)
	if err != nil {
		return nil, err
	}
	admin.Role = RoleAdmin
	admin.KycTier = KycTier2
	admin.KycStatut = KycStatutVerifie
	return admin, nil
}
