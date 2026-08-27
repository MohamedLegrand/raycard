package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
)

// Même logique d'escalade que VerrouConnexion (voir verrou_connexion_test.go),
// testée ici séparément parce que VerrouReinitialisation est un type
// distinct (clé IP, pas utilisateur) — voir le commentaire du fichier
// domain/auth/verrou_reinitialisation.go sur pourquoi la duplication est
// volontaire.
func TestNouveauVerrouReinitialisation_DonneesInvalides(t *testing.T) {
	_, err := auth.NouveauVerrouReinitialisation("")
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
}

func TestVerrouReinitialisation_EstVerrouille(t *testing.T) {
	v, err := auth.NouveauVerrouReinitialisation("1.2.3.4")
	require.NoError(t, err)

	assert.False(t, v.EstVerrouille(time.Now().UTC()), "aucun verrou posé au départ")
}

func TestVerrouReinitialisation_EnregistrerEchec_PremierVerrouillage(t *testing.T) {
	v, err := auth.NouveauVerrouReinitialisation("1.2.3.4")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}

	assert.True(t, v.EstVerrouille(maintenant))
	assert.False(t, v.EstVerrouille(maintenant.Add(dureeBaseTest+time.Second)), "le verrou expire après dureeBase")
}

func TestVerrouReinitialisation_EnregistrerEchec_EscaladeDouble(t *testing.T) {
	v, err := auth.NouveauVerrouReinitialisation("1.2.3.4")
	require.NoError(t, err)

	t0 := time.Now().UTC()
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(t0, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.False(t, v.EstVerrouille(t0.Add(dureeBaseTest+time.Second)))

	t1 := t0.Add(dureeBaseTest + time.Second)
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(t1, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.True(t, v.EstVerrouille(t1.Add(dureeBaseTest)), "toujours verrouillé après seulement 1 min (durée doublée à 2 min)")
	assert.False(t, v.EstVerrouille(t1.Add(2*dureeBaseTest+time.Second)))
}

func TestVerrouReinitialisation_EnregistrerSucces(t *testing.T) {
	v, err := auth.NouveauVerrouReinitialisation("1.2.3.4")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	require.True(t, v.EstVerrouille(maintenant))

	v.EnregistrerSucces()
	assert.False(t, v.EstVerrouille(maintenant), "une réinitialisation réussie efface le verrou")
}
