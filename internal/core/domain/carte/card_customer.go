package carte

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// StatutCardCustomer représente l'état du KYC porteur de carte exigé par
// l'agrégateur (Cartevo, via HR-Skills Pay) — distinct du KYC Tier 2 de
// RAYCARD lui-même : un utilisateur peut être Tier 2 chez RAYCARD tout en
// n'ayant pas encore de porteur de carte, ou en attente de sa revue par
// l'agrégateur.
type StatutCardCustomer string

const (
	// StatutCardCustomerEnAttente : dossier soumis à l'agrégateur, en
	// attente de revue par un administrateur HR-Skills Pay — hors du
	// contrôle de RAYCARD, délai variable (voir leur documentation).
	StatutCardCustomerEnAttente StatutCardCustomer = "en_attente_revue"
	// StatutCardCustomerEnrolement : dossier approuvé côté HR-Skills, mais
	// l'enrôlement chez Cartevo lui-même a dû être retenté (échec réseau
	// ou 5xx transitoire côté agrégateur) — état transitoire, jamais final.
	StatutCardCustomerEnrolement StatutCardCustomer = "enrolement_en_cours"
	// StatutCardCustomerEnrole : le porteur peut désormais recevoir des
	// cartes (seul statut qui débloque CarteUseCase.CreerCarte).
	StatutCardCustomerEnrole StatutCardCustomer = "enrole"
	// StatutCardCustomerRejeteLocal : rejeté directement par un
	// administrateur HR-Skills, sans même atteindre Cartevo.
	StatutCardCustomerRejeteLocal StatutCardCustomer = "rejete_local"
	// StatutCardCustomerRejeteFournisseur : approuvé côté HR-Skills mais
	// refusé définitivement par Cartevo (ex: document illisible).
	StatutCardCustomerRejeteFournisseur StatutCardCustomer = "rejete_fournisseur"
)

// EstRejete regroupe les deux statuts de rejet — le motif de refus est
// pertinent à afficher dans les deux cas, la distinction local/fournisseur
// n'intéresse que le diagnostic interne.
func (s StatutCardCustomer) EstRejete() bool {
	return s == StatutCardCustomerRejeteLocal || s == StatutCardCustomerRejeteFournisseur
}

// CardCustomer trace le dossier KYC porteur de carte d'un utilisateur
// RAYCARD auprès de l'agrégateur — un par utilisateur (voir
// NouveauCardCustomer). IDExterne reste vide tant que l'enrôlement chez
// Cartevo n'a pas abouti.
type CardCustomer struct {
	ID            string
	UtilisateurID string
	IDExterne     string
	Statut        StatutCardCustomer
	MotifRejet    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NouveauCardCustomer trace une soumission de porteur de carte, aussitôt
// envoyée à l'agrégateur par l'appelant (voir
// AgregateurCarte.SoumettreCardCustomer) — toujours créé en_attente_revue,
// jamais directement enrole : même un enrôlement synchrone réussi côté
// Cartevo passe par ce statut initial, mis à jour ensuite par l'appelant.
func NouveauCardCustomer(utilisateurID string) (*CardCustomer, error) {
	if utilisateurID == "" {
		return nil, commun.ErrDonneesInvalides
	}
	maintenant := time.Now().UTC()
	return &CardCustomer{
		ID:            commun.NewID(),
		UtilisateurID: utilisateurID,
		Statut:        StatutCardCustomerEnAttente,
		CreatedAt:     maintenant,
		UpdatedAt:     maintenant,
	}, nil
}

// MarquerEnrole enregistre un enrôlement réussi chez l'agrégateur.
func (c *CardCustomer) MarquerEnrole(idExterne string, maintenant time.Time) error {
	if idExterne == "" {
		return commun.ErrDonneesInvalides
	}
	c.IDExterne = idExterne
	c.Statut = StatutCardCustomerEnrole
	c.MotifRejet = ""
	c.UpdatedAt = maintenant
	return nil
}

// MarquerEnrolementEnCours signale un enrôlement Cartevo à retenter (échec
// réseau/5xx ambigu, jamais un rejet) — le dossier reste soumis, aucune
// action de l'utilisateur n'est requise.
func (c *CardCustomer) MarquerEnrolementEnCours(maintenant time.Time) {
	c.Statut = StatutCardCustomerEnrolement
	c.UpdatedAt = maintenant
}

// MarquerRejete enregistre un refus — rejeteFournisseur distingue un refus
// de Cartevo (après approbation HR-Skills) d'un refus direct côté
// HR-Skills, pour le diagnostic interne uniquement (voir StatutCardCustomer.EstRejete).
func (c *CardCustomer) MarquerRejete(motif string, rejeteFournisseur bool, maintenant time.Time) {
	if rejeteFournisseur {
		c.Statut = StatutCardCustomerRejeteFournisseur
	} else {
		c.Statut = StatutCardCustomerRejeteLocal
	}
	c.MotifRejet = motif
	c.UpdatedAt = maintenant
}
