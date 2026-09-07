// Package carte regroupe les ports sortants du module carte :
// persistance des cartes et accès à l'agrégateur de paiement externe pour
// l'émission. Les implémentations concrètes vivent dans
// internal/infrastructure.
package carte

import (
	"context"
	"time"

	"raycard/internal/core/domain/carte"
)

// CarteRepository persiste les cartes. Une absence de résultat doit être
// signalée par carte.ErrCarteIntrouvable.
type CarteRepository interface {
	Create(ctx context.Context, c *carte.Carte) error
	FindByID(ctx context.Context, id string) (*carte.Carte, error)
	ListByUtilisateurID(ctx context.Context, utilisateurID string) ([]*carte.Carte, error)
	Update(ctx context.Context, c *carte.Carte) error

	// ListAVerifier liste les cartes actives dont ProchaineVerificationAt
	// est dépassé, tous utilisateurs confondus — utilisé par le job
	// planifié de synchronisation des soldes (voir
	// carte.CarteUseCase.SynchroniserSoldes). Ne renvoie jamais une carte
	// dont l'intervalle de vérification n'est pas encore écoulé : c'est ce
	// qui rend le sondage adaptatif plutôt que systématique.
	ListAVerifier(ctx context.Context, avant time.Time) ([]*carte.Carte, error)

	// ListToutes liste les cartes tous utilisateurs confondus, pour le
	// back-office (voir ListByUtilisateurID pour l'équivalent client).
	ListToutes(ctx context.Context, filtre FiltreCartes) ([]*carte.Carte, error)
}

// FiltreCartes restreint le listing back-office des cartes ; un champ
// vide ne filtre pas dessus.
type FiltreCartes struct {
	UtilisateurID string
	Statut        string
}

// DepenseCarteRepository persiste les dépenses détectées par
// rapprochement de solde.
type DepenseCarteRepository interface {
	Create(ctx context.Context, d *carte.DepenseCarte) error
	ListByCarteID(ctx context.Context, carteID string) ([]*carte.DepenseCarte, error)
}

// CardCustomerRepository persiste les dossiers de porteur de carte — un
// par utilisateur (voir carte.NouveauCardCustomer). Une absence de
// résultat doit être signalée par carte.ErrCardCustomerIntrouvable.
type CardCustomerRepository interface {
	Create(ctx context.Context, c *carte.CardCustomer) error
	FindByUtilisateurID(ctx context.Context, utilisateurID string) (*carte.CardCustomer, error)
	Update(ctx context.Context, c *carte.CardCustomer) error
}

// CreerCarteParams décrit une demande d'émission de carte virtuelle.
// MontantUSDCentimes est le financement initial de la carte, en centimes
// de dollar (voir le commentaire sur carte.Carte.Devise : les cartes sont
// systématiquement émises en USD par l'agrégateur, quelle que soit la
// devise du wallet RAYCARD qui a financé l'opération).
type CreerCarteParams struct {
	CustomerIDExterne  string
	Brand              string // "VISA" ou "MASTERCARD"
	NomSurCarte        string
	Label              string
	MontantUSDCentimes int64
}

// CreerCarteResultat est renvoyé une fois la carte effectivement émise
// par l'agrégateur. StatutProvisoire est vrai en cas de réponse ambiguë
// (202 — échec réseau/5xx côté Cartevo, jamais de double-tentative
// aveugle) : la carte existe peut-être déjà, la confirmation arrivera par
// webhook ou réconciliation ultérieure — voir carte.StatutCarteActive et
// la gestion de ce cas côté service applicatif.
type CreerCarteResultat struct {
	IDExterne         string
	SoldeUSDCentimes  int64
	StatutProvisoire  bool
}

// SoumettreCardCustomerParams porte l'identité du porteur de carte à
// soumettre à l'agrégateur — un KYC distinct du KYC Tier 2 de RAYCARD
// (voir carte.CardCustomer). Les documents sont fournis en octets bruts :
// c'est l'implémentation qui les encode en data-URI base64, jamais le
// domaine.
type SoumettreCardCustomerParams struct {
	Prenom                string
	Nom                   string
	Email                 string
	PaysNomComplet        string // ex: "Cameroon" — nom complet attendu par l'agrégateur, pas le code ISO
	PaysCodeISO           string // ex: "CM"
	IndicatifPays         string // ex: "+237"
	TelephoneLocal        string // sans indicatif
	Rue, Ville, Region    string
	CodePostal            string
	NumeroIdentification  string
	TypeDocument          string // NIN | PASSPORT | VOTERS_CARD | DRIVERS_LICENSE
	DateNaissance         string // YYYY-MM-DD
	DocumentRecto         []byte
	DocumentRectoMimeType string
	DocumentVerso         []byte
	DocumentVersoMimeType string
}

