package domain

import "time"

// StatutDossierKyc représente l'état d'une demande de passage à un
// palier KYC supérieur, en attente de revue par un administrateur.
type StatutDossierKyc string

const (
	StatutDossierEnAttente StatutDossierKyc = "en_attente"
	StatutDossierApprouve  StatutDossierKyc = "approuve"
	StatutDossierRejete    StatutDossierKyc = "rejete"
)

// DossierKyc trace une demande de passage au Tier 2 : contrairement au
// Tier 1 (auto-validé à l'inscription), ce palier exige toujours une
// revue humaine avant que le wallet de l'utilisateur ne bénéficie de
// plafonds plus élevés.
type DossierKyc struct {
	ID            string
	UtilisateurID string
	TierDemande   KycTier
	Statut        StatutDossierKyc
	MotifRejet    string
	AdminID       string // vide tant que le dossier n'a pas été traité
	CreatedAt     time.Time
	TraiteAt      *time.Time
}

// NouveauDossierKyc crée une demande de passage au Tier 2 en attente
// de revue.
func NouveauDossierKyc(utilisateurID string) (*DossierKyc, error) {
	if utilisateurID == "" {
		return nil, ErrDonneesInvalides
	}

	return &DossierKyc{
		ID:            NewID(),
		UtilisateurID: utilisateurID,
		TierDemande:   KycTier2,
		Statut:        StatutDossierEnAttente,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// Approuver marque le dossier comme approuvé par l'administrateur
// donné. N'agit pas sur l'Utilisateur lui-même : c'est à l'appelant
// (application) d'enchaîner avec Utilisateur.PasserAuTier2.
func (d *DossierKyc) Approuver(adminID string) error {
	if d.Statut != StatutDossierEnAttente {
		return ErrTransitionKycInvalide
	}
	if adminID == "" {
		return ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()
	d.Statut = StatutDossierApprouve
	d.AdminID = adminID
	d.TraiteAt = &maintenant
	return nil
}

// Rejeter marque le dossier comme rejeté, avec un motif obligatoire
// (communiqué à l'utilisateur, qui pourra resoumettre une demande).
func (d *DossierKyc) Rejeter(adminID, motif string) error {
	if d.Statut != StatutDossierEnAttente {
		return ErrTransitionKycInvalide
	}
	if adminID == "" || motif == "" {
		return ErrDonneesInvalides
	}

	maintenant := time.Now().UTC()
	d.Statut = StatutDossierRejete
	d.AdminID = adminID
	d.MotifRejet = motif
	d.TraiteAt = &maintenant
	return nil
}
