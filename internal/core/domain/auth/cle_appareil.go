package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// CleAppareil est la clé publique d'un appareil enregistré pour la
// connexion par empreinte digitale. L'empreinte elle-même ne quitte
// jamais l'appareil (zone sécurisée du téléphone) : seule la clé
// publique de la paire générée après déverrouillage biométrique est
// connue du serveur, qui vérifie ensuite des signatures produites par
// la clé privée correspondante.
type CleAppareil struct {
	ID                    string
	UtilisateurID         string
	ClePublique           string // encodée en base64 (Ed25519, 32 octets)
	NomAppareil           string
	DerniereUtilisationAt *time.Time
	RevokedAt             *time.Time
	CreatedAt             time.Time
}

// NouvelleCleAppareil enregistre un nouvel appareil pour la connexion
// par empreinte.
func NouvelleCleAppareil(utilisateurID, clePublique, nomAppareil string) (*CleAppareil, error) {
	if utilisateurID == "" || clePublique == "" || nomAppareil == "" {
		return nil, commun.ErrDonneesInvalides
	}

	return &CleAppareil{
		ID:            commun.NewID(),
		UtilisateurID: utilisateurID,
		ClePublique:   clePublique,
		NomAppareil:   nomAppareil,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// EstValide indique si cette clé peut encore servir à se connecter :
// non révoquée (perte/vol de l'appareil, ou révocation en masse lors
// d'une réinitialisation de mot de passe).
func (c *CleAppareil) EstValide() bool {
	return c.RevokedAt == nil
}

// Revoquer invalide définitivement cet appareil pour la connexion par
// empreinte.
func (c *CleAppareil) Revoquer() {
	maintenant := time.Now().UTC()
	c.RevokedAt = &maintenant
}

// MarquerUtilisee met à jour l'horodatage de dernière utilisation,
// après une connexion réussie via cet appareil.
func (c *CleAppareil) MarquerUtilisee() {
	maintenant := time.Now().UTC()
	c.DerniereUtilisationAt = &maintenant
}
