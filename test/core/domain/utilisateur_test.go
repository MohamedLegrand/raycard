package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauUtilisateur_Valide(t *testing.T) {
	u, err := domain.NouveauUtilisateur("Koné", "Awa", "Awa.Kone@Example.com", "+2250700000000", "ci", "hash")
	require.NoError(t, err)

	assert.NotEmpty(t, u.ID)
	assert.Equal(t, "awa.kone@example.com", u.Email, "l'email doit être normalisé en minuscules")
	assert.Equal(t, "CI", u.PaysCode, "le code pays doit être normalisé en majuscules")
	assert.Equal(t, domain.KycTierAucun, u.KycTier)
	assert.Equal(t, domain.KycStatutEnAttente, u.KycStatut)
	assert.Equal(t, domain.RoleClient, u.Role)
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
			_, err := domain.NouveauUtilisateur(c.nom, c.prenom, c.email, c.telephone, c.paysCode, c.hash)
			assert.ErrorIs(t, err, domain.ErrDonneesInvalides)
		})
	}
}

func TestUtilisateur_ValiderKycTier1(t *testing.T) {
	u, err := domain.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)

	require.NoError(t, u.ValiderKycTier1())
	assert.Equal(t, domain.KycTier1, u.KycTier)
	assert.Equal(t, domain.KycStatutVerifie, u.KycStatut)

	// Une deuxième validation doit échouer : la transition n'est valide
	// que depuis le statut "en_attente".
	err = u.ValiderKycTier1()
	assert.ErrorIs(t, err, domain.ErrTransitionKycInvalide)
}

func TestUtilisateur_PasserAuTier2(t *testing.T) {
	u, err := domain.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)

	t.Run("depuis tier aucun : refusé", func(t *testing.T) {
		assert.ErrorIs(t, u.PasserAuTier2(), domain.ErrTransitionKycInvalide)
	})

	require.NoError(t, u.ValiderKycTier1())
	require.NoError(t, u.PasserAuTier2())
	assert.Equal(t, domain.KycTier2, u.KycTier)

	t.Run("depuis tier2 : refusé", func(t *testing.T) {
		assert.ErrorIs(t, u.PasserAuTier2(), domain.ErrTransitionKycInvalide)
	})
}

func TestNouvelAdministrateur(t *testing.T) {
	admin, err := domain.NouvelAdministrateur("Zoa", "Stéphane", "admin@example.com", "+2250700000001", "CI", "hash")
	require.NoError(t, err)

	assert.Equal(t, domain.RoleAdmin, admin.Role)
	assert.True(t, admin.EstAdmin())
	assert.Equal(t, domain.KycStatutVerifie, admin.KycStatut)
}
