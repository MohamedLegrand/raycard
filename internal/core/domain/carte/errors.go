package carte

import "errors"

// Erreurs spécifiques aux cartes virtuelles.
var (
	ErrCarteIntrouvable        = errors.New("carte introuvable")
	ErrKycTierInsuffisant      = errors.New("palier KYC insuffisant pour émettre une carte (Tier 2 requis)")
	ErrEmissionEchouee         = errors.New("émission de la carte échouée")
	ErrRechargeEchouee         = errors.New("recharge de la carte échouée")
	ErrAnnulationEchouee       = errors.New("annulation de la carte échouée")
	ErrTransitionCarteInvalide = errors.New("transition de statut de carte invalide")
)
