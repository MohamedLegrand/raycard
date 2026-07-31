package auth

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// VerrouConnexion protège un compte contre le bourrage de mot de passe
// (brute force) : après un nombre d'échecs consécutifs, le compte est
// verrouillé temporairement, y compris si le bon mot de passe finit par
// arriver. La durée du verrou double à chaque nouveau cycle d'échecs
// (1 min, 2 min, 4 min...) jusqu'à un plafond, pour rendre une attaque
// soutenue de plus en plus coûteuse en temps sans pénaliser lourdement
// une simple faute de frappe isolée.
type VerrouConnexion struct {
	UtilisateurID      string
	NombreEchecs       int
	NiveauEscalade     int
	DerniereActiviteAt time.Time
	VerrouilleJusqua   *time.Time
}

// NouveauVerrouConnexion crée un verrou vierge (aucun échec encore
// enregistré) pour l'utilisateur donné.
func NouveauVerrouConnexion(utilisateurID string) (*VerrouConnexion, error) {
	if utilisateurID == "" {
		return nil, commun.ErrDonneesInvalides
	}
	return &VerrouConnexion{UtilisateurID: utilisateurID}, nil
}

// EstVerrouille indique si le compte est actuellement bloqué : dans ce
// cas, la connexion doit être refusée avant même de vérifier le mot de
// passe (évite un calcul bcrypt inutile et un éventuel signal de timing).
func (v *VerrouConnexion) EstVerrouille(maintenant time.Time) bool {
	return v.VerrouilleJusqua != nil && maintenant.Before(*v.VerrouilleJusqua)
}

// exposantEscaladeMax borne le calcul de la durée de verrou : au-delà,
// dureeMax prend de toute façon le relais, ça évite juste un
// dépassement arithmétique si un même niveau d'escalade persistait de
// façon anormalement longue.
const exposantEscaladeMax = 10

// EnregistrerEchec comptabilise une tentative de mot de passe
// incorrecte. Si trop d'échecs se sont accumulés depuis le dernier
// verrouillage (seuilEchecs), un nouveau verrou est posé, avec une durée
// qui double à chaque cycle (dureeBase, dureeBase*2, dureeBase*4...,
// plafonnée à dureeMax). Si aucune activité n'a eu lieu depuis
// dureeResetEscalade, l'escalade repart de zéro : un compte qui n'a pas
// été attaqué depuis longtemps ne doit pas hériter d'une pénalité
// ancienne.
//
// Renvoie true si une alerte de sécurité doit être envoyée : jamais dès
// le premier verrouillage (peut être une simple faute de frappe), mais
// dès le second, signe qu'il ne s'agit probablement plus d'une erreur
// isolée.
func (v *VerrouConnexion) EnregistrerEchec(maintenant time.Time, seuilEchecs int, dureeBase, dureeMax, dureeResetEscalade time.Duration) (alerteNecessaire bool) {
	if v.DerniereActiviteAt.IsZero() || maintenant.Sub(v.DerniereActiviteAt) > dureeResetEscalade {
		v.NiveauEscalade = 0
		v.NombreEchecs = 0
	}

	v.NombreEchecs++
	v.DerniereActiviteAt = maintenant

	if v.NombreEchecs < seuilEchecs {
		return false
	}

	v.NiveauEscalade++
	v.NombreEchecs = 0

	exposant := v.NiveauEscalade - 1
	if exposant > exposantEscaladeMax {
		exposant = exposantEscaladeMax
	}
	duree := dureeBase * time.Duration(int64(1)<<uint(exposant))
	if duree <= 0 || duree > dureeMax {
		duree = dureeMax
	}

	verrouilleJusqua := maintenant.Add(duree)
	v.VerrouilleJusqua = &verrouilleJusqua

	return v.NiveauEscalade >= 2
}

// EnregistrerSucces efface l'historique d'échecs : une connexion réussie
// signifie que le véritable propriétaire du compte a repris la main.
func (v *VerrouConnexion) EnregistrerSucces() {
	v.NombreEchecs = 0
	v.NiveauEscalade = 0
	v.VerrouilleJusqua = nil
}
