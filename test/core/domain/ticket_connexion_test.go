package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauTicketConnexion(t *testing.T) {
	tk, err := domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 5, time.Hour)
	require.NoError(t, err)

	assert.NotEmpty(t, tk.ID)
	assert.Equal(t, 5, tk.TentativesRestantes)
	assert.Nil(t, tk.UtiliseAt)
	assert.True(t, tk.EstValide())
}

func TestNouveauTicketConnexion_DonneesInvalides(t *testing.T) {
	_, err := domain.NouveauTicketConnexion("", "hash-ticket", "hash-code", 5, time.Hour)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)

	_, err = domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 0, time.Hour)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)

	_, err = domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 5, 0)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)
}

func TestTicketConnexion_EstValide(t *testing.T) {
	t.Run("expiré", func(t *testing.T) {
		tk, err := domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 5, time.Hour)
		require.NoError(t, err)
		tk.ExpireAt = time.Now().UTC().Add(-time.Minute)
		assert.False(t, tk.EstValide())
	})

	t.Run("déjà utilisé", func(t *testing.T) {
		tk, err := domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 5, time.Hour)
		require.NoError(t, err)
		tk.Consommer()
		assert.False(t, tk.EstValide())
		assert.NotNil(t, tk.UtiliseAt)
	})

	t.Run("tentatives épuisées", func(t *testing.T) {
		tk, err := domain.NouveauTicketConnexion("user-1", "hash-ticket", "hash-code", 2, time.Hour)
		require.NoError(t, err)

		tk.EnregistrerEchec()
		assert.True(t, tk.EstValide(), "encore une tentative restante")

		tk.EnregistrerEchec()
		assert.False(t, tk.EstValide(), "plus aucune tentative")

		// Un échec supplémentaire ne doit pas faire passer le compteur en négatif.
		tk.EnregistrerEchec()
		assert.Equal(t, 0, tk.TentativesRestantes)
	})
}
