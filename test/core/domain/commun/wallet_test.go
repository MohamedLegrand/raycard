package commun_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/commun"
)

func TestNouveauWallet(t *testing.T) {
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)

	assert.NotEmpty(t, w.ID)
	assert.Equal(t, int64(0), w.SoldeDisponibleCentimes)
	assert.Equal(t, int64(0), w.SoldeEnAttenteCentimes)
	assert.Equal(t, commun.StatutWalletActif, w.Statut)
}

func TestWallet_Crediter(t *testing.T) {
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)

	require.NoError(t, w.Crediter(50000))
	assert.Equal(t, int64(50000), w.SoldeDisponibleCentimes)

	t.Run("montant invalide", func(t *testing.T) {
		assert.ErrorIs(t, w.Crediter(0), commun.ErrMontantInvalide)
		assert.ErrorIs(t, w.Crediter(-1), commun.ErrMontantInvalide)
	})

	t.Run("plafond dépassé", func(t *testing.T) {
		assert.ErrorIs(t, w.Crediter(200000), commun.ErrPlafondDepasse)
	})

	t.Run("wallet gelé", func(t *testing.T) {
		w.Statut = commun.StatutWalletGele
		assert.ErrorIs(t, w.Crediter(1000), commun.ErrWalletGele)
	})
}

func TestWallet_Debiter(t *testing.T) {
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)
	require.NoError(t, w.Crediter(10000))

	t.Run("solde insuffisant", func(t *testing.T) {
		assert.ErrorIs(t, w.Debiter(20000), commun.ErrSoldeInsuffisant)
	})

	require.NoError(t, w.Debiter(4000))
	assert.Equal(t, int64(6000), w.SoldeDisponibleCentimes)

	t.Run("montant invalide", func(t *testing.T) {
		assert.ErrorIs(t, w.Debiter(0), commun.ErrMontantInvalide)
	})

	t.Run("wallet gelé", func(t *testing.T) {
		w.Statut = commun.StatutWalletGele
		assert.ErrorIs(t, w.Debiter(1000), commun.ErrWalletGele)
	})
}

func TestWallet_CrediterEnAttente(t *testing.T) {
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)

	require.NoError(t, w.CrediterEnAttente(50000))
	assert.Equal(t, int64(50000), w.SoldeEnAttenteCentimes)
	assert.Equal(t, int64(0), w.SoldeDisponibleCentimes)
	assert.Equal(t, int64(50000), w.SoldeTotalCentimes())

	t.Run("plafond dépassé (sur le total, pas seulement en attente)", func(t *testing.T) {
		require.NoError(t, w.Crediter(100000))
		assert.ErrorIs(t, w.CrediterEnAttente(60000), commun.ErrPlafondDepasse)
	})

	t.Run("wallet gelé", func(t *testing.T) {
		w.Statut = commun.StatutWalletGele
		assert.ErrorIs(t, w.CrediterEnAttente(1000), commun.ErrWalletGele)
	})
}

func TestWallet_BasculerEnAttenteVersDisponible(t *testing.T) {
	w, err := commun.NouveauWallet("user-1", "CI", "XOF", 200000)
	require.NoError(t, err)
	require.NoError(t, w.CrediterEnAttente(50000))

	require.NoError(t, w.BasculerEnAttenteVersDisponible(30000))
	assert.Equal(t, int64(20000), w.SoldeEnAttenteCentimes)
	assert.Equal(t, int64(30000), w.SoldeDisponibleCentimes)

	t.Run("montant supérieur au solde en attente", func(t *testing.T) {
		assert.ErrorIs(t, w.BasculerEnAttenteVersDisponible(30000), commun.ErrMontantInvalide)
	})

	t.Run("montant invalide", func(t *testing.T) {
		assert.ErrorIs(t, w.BasculerEnAttenteVersDisponible(0), commun.ErrMontantInvalide)
	})
}
