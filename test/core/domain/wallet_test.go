package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauWallet(t *testing.T) {
	w, err := domain.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)

	assert.NotEmpty(t, w.ID)
	assert.Equal(t, int64(0), w.SoldeCentimes)
	assert.Equal(t, domain.StatutWalletActif, w.Statut)
}

func TestWallet_Crediter(t *testing.T) {
	w, err := domain.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)

	require.NoError(t, w.Crediter(50000))
	assert.Equal(t, int64(50000), w.SoldeCentimes)

	t.Run("montant invalide", func(t *testing.T) {
		assert.ErrorIs(t, w.Crediter(0), domain.ErrMontantInvalide)
		assert.ErrorIs(t, w.Crediter(-1), domain.ErrMontantInvalide)
	})

	t.Run("plafond dépassé", func(t *testing.T) {
		assert.ErrorIs(t, w.Crediter(200000), domain.ErrPlafondDepasse)
	})

	t.Run("wallet gelé", func(t *testing.T) {
		w.Statut = domain.StatutWalletGele
		assert.ErrorIs(t, w.Crediter(1000), domain.ErrWalletGele)
	})
}

func TestWallet_Debiter(t *testing.T) {
	w, err := domain.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)
	require.NoError(t, w.Crediter(10000))

	t.Run("solde insuffisant", func(t *testing.T) {
		assert.ErrorIs(t, w.Debiter(20000), domain.ErrSoldeInsuffisant)
	})

	require.NoError(t, w.Debiter(4000))
	assert.Equal(t, int64(6000), w.SoldeCentimes)

	t.Run("montant invalide", func(t *testing.T) {
		assert.ErrorIs(t, w.Debiter(0), domain.ErrMontantInvalide)
	})

	t.Run("wallet gelé", func(t *testing.T) {
		w.Statut = domain.StatutWalletGele
		assert.ErrorIs(t, w.Debiter(1000), domain.ErrWalletGele)
	})
}