// SoumettreCardCustomerResultat reflète l'état retourné par l'agrégateur
// juste après soumission — jamais "enrole" à ce stade (voir
// carte.NouveauCardCustomer) : la revue est humaine, toujours différée.
type SoumettreCardCustomerResultat struct {
	IDExterne string
}

// AgregateurCarte isole les appels cartes auprès de l'agrégateur de
// paiement externe (aujourd'hui HR-Skills Pay/Cartevo) derrière une
// interface propre au domaine.
//
// Important : comme wallet.AgregateurPaiement, ce port ne permet pas de
// contrôler une clé d'idempotence — la protection contre un double appel
// se fait via la Transaction locale (voir
// wallet.Transaction.MarquerEnvoyee) et, pour les cartes, via l'état du
// carte.CardCustomer avant tout nouvel appel.
type AgregateurCarte interface {
	// SoumettreCardCustomer enrôle un porteur pour financement futur de
	// cartes — préalable obligatoire à CreerCarte (voir carte.CardCustomer).
	SoumettreCardCustomer(ctx context.Context, params SoumettreCardCustomerParams) (*SoumettreCardCustomerResultat, error)

	// ObtenirCardWallet renvoie le solde actuel du portefeuille USD dédié
	// aux cartes — distinct du wallet XAF principal du marchand (voir
	// wallet.AgregateurPaiement), débité par toute opération carte
	// (forfait d'émission, recharge, retrait).
	ObtenirCardWallet(ctx context.Context) (soldeUSDCentimes int64, err error)

	// CoterConversion prévisualise, sans effectuer l'opération, le montant
	// USD résultant de la conversion d'un montant XAF — la marge de
	// change appliquée n'est jamais recalculée localement (voir la
	// documentation de l'agrégateur : le taux vient toujours du serveur).
	CoterConversion(ctx context.Context, montantXAFCentimes int64) (montantUSDCentimes int64, err error)

	// AlimenterCardWallet convertit des fonds du wallet XAF principal du
	// marchand vers le portefeuille USD cartes et renvoie le nouveau
	// solde USD.
	AlimenterCardWallet(ctx context.Context, montantUSDCentimes int64) (nouveauSoldeUSDCentimes int64, err error)

	CreerCarte(ctx context.Context, params CreerCarteParams) (*CreerCarteResultat, error)

	// ObtenirEtatCarte interroge le solde et le statut actuels de la carte
	// chez l'agrégateur — seul moyen de détecter une dépense ou un
	// changement de statut décidé côté agrégateur, faute de webhook dédié
	// (voir les commentaires sur carte.Carte.SoldeCentimes et
	// carte.Carte.SynchroniserStatut). Le statut est déjà traduit vers
	// carte.StatutCarte par l'implémentation : le domaine ne connaît jamais
	// les codes bruts du SDK.
	ObtenirEtatCarte(ctx context.Context, idExterne string) (soldeCentimes int64, statut carte.StatutCarte, err error)

	GelerCarte(ctx context.Context, idExterne string) error
	DegelerCarte(ctx context.Context, idExterne string) error

	// RechargerCarte ajoute des fonds à une carte existante et retourne le
	// solde résultant, tel que rapporté par l'agrégateur (jamais calculé
	// localement). montantUSDCentimes est en centimes de dollar (voir
	// CreerCarteParams.MontantUSDCentimes).
	RechargerCarte(ctx context.Context, idExterne string, montantUSDCentimes int64) (soldeApresCentimes int64, err error)

	// AnnulerCarte détruit définitivement la carte et retourne le solde
	// qui restait dessus au moment de l'annulation, à rembourser par
	// l'appelant.
	AnnulerCarte(ctx context.Context, idExterne string) (soldeRestantCentimes int64, err error)
}
