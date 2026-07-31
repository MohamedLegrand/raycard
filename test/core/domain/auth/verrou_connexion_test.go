package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
)

const (
	seuilTest              = 3
	dureeBaseTest          = time.Minute
	dureeMaxTest           = 30 * time.Minute
	dureeResetEscaladeTest = time.Hour
)

func TestNouveauVerrouConnexion_DonneesInvalides(t *testing.T) {
	_, err := auth.NouveauVerrouConnexion("")
	assert.ErrorIs(t, err, commun.ErrDonneesInvalides)
}

func TestVerrouConnexion_EstVerrouille(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	assert.False(t, v.EstVerrouille(maintenant), "aucun verrou posé au départ")
}

func TestVerrouConnexion_EnregistrerEchec_PasEncoreAuSeuil(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	alerte := v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	assert.False(t, alerte)
	assert.False(t, v.EstVerrouille(maintenant), "moins de 3 échecs : pas encore verrouillé")
}

func TestVerrouConnexion_EnregistrerEchec_PremierVerrouillage(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	var alerte bool
	for i := 0; i < seuilTest; i++ {
		alerte = v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}

	assert.False(t, alerte, "pas d'alerte dès le premier verrouillage")
	assert.True(t, v.EstVerrouille(maintenant))
	assert.False(t, v.EstVerrouille(maintenant.Add(dureeBaseTest+time.Second)), "le verrou expire après dureeBase")
}

func TestVerrouConnexion_EnregistrerEchec_EscaladeDouble(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	t0 := time.Now().UTC()

	// Premier cycle : verrouillé pour dureeBase (1 min).
	var maintenant time.Time
	for i := 0; i < seuilTest; i++ {
		maintenant = t0
		v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.True(t, v.EstVerrouille(t0.Add(dureeBaseTest-time.Second)))
	assert.False(t, v.EstVerrouille(t0.Add(dureeBaseTest+time.Second)))

	// Deuxième cycle, une fois le premier verrou expiré : doit durer le
	// double (2 min), pas encore expiré après seulement 1 min.
	t1 := t0.Add(dureeBaseTest + time.Second)
	var alerte bool
	for i := 0; i < seuilTest; i++ {
		alerte = v.EnregistrerEchec(t1, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.True(t, alerte, "dès le second verrouillage, une alerte doit être déclenchée")
	assert.True(t, v.EstVerrouille(t1.Add(dureeBaseTest)), "toujours verrouillé après seulement 1 min (durée doublée à 2 min)")
	assert.False(t, v.EstVerrouille(t1.Add(2*dureeBaseTest+time.Second)), "expiré après les 2 min complètes")
}

func TestVerrouConnexion_EnregistrerEchec_PlafondDuree(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	// Force artificiellement un niveau d'escalade élevé en enchaînant de
	// nombreux cycles à la suite (sans jamais atteindre dureeResetEscalade).
	for cycle := 0; cycle < 10; cycle++ {
		for i := 0; i < seuilTest; i++ {
			v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
		}
		maintenant = maintenant.Add(time.Second) // reste bien avant dureeResetEscaladeTest
	}
	dernierMaintenant := maintenant.Add(-time.Second) // celui effectivement utilisé au dernier cycle

	// La durée du dernier verrou ne doit jamais dépasser dureeMaxTest.
	assert.True(t, v.EstVerrouille(dernierMaintenant.Add(dureeMaxTest-time.Second)))
	assert.False(t, v.EstVerrouille(dernierMaintenant.Add(dureeMaxTest+time.Second)))
}

func TestVerrouConnexion_EnregistrerEchec_ResetApresInactivite(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	t0 := time.Now().UTC()
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(t0, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	// Deuxième cycle immédiat : escalade au niveau 2.
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(t0, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}

	// Longtemps après (au-delà de dureeResetEscaladeTest) : l'escalade
	// repart de zéro, le prochain verrouillage ne doit PAS déclencher
	// d'alerte (comme un tout premier verrouillage).
	tLoin := t0.Add(dureeResetEscaladeTest + time.Minute)
	var alerte bool
	for i := 0; i < seuilTest; i++ {
		alerte = v.EnregistrerEchec(tLoin, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.False(t, alerte, "l'escalade doit être repartie de zéro après une longue inactivité")
	assert.False(t, v.EstVerrouille(tLoin.Add(dureeBaseTest+time.Second)), "verrou de durée de base, pas doublée")
}

func TestVerrouConnexion_EnregistrerSucces(t *testing.T) {
	v, err := auth.NouveauVerrouConnexion("user-1")
	require.NoError(t, err)

	maintenant := time.Now().UTC()
	for i := 0; i < seuilTest; i++ {
		v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	require.True(t, v.EstVerrouille(maintenant))

	v.EnregistrerSucces()
	assert.False(t, v.EstVerrouille(maintenant), "une connexion réussie efface le verrou")

	// L'escalade repart aussi de zéro : le prochain verrouillage ne doit
	// pas être immédiatement doublé.
	var alerte bool
	for i := 0; i < seuilTest; i++ {
		alerte = v.EnregistrerEchec(maintenant, seuilTest, dureeBaseTest, dureeMaxTest, dureeResetEscaladeTest)
	}
	assert.False(t, alerte)
}
