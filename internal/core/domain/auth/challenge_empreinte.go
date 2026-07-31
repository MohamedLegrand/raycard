package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// ChallengeEmpreinte est un nonce à usage unique que le client doit
// signer avec la clé privée de son appareil pour prouver sa possession
// et obtenir une session, sans repasser par le code de vérification par
// email.
type ChallengeEmpreinte struct {
	ID            string
	CleAppareilID string
	Nonce         string
	ExpireAt      time.Time
	UtiliseAt     *time.Time
	CreatedAt     time.Time
}

// NouveauChallengeEmpreinte crée un challenge valide pour la durée
// donnée, lié à un appareil déjà enregistré.
func NouveauChallengeEmpreinte(cleAppareilID, nonce string, duree time.Duration) (*ChallengeEmpreinte, error) {
	if cleAppareilID == "" || nonce == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if duree <= 0 {
		return nil, commun.ErrDonneesInvalides
	}

	return &ChallengeEmpreinte{
		ID:            commun.NewID(),
		CleAppareilID: cleAppareilID,
		Nonce:         nonce,
		ExpireAt:      time.Now().UTC().Add(duree),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// EstValide indique si le challenge peut encore être vérifié : ni déjà
// consommé, ni expiré.
func (c *ChallengeEmpreinte) EstValide() bool {
	if c.UtiliseAt != nil {
		return false
	}
	return time.Now().UTC().Before(c.ExpireAt)
}

// Consommer marque le challenge comme utilisé (usage unique, à l'issue
// d'une vérification de signature réussie).
func (c *ChallengeEmpreinte) Consommer() {
	maintenant := time.Now().UTC()
	c.UtiliseAt = &maintenant
}
