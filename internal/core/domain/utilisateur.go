package domain

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

var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// Utilisateur est l'entité centrale du domaine : un client RAYCARD,
// avec son statut KYC et son pays de rattachement (le multi-pays est
// une exigence dès la V1).
type Utilisateur struct {
	ID              string
	Nom             string
	Prenom          string
	Email           string
	Telephone       string
	PaysCode        string // ISO 3166-1 alpha-2, ex: "CI", "SN"
	MotDePasseHash  string
	KycTier         KycTier
	KycStatut       KycStatut
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NouveauUtilisateur construit un utilisateur valide, prêt à être
// persisté. Le mot de passe doit déjà être haché par l'appelant
// (l'application, jamais le domaine, ne connaît de bibliothèque de
// hachage).
func NouveauUtilisateur(nom, prenom, email, telephone, paysCode, motDePasseHash string) (*Utilisateur, error) {
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
	if motDePasseHash == "" {
		return nil, ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()

	return &Utilisateur{
		ID:             NewID(),
		Nom:            nom,
		Prenom:         prenom,
		Email:          email,
		Telephone:      telephone,
		PaysCode:       paysCode,
		MotDePasseHash: motDePasseHash,
		KycTier:        KycTierAucun,
		KycStatut:      KycStatutEnAttente,
		CreatedAt:      maintenant,
		UpdatedAt:      maintenant,
	}, nil
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
