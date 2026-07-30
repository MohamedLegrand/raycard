package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
)

func TestNouveauTokenReinitialisation(t *testing.T) {
	tr, err := auth.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
	require.NoError(t, err)

	assert.NotEmpty(t, tr.ID)
	assert.Nil(t, tr.UtiliseAt)
	assert.True(t, tr.EstValide())
}

func TestNouveauTokenReinitialisation_DonneesInvalides(t *testing.T) {
	_, err := auth.NouveauTokenReinitialisation("", "hash-abc", time.Hour)
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)

	_, err = auth.NouveauTokenReinitialisation("user-1", "hash-abc", 0)
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
}

func TestTokenReinitialisation_EstValide(t *testing.T) {
	t.Run("expiré", func(t *testing.T) {
		tr, err := auth.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		tr.ExpireAt = time.Now().UTC().Add(-time.Minute)
		assert.False(t, tr.EstValide())
	})

	t.Run("déjà utilisé", func(t *testing.T) {
		tr, err := auth.NouveauTokenReinitialisation("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		tr.MarquerUtilise()
		assert.False(t, tr.EstValide())
		assert.NotNil(t, tr.UtiliseAt)
	})
}
