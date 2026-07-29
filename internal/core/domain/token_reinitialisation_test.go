package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauTokenReinitialisation(t *testing.T) {
	tr, err := domain.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
	require.NoError(t, err)

	assert.NotEmpty(t, tr.ID)
	assert.Nil(t, tr.UtiliseAt)
	assert.True(t, tr.EstValide())
}

func TestNouveauTokenReinitialisation_DonneesInvalides(t *testing.T) {
	_, err := domain.NouveauTokenReinitialisation("", "hash-abc", time.Hour)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)

	_, err = domain.NouveauTokenReinitialisation("user-1", "hash-abc", 0)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)
}

func TestTokenReinitialisation_EstValide(t *testing.T) {
	t.Run("expiré", func(t *testing.T) {
		tr, err := domain.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		tr.ExpireAt = time.Now().UTC().Add(-time.Minute)
		assert.False(t, tr.EstValide())
	})

	t.Run("déjà utilisé", func(t *testing.T) {
		tr, err := domain.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		tr.MarquerUtilise()
		assert.False(t, tr.EstValide())
		assert.NotNil(t, tr.UtiliseAt)
	})
}
