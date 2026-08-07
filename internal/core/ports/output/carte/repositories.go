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

// CreerCarteParams décrit une demande d'émission de carte virtuelle.
type CreerCarteParams struct {
	Label           string
	Devise          string
	MontantCentimes int64
}

// CreerCarteResultat est renvoyé une fois la carte effectivement émise
// par l'agrégateur (appel synchrone, contrairement au cash-in/cash-out :
// pas de webhook de confirmation à attendre).
type CreerCarteResultat struct {
	IDExterne string
}

// AgregateurCarte isole l'appel d'émission de carte virtuelle auprès de
// l'agrégateur de paiement externe (aujourd'hui HR-Skills Pay/Cartevo)
// derrière une interface propre au domaine.
//
// Important : comme wallet.AgregateurPaiement, ce port ne permet pas de
// contrôler une clé d'idempotence — la protection contre un double appel
// se fait via la Transaction locale (voir
// wallet.Transaction.MarquerEnvoyee). De plus, cette version du SDK
// n'expose aucun moyen de récupérer le PAN/CVV d'une carte : la
// consultation sécurisée du numéro complet n'est pas implémentable pour
// l'instant.
type AgregateurCarte interface {
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
	// localement).
	RechargerCarte(ctx context.Context, idExterne string, montantCentimes int64) (soldeApresCentimes int64, err error)

	// AnnulerCarte détruit définitivement la carte et retourne le solde
	// qui restait dessus au moment de l'annulation, à rembourser au
	// wallet par l'appelant.
	AnnulerCarte(ctx context.Context, idExterne string) (soldeRestantCentimes int64, err error)
}
