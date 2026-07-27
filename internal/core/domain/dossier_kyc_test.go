package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain"
)

func TestNouveauDossierKyc(t *testing.T) {
	d, err := domain.NouveauDossierKyc("user-1")
	require.NoError(t, err)

	assert.NotEmpty(t, d.ID)
	assert.Equal(t, domain.KycTier2, d.TierDemande)
	assert.Equal(t, domain.StatutDossierEnAttente, d.Statut)
	assert.Nil(t, d.TraiteAt)
}

func TestNouveauDossierKyc_DonneesInvalides(t *testing.T) {
	_, err := domain.NouveauDossierKyc("")
	assert.ErrorIs(t, err, domain.ErrDonneesInvalides)
}

func TestDossierKyc_Approuver(t *testing.T) {
	d, err := domain.NouveauDossierKyc("user-1")
	require.NoError(t, err)

	require.NoError(t, d.Approuver("admin-1"))
	assert.Equal(t, domain.StatutDossierApprouve, d.Statut)
	assert.Equal(t, "admin-1", d.AdminID)
	assert.NotNil(t, d.TraiteAt)

	// Un dossier déjà traité ne peut plus être approuvé.
	assert.ErrorIs(t, d.Approuver("admin-1"), domain.ErrTransitionKycInvalide)
}

func TestDossierKyc_Rejeter(t *testing.T) {
	d, err := domain.NouveauDossierKyc("user-1")
	require.NoError(t, err)

	require.NoError(t, d.Rejeter("admin-1", "pièce d'identité illisible"))
	assert.Equal(t, domain.StatutDossierRejete, d.Statut)
	assert.Equal(t, "pièce d'identité illisible", d.MotifRejet)

	t.Run("motif obligatoire", func(t *testing.T) {
		d2, err := domain.NouveauDossierKyc("user-2")
		require.NoError(t, err)
		assert.ErrorIs(t, d2.Rejeter("admin-1", ""), domain.ErrDonneesInvalides)
	})
}
