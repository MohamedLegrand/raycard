package domain

import "time"

// TokenReinitialisation est un code de réinitialisation de mot de
// passe : comme le refresh token, seul son hash est persisté (voir
// output.TokenReinitialisationRepository). Contrairement au refresh
// token, il n'est jamais renouvelé — juste consommé une fois puis
// définitivement mort.
type TokenReinitialisation struct {
	ID            string
	UtilisateurID string
	TokenHash     string
	ExpireAt      time.Time
	UtiliseAt     *time.Time
	CreatedAt     time.Time
}

// NouveauTokenReinitialisation crée un token valide pour la durée
// donnée. tokenHash doit déjà être le hash du code brut envoyé à
// l'utilisateur.
func NouveauTokenReinitialisation(utilisateurID, tokenHash string, duree time.Duration) (*TokenReinitialisation, error) {
	if utilisateurID == "" || tokenHash == "" {
		return nil, ErrDonneesInvalides
	}
	if duree <= 0 {
		return nil, ErrDonneesInvalides
	}

	return &TokenReinitialisation{
		ID:            NewID(),
		UtilisateurID: utilisateurID,
		TokenHash:     tokenHash,
		ExpireAt:      time.Now().UTC().Add(duree),
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// EstValide indique si le token peut encore être utilisé : ni déjà
// consommé, ni expiré.
func (t *TokenReinitialisation) EstValide() bool {
	if t.UtiliseAt != nil {
		return false
	}
	return time.Now().UTC().Before(t.ExpireAt)
}

// MarquerUtilise consomme le token (usage unique).
func (t *TokenReinitialisation) MarquerUtilise() {
	maintenant := time.Now().UTC()
	t.UtiliseAt = &maintenant
}
