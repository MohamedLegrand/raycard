// Package carte regroupe les interfaces (ports entrants) exposées par la
// couche application au transport pour les cartes virtuelles.
package carte

import (
	"context"

	"raycard/internal/core/domain/carte"
)

// CreerCarteRequest transporte les données brutes d'une demande
// d'émission de carte depuis le transport vers l'application.
type CreerCarteRequest struct {
	Label           string
	MontantCentimes int64
}

// RechargerCarteRequest transporte les données brutes d'une demande de
// recharge d'une carte existante.
type RechargerCarteRequest struct {
	MontantCentimes int64
}

// SoumettrePorteurCarteRequest transporte les données du KYC porteur de
// carte exigé par l'agrégateur — distinct du KYC Tier 2 de RAYCARD (voir
// carte.CardCustomer). Les pièces d'identité recto/verso ne sont pas
// redemandées ici : le service les récupère du dossier KYC Tier 2 déjà
// approuvé de l'utilisateur.
type SoumettrePorteurCarteRequest struct {
	PaysNomComplet       string
	PaysCodeISO          string
	IndicatifPays        string
	TelephoneLocal       string
	Rue, Ville, Region   string
	CodePostal           string
	NumeroIdentification string
	TypeDocument         string // NIN | PASSPORT | VOTERS_CARD | DRIVERS_LICENSE
	DateNaissance        string // YYYY-MM-DD
}

// CarteUseCase orchestre l'émission et la consultation des cartes
// virtuelles.
type CarteUseCase interface {
	// SoumettrePorteurCarte enrôle l'utilisateur comme porteur de carte
	// auprès de l'agrégateur — préalable obligatoire à CreerCarte (voir
	// carte.CardCustomer). Réservé aux utilisateurs Tier 2
	// (carte.ErrKycTierInsuffisant sinon) ; carte.ErrCardCustomerDejaSoumis
	// si un dossier existe déjà pour cet utilisateur.
	SoumettrePorteurCarte(ctx context.Context, utilisateurID string, req SoumettrePorteurCarteRequest) (*carte.CardCustomer, error)

	// ObtenirStatutPorteurCarte renvoie le dossier de porteur de carte de
	// l'utilisateur (carte.ErrCardCustomerIntrouvable si aucun n'a encore
	// été soumis — un état normal, pas une erreur à afficher tel quel).
	ObtenirStatutPorteurCarte(ctx context.Context, utilisateurID string) (*carte.CardCustomer, error)

	// CreerCarte débite immédiatement le solde disponible du wallet puis
	// déclenche l'émission auprès de l'agrégateur. Réservé aux
	// utilisateurs au palier KYC Tier 2 (carte.ErrKycTierInsuffisant sinon)
	// dont le porteur de carte est enrole (carte.ErrCardCustomerNonEnrole
	// ou carte.ErrCardCustomerRejete sinon — voir SoumettrePorteurCarte).
	CreerCarte(ctx context.Context, utilisateurID string, req CreerCarteRequest) (*carte.Carte, error)

	ListerCartes(ctx context.Context, utilisateurID string) ([]*carte.Carte, error)

	// ObtenirCarte retourne une carte précise. Elle doit appartenir à
	// utilisateurID (carte.ErrCarteIntrouvable sinon, pour ne jamais
	// révéler l'existence de la carte d'un autre utilisateur).
	ObtenirCarte(ctx context.Context, utilisateurID, carteID string) (*carte.Carte, error)

	// GelerCarte bloque une carte active. La carte doit appartenir à
	// utilisateurID (carte.ErrCarteIntrouvable sinon).
	GelerCarte(ctx context.Context, utilisateurID, carteID string) (*carte.Carte, error)

	// DegelerCarte réactive une carte gelée. La carte doit appartenir à
	// utilisateurID (carte.ErrCarteIntrouvable sinon).
	DegelerCarte(ctx context.Context, utilisateurID, carteID string) (*carte.Carte, error)

	// RechargerCarte débite immédiatement le solde disponible du wallet
	// puis ajoute les fonds à une carte existante et active. La carte doit
	// appartenir à utilisateurID (carte.ErrCarteIntrouvable sinon).
	RechargerCarte(ctx context.Context, utilisateurID, carteID string, req RechargerCarteRequest) (*carte.Carte, error)

	// AnnulerCarte détruit définitivement une carte active ou gelée et
	// rembourse au wallet ce qu'il restait dessus. La carte doit
	// appartenir à utilisateurID (carte.ErrCarteIntrouvable sinon).
	AnnulerCarte(ctx context.Context, utilisateurID, carteID string) (*carte.Carte, error)

	// ListerDepenses retourne les dépenses détectées sur une carte donnée
	// par rapprochement de solde (voir SynchroniserSoldes). La carte doit
	// appartenir à utilisateurID (carte.ErrCarteIntrouvable sinon, pour ne
	// jamais révéler l'existence de la carte d'un autre utilisateur).
	ListerDepenses(ctx context.Context, utilisateurID, carteID string) ([]*carte.DepenseCarte, error)

	// SynchroniserSoldes interroge l'agrégateur pour chaque carte active
	// et détecte les dépenses par comparaison de solde. Appelé
	// périodiquement par un job planifié, jamais par une requête
	// utilisateur — faute de webhook de transaction carte côté agrégateur.
	// Retourne le nombre de dépenses détectées.
	SynchroniserSoldes(ctx context.Context) (int, error)
}
