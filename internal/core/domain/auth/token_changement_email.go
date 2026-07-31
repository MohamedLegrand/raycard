package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// TokenChangementEmail est un code de confirmation de changement
// d'email : envoyé au NOUVEL email (jamais à l'ancien), il prouve que
// son propriétaire en a bien le contrôle avant que le changement ne
// prenne effet. Comme les autres tokens, seul son hash est persisté et
// il est à usage unique.
type TokenChangementEmail struct {
	ID            string
	UtilisateurID string
	NouvelEmail   string
	TokenHash     string
	ExpireAt      time.Time
	UtiliseAt     *time.Time
	CreatedAt     time.Time
}

// NouveauTokenChangementEmail crée un token valide pour la durée
// donnée. tokenHash doit déjà être le hash du code brut envoyé au
// nouvel email.
func NouveauTokenChangementEmail(utilisateurID, nouvelEmail, tokenHash string, duree time.Duration) (*TokenChangementEmail, error) {
	if utilisateurID == "" || nouvelEmail == "" || tokenHash == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if duree <= 0 {
		return nil, commun.ErrDonneesInvalides
	}

	return &TokenChangementEmail{
		ID:            commun.NewID(),
		UtilisateurID: utilisateurID,
		NouvelEmail:   nouvelEmail,
		TokenHash:     tokenHash,
		ExpireAt:      time.Now().UTC().Add(duree),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// EstValide indique si le token peut encore être utilisé : ni déjà
// consommé, ni expiré.
func (t *TokenChangementEmail) EstValide() bool {
	if t.UtiliseAt != nil {
		return false
	}
	return time.Now().UTC().Before(t.ExpireAt)
}

// MarquerUtilise consomme le token (usage unique).
func (t *TokenChangementEmail) MarquerUtilise() {
	maintenant := time.Now().UTC()
	t.UtiliseAt = &maintenant
}
