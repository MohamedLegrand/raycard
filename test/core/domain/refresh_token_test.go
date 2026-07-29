package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauRefreshToken(t *testing.T) {
	rt, err := domain.NouveauRefreshToken("user-1", "hash-abc", time.Hour)
	require.NoError(t, err)

	assert.NotEmpty(t, rt.ID)
	assert.Nil(t, rt.RevokedAt)
	assert.True(t, rt.EstValide())
}

func TestNouveauRefreshToken_DonneesInvalides(t *testing.T) {
	_, err := domain.NouveauRefreshToken("", "hash-abc", time.Hour)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)

	_, err = domain.NouveauRefreshToken("user-1", "hash-abc", 0)
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)
}

func TestRefreshToken_EstValide(t *testing.T) {
	t.Run("expiré", func(t *testing.T) {
		rt, err := domain.NouveauRefreshToken("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		rt.ExpireAt = time.Now().UTC().Add(-time.Minute)
		assert.False(t, rt.EstValide())
	})

	t.Run("révoqué", func(t *testing.T) {
		rt, err := domain.NouveauRefreshToken("user-1", "hash-abc", time.Hour)
		require.NoError(t, err)
		rt.Revoquer()
		assert.False(t, rt.EstValide())
		assert.NotNil(t, rt.RevokedAt)
	})
}
