package wallet_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/wallet"
)

func TestNouvelleTransactionRecharge(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)

	assert.NotEmpty(t, tx.ID)
	assert.Equal(t, wallet.TypeTransactionRecharge, tx.Type)
	assert.Equal(t, wallet.StatutTransactionEnAttente, tx.Statut)

	t.Run("données invalides", func(t *testing.T) {
		_, err := wallet.NouvelleTransactionRecharge("", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
		assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
	})

	t.Run("opérateur manquant", func(t *testing.T) {
		_, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "", "+2250700000000", 5000)
		assert.ErrorIs(t, err, wallet.ErrOperateurNonSupporte)
	})

	t.Run("montant invalide", func(t *testing.T) {
		_, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 0)
		assert.ErrorIs(t, err, commun.ErrMontantInvalide)
	})
}

func TestNouvelleTransactionRetrait(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRetrait("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)

	assert.NotEmpty(t, tx.ID)
	assert.Equal(t, wallet.TypeTransactionRetrait, tx.Type)
	assert.Equal(t, wallet.StatutTransactionEnAttente, tx.Statut)
}

func TestTransaction_MarquerSucces_Retrait_SansDisponibleLe(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRetrait("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)
	require.NoError(t, tx.MarquerEnvoyee("ref-123"))

	require.NoError(t, tx.MarquerSucces(50, nil))
	assert.Equal(t, wallet.StatutTransactionSucces, tx.Statut)
	assert.Equal(t, int64(50), tx.FraisCentimes)
	assert.Nil(t, tx.DisponibleLe, "un retrait n'a pas de délai de retenue")
}

func TestTransaction_MarquerEnvoyee(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)

	require.NoError(t, tx.MarquerEnvoyee("ref-123"))
	assert.Equal(t, wallet.StatutTransactionEnvoyee, tx.Statut)
	assert.Equal(t, "ref-123", tx.ReferenceExterne)

	t.Run("ne peut pas être appelée deux fois", func(t *testing.T) {
		assert.ErrorIs(t, tx.MarquerEnvoyee("ref-456"), wallet.ErrTransitionTransactionInvalide)
	})

	t.Run("référence vide", func(t *testing.T) {
		tx2, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
		require.NoError(t, err)
		assert.ErrorIs(t, tx2.MarquerEnvoyee(""), commun.ErrDonneesInvalides)
	})
}

func TestTransaction_MarquerSucces(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)

	t.Run("impossible avant MarquerEnvoyee", func(t *testing.T) {
		disponibleLe := time.Now()
		assert.ErrorIs(t, tx.MarquerSucces(75, &disponibleLe), wallet.ErrTransitionTransactionInvalide)
	})

	require.NoError(t, tx.MarquerEnvoyee("ref-123"))
	disponibleLe := time.Now().UTC().Add(48 * time.Hour)
	require.NoError(t, tx.MarquerSucces(75, &disponibleLe))

	assert.Equal(t, wallet.StatutTransactionSucces, tx.Statut)
	assert.Equal(t, int64(75), tx.FraisCentimes)
	assert.Equal(t, int64(4925), tx.MontantNetCentimes())
	require.NotNil(t, tx.DisponibleLe)
	assert.True(t, tx.DisponibleLe.Equal(disponibleLe))
}

func TestTransaction_MarquerEchouee(t *testing.T) {
	tx, err := wallet.NouvelleTransactionRecharge("wallet-1", "user-1", "XOF", "ORANGE", "+2250700000000", 5000)
	require.NoError(t, err)

	t.Run("impossible avant MarquerEnvoyee", func(t *testing.T) {
		assert.ErrorIs(t, tx.MarquerEchouee(), wallet.ErrTransitionTransactionInvalide)
	})

	require.NoError(t, tx.MarquerEnvoyee("ref-123"))
	require.NoError(t, tx.MarquerEchouee())
	assert.Equal(t, wallet.StatutTransactionEchouee, tx.Statut)
}
