package carte_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/carte"
	"raycard/internal/core/domain/commun"
)

const (
	intervalleBaseTest = 20 * time.Second
	intervalleMaxTest  = 30 * time.Minute
)

func TestNouvelleCarte(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)

	assert.NotEmpty(t, c.ID)
	assert.Equal(t, carte.StatutCarteActive, c.Statut)
	assert.Equal(t, "card-ext-1", c.IDExterne)
	assert.Equal(t, int64(10000), c.SoldeCentimes, "le solde initial correspond au montant chargé")

	t.Run("données invalides", func(t *testing.T) {
		_, err := carte.NouvelleCarte("", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
		assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
	})

	t.Run("id externe manquant", func(t *testing.T) {
		_, err := carte.NouvelleCarte("user-1", "wallet-1", "", "Carte courses", "XOF", 10000)
		assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
	})

	t.Run("montant invalide", func(t *testing.T) {
		_, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 0)
		assert.ErrorIs(t, err, commun.ErrMontantInvalide)
	})
}

func TestCarte_MettreAJourSolde(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	maintenant := time.Now().UTC()

	t.Run("baisse de solde signale une dépense", func(t *testing.T) {
		montantDepense, err := c.MettreAJourSolde(7000, maintenant, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.Equal(t, int64(3000), montantDepense)
		assert.Equal(t, int64(7000), c.SoldeCentimes)
	})

	t.Run("solde stable : aucune dépense", func(t *testing.T) {
		montantDepense, err := c.MettreAJourSolde(7000, maintenant, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.Equal(t, int64(0), montantDepense)
	})

	t.Run("hausse de solde : aucune dépense, solde mis à jour", func(t *testing.T) {
		montantDepense, err := c.MettreAJourSolde(9000, maintenant, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.Equal(t, int64(0), montantDepense)
		assert.Equal(t, int64(9000), c.SoldeCentimes)
	})

	t.Run("solde négatif invalide", func(t *testing.T) {
		_, err := c.MettreAJourSolde(-100, maintenant, intervalleBaseTest, intervalleMaxTest)
		assert.ErrorIs(t, err, commun.ErrMontantInvalide)
	})
}

func TestCarte_MettreAJourSolde_Escalade(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	t0 := time.Now().UTC()

	t.Run("dès la première vérification sans dépense, intervalle de base", func(t *testing.T) {
		_, err := c.MettreAJourSolde(10000, t0, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.True(t, c.ProchaineVerificationAt.Equal(t0.Add(intervalleBaseTest)))
	})

	t.Run("double à chaque passage sans dépense", func(t *testing.T) {
		_, err := c.MettreAJourSolde(10000, t0, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.True(t, c.ProchaineVerificationAt.Equal(t0.Add(2*intervalleBaseTest)))
	})

	t.Run("plafonné à intervalleMax après suffisamment de cycles", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			_, err := c.MettreAJourSolde(10000, t0, intervalleBaseTest, intervalleMaxTest)
			require.NoError(t, err)
		}
		assert.True(t, c.ProchaineVerificationAt.Equal(t0.Add(intervalleMaxTest)))
	})

	t.Run("une dépense détectée redescend immédiatement à l'intervalle de base", func(t *testing.T) {
		_, err := c.MettreAJourSolde(9000, t0, intervalleBaseTest, intervalleMaxTest)
		require.NoError(t, err)
		assert.True(t, c.ProchaineVerificationAt.Equal(t0.Add(intervalleBaseTest)))
		assert.Equal(t, 0, c.NiveauVerification)
	})
}

func TestCarte_SynchroniserStatut(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	maintenant := time.Now().UTC()

	t.Run("statut identique : aucun changement", func(t *testing.T) {
		change := c.SynchroniserStatut(carte.StatutCarteActive, maintenant)
		assert.False(t, change)
	})

	t.Run("statut différent : mis à jour", func(t *testing.T) {
		change := c.SynchroniserStatut(carte.StatutCarteGelee, maintenant)
		assert.True(t, change)
		assert.Equal(t, carte.StatutCarteGelee, c.Statut)
	})
}

func TestCarte_Geler(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	maintenant := time.Now().UTC()

	require.NoError(t, c.Geler(maintenant))
	assert.Equal(t, carte.StatutCarteGelee, c.Statut)

	t.Run("déjà gelée : transition invalide", func(t *testing.T) {
		assert.ErrorIs(t, c.Geler(maintenant), carte.ErrTransitionCarteInvalide)
	})
}

func TestCarte_Degeler(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	t0 := time.Now().UTC()

	t.Run("pas encore gelée : transition invalide", func(t *testing.T) {
		assert.ErrorIs(t, c.Degeler(t0), carte.ErrTransitionCarteInvalide)
	})

	require.NoError(t, c.Geler(t0))

	// Simule une longue période gelée, avec un niveau de vérification qui
	// aurait continué à escalader avant le gel.
	c.NiveauVerification = 5
	tPlusTard := t0.Add(2 * time.Hour)

	require.NoError(t, c.Degeler(tPlusTard))
	assert.Equal(t, carte.StatutCarteActive, c.Statut)
	assert.Equal(t, 0, c.NiveauVerification, "le sondage repart de zéro après un dégel")
	assert.True(t, c.ProchaineVerificationAt.Equal(tPlusTard), "vérifiable immédiatement après le dégel")
}

func TestCarte_Recharger(t *testing.T) {
	c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
	require.NoError(t, err)
	maintenant := time.Now().UTC()

	require.NoError(t, c.Recharger(5000, 15000, maintenant))
	assert.Equal(t, int64(15000), c.MontantChargeCentimes)
	assert.Equal(t, int64(15000), c.SoldeCentimes)

	t.Run("carte gelée : transition invalide", func(t *testing.T) {
		require.NoError(t, c.Geler(maintenant))
		assert.ErrorIs(t, c.Recharger(1000, 16000, maintenant), carte.ErrTransitionCarteInvalide)
	})

	t.Run("montant invalide", func(t *testing.T) {
		c2, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-2", "Carte courses", "XOF", 10000)
		require.NoError(t, err)
		assert.ErrorIs(t, c2.Recharger(0, 10000, maintenant), commun.ErrMontantInvalide)
		assert.ErrorIs(t, c2.Recharger(1000, -1, maintenant), commun.ErrMontantInvalide)
	})
}

func TestCarte_Annuler(t *testing.T) {
	maintenant := time.Now().UTC()

	t.Run("depuis active", func(t *testing.T) {
		c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-1", "Carte courses", "XOF", 10000)
		require.NoError(t, err)
		require.NoError(t, c.Annuler(maintenant))
		assert.Equal(t, carte.StatutCarteAnnulee, c.Statut)
		assert.Equal(t, int64(0), c.SoldeCentimes)
	})

	t.Run("depuis gelée", func(t *testing.T) {
		c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-2", "Carte courses", "XOF", 10000)
		require.NoError(t, err)
		require.NoError(t, c.Geler(maintenant))
		require.NoError(t, c.Annuler(maintenant))
		assert.Equal(t, carte.StatutCarteAnnulee, c.Statut)
	})

	t.Run("déjà annulée : transition invalide", func(t *testing.T) {
		c, err := carte.NouvelleCarte("user-1", "wallet-1", "card-ext-3", "Carte courses", "XOF", 10000)
		require.NoError(t, err)
		require.NoError(t, c.Annuler(maintenant))
		assert.ErrorIs(t, c.Annuler(maintenant), carte.ErrTransitionCarteInvalide)
	})
}

func TestNouvelleDepenseCarte(t *testing.T) {
	d, err := carte.NouvelleDepenseCarte("carte-1", 3000, 10000, 7000)
	require.NoError(t, err)

	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "carte-1", d.CarteID)
	assert.Equal(t, int64(3000), d.MontantCentimes)

	t.Run("carte id manquant", func(t *testing.T) {
		_, err := carte.NouvelleDepenseCarte("", 3000, 10000, 7000)
		assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
	})

	t.Run("montant invalide", func(t *testing.T) {
		_, err := carte.NouvelleDepenseCarte("carte-1", 0, 10000, 10000)
		assert.ErrorIs(t, err, commun.ErrMontantInvalide)
	})
}
