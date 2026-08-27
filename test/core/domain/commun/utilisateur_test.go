package commun_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/commun"
)

func TestNouveauUtilisateur_Valide(t *testing.T) {
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "Awa.Kone@Example.com", "+2250700000000", "ci", "hash")
	require.NoError(t, err)

	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "awa.kone@example.com", u.Email, "l'email doit être normalisé en minuscules")
	assert.Equal(t, "CI", u.PaysCode, "le code pays doit être normalisé en majuscules")
	assert.Equal(t, commun.KycTierAucun, u.KycTier)
	assert.Equal(t, commun.KycStatutEnAttente, u.KycStatut)
	assert.Equal(t, commun.RoleClient, u.Role)
	assert.False(t, u.EstAdmin())
}

func TestNouveauUtilisateur_DonneesInvalides(t *testing.T) {
	cas := map[string]struct {
		nom, prenom, email, telephone, paysCode, hash string
	}{
		"nom vide":               {"", "Awa", "awa@example.com", "+2250700000000", "CI", "hash"},
		"email invalide":         {"Koné", "Awa", "pas-un-email", "+2250700000000", "CI", "hash"},
		"pays code trop court":   {"Koné", "Awa", "awa@example.com", "+2250700000000", "C", "hash"},
		"mot de passe hash vide": {"Koné", "Awa", "awa@example.com", "+2250700000000", "CI", ""},
	}

	for nom, c := range cas {
		t.Run(nom, func(t *testing.T) {
			_, err := commun.NouveauUtilisateur(c.nom, c.prenom, c.email, c.telephone, c.paysCode, c.hash)
			assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
		})
	}
}

func TestUtilisateur_ValiderKycTier1(t *testing.T) {
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)

	require.NoError(t, u.ValiderKycTier1())
	assert.Equal(t, commun.KycTier1, u.KycTier)
	assert.Equal(t, commun.KycStatutVerifie, u.KycStatut)

	// Une deuxième validation doit échouer : la transition n'est valide
	// que depuis le statut "en_attente".
	err = u.ValiderKycTier1()
	assert.ErrorIs(t, err, commun.ErrTransitionKycInvalide)
}

func TestUtilisateur_PasserAuTier2(t *testing.T) {
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)

	t.Run("depuis tier aucun : refusé", func(t *testing.T) {
		assert.ErrorIs(t, u.PasserAuTier2(), commun.ErrTransitionKycInvalide)
	})

	require.NoError(t, u.ValiderKycTier1())
	require.NoError(t, u.PasserAuTier2())
	assert.Equal(t, commun.KycTier2, u.KycTier)

	t.Run("depuis tier2 : refusé", func(t *testing.T) {
		assert.ErrorIs(t, u.PasserAuTier2(), commun.ErrTransitionKycInvalide)
	})
}

func TestNouvelAdministrateur(t *testing.T) {
	admin, err := commun.NouvelAdministrateur("Zoa", "Stéphane", "admin@example.com", "+2250700000001", "CI", "hash", commun.RoleAdmin)
	require.NoError(t, err)

	assert.Equal(t, commun.RoleAdmin, admin.Role)
	assert.True(t, admin.EstAdmin())
	assert.Equal(t, commun.KycStatutVerifie, admin.KycStatut)
}

func TestNouvelUtilisateurGoogle(t *testing.T) {
	u, err := commun.NouvelUtilisateurGoogle("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "google-sub-123")
	require.NoError(t, err)

	assert.Equal(t, "google-sub-123", u.GoogleID)
	assert.Empty(t, u.MotDePasseHash, "un compte Google pur n'a pas de mot de passe")
	assert.Equal(t, commun.RoleClient, u.Role)
}

func TestNouvelUtilisateurGoogle_DonneesInvalides(t *testing.T) {
	_, err := commun.NouvelUtilisateurGoogle("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "")
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides, "google id vide refusé")

	_, err = commun.NouvelUtilisateurGoogle("", "Awa", "awa@example.com", "+2250700000000", "CI", "google-sub-123")
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides, "les autres champs restent validés normalement")
}

func TestUtilisateur_LierGoogleID(t *testing.T) {
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.Empty(t, u.GoogleID)

	require.NoError(t, u.LierGoogleID("google-sub-456"))
	assert.Equal(t, "google-sub-456", u.GoogleID)

	assert.ErrorIs(t, u.LierGoogleID(""), commun.ErrDonneesInvalides)
}
