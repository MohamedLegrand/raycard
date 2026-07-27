package domain

import "time"

// RefreshToken représente un jeton de rafraîchissement opaque : seul son
// hash est persisté (voir output.RefreshTokenRepository), jamais la
// valeur brute envoyée au client. Il est à usage unique : chaque
// rafraîchissement le révoque et en émet un nouveau (rotation).
type RefreshToken struct {
	ID            string
	UtilisateurID string
	TokenHash     string
	ExpireAt      time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
}

// NouveauRefreshToken crée un refresh token valide pour la durée donnée.
// tokenHash doit déjà être le hash du token brut (l'application génère
// et hache la valeur aléatoire, le domaine ne fait que la modéliser).
func NouveauRefreshToken(utilisateurID, tokenHash string, duree time.Duration) (*RefreshToken, error) {
	if utilisateurID == "" || tokenHash == "" {
		return nil, ErrDonneesInvalides
	}
	if duree <= 0 {
		return nil, ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()

	return &RefreshToken{
		ID:            NewID(),
		UtilisateurID: utilisateurID,
		TokenHash:     tokenHash,
		ExpireAt:      maintenant.Add(duree),
		CreatedAt:     maintenant,
	}, nil
}

// EstValide indique si le token peut encore être utilisé pour un
// rafraîchissement : ni révoqué, ni expiré.
func (rt *RefreshToken) EstValide() bool {
	if rt.RevokedAt != nil {
		return false
	}
	return time.Now().UTC().Before(rt.ExpireAt)
}

// Revoquer marque le token comme révoqué (déconnexion ou rotation).
func (rt *RefreshToken) Revoquer() {
	maintenant := time.Now().UTC()
	rt.RevokedAt = &maintenant
}
