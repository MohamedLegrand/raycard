package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// VerrouReinitialisation protège POST /auth/reinitialiser-mot-de-passe
// contre le bourrage du code à 6 chiffres — même logique d'escalade que
// VerrouConnexion (voir ce fichier pour le détail), mais délibérément
// dupliquée plutôt que réutilisée : la clé n'est pas un UtilisateurID
// mais une AdresseIP (aucun identifiant de compte ne circule dans la
// requête de réinitialisation, seul le code lui-même), ce qui change la
// persistance (pas de FK vers utilisateurs) et le sens métier du champ.
//
// Sans ce verrou, seule la limite HTTP par IP (voir router.limiteurConnexion)
// protégeait cet endpoint : un code à 6 chiffres n'a que 1 000 000 de
// valeurs possibles, cherché par égalité de hash exacte, sans qu'aucune
// tentative individuelle ne soit rattachable à un compte précis pour y
// appliquer un verrou classique.
type VerrouReinitialisation struct {
	AdresseIP          string
	NombreEchecs       int
	NiveauEscalade     int
	DerniereActiviteAt time.Time
	VerrouilleJusqua   *time.Time
}

// NouveauVerrouReinitialisation crée un verrou vierge pour l'adresse IP
// donnée.
func NouveauVerrouReinitialisation(adresseIP string) (*VerrouReinitialisation, error) {
	if adresseIP == "" {
		return nil, commun.ErrDonneesInvalides
	}
	return &VerrouReinitialisation{AdresseIP: adresseIP}, nil
}

// EstVerrouille indique si cette IP est actuellement bloquée.
func (v *VerrouReinitialisation) EstVerrouille(maintenant time.Time) bool {
	return v.VerrouilleJusqua != nil && maintenant.Before(*v.VerrouilleJusqua)
}

// exposantEscaladeReinitialisationMax : même rôle que
// exposantEscaladeMax pour VerrouConnexion (borne le calcul, évite un
// dépassement arithmétique).
const exposantEscaladeReinitialisationMax = 10

// EnregistrerEchec comptabilise une soumission de code invalide. Voir
// VerrouConnexion.EnregistrerEchec pour la logique d'escalade complète
// (identique ici, dupliquée intentionnellement).
func (v *VerrouReinitialisation) EnregistrerEchec(maintenant time.Time, seuilEchecs int, dureeBase, dureeMax, dureeResetEscalade time.Duration) {
	if v.DerniereActiviteAt.IsZero() || maintenant.Sub(v.DerniereActiviteAt) > dureeResetEscalade {
		v.NiveauEscalade = 0
		v.NombreEchecs = 0
	}

	v.NombreEchecs++
	v.DerniereActiviteAt = maintenant

	if v.NombreEchecs < seuilEchecs {
		return
	}

	v.NiveauEscalade++
	v.NombreEchecs = 0

	exposant := v.NiveauEscalade - 1
	if exposant > exposantEscaladeReinitialisationMax {
		exposant = exposantEscaladeReinitialisationMax
	}
	duree := dureeBase * time.Duration(int64(1)<<uint(exposant))
	if duree <= 0 || duree > dureeMax {
		duree = dureeMax
	}

	verrouilleJusqua := maintenant.Add(duree)
	v.VerrouilleJusqua = &verrouilleJusqua
}

// EnregistrerSucces efface l'historique d'échecs — une réinitialisation
// réussie depuis cette IP signifie qu'elle n'a plus besoin d'être
// pénalisée pour les échecs précédents.
func (v *VerrouReinitialisation) EnregistrerSucces() {
	v.NombreEchecs = 0
	v.NiveauEscalade = 0
	v.VerrouilleJusqua = nil
}
