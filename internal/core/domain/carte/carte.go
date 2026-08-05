// Package carte contient le domaine des cartes virtuelles, financées
// depuis le Wallet du module commun et émises via un agrégateur de
// paiement externe (aujourd'hui HR-Skills Pay / Cartevo).
package carte

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// StatutCarte représente l'état opérationnel d'une carte.
type StatutCarte string

const (
	StatutCarteActive  StatutCarte = "active"
	StatutCarteGelee   StatutCarte = "gelee"
	StatutCarteAnnulee StatutCarte = "annulee"
)

// Carte trace une carte virtuelle émise pour un utilisateur. Le PAN et le
// CVV ne sont jamais stockés ici (le SDK de l'agrégateur ne les expose
// d'ailleurs pas au-delà de la création) : seul l'identifiant technique
// externe est conservé, pour toute opération ultérieure (gel, annulation).
//
// SoldeCentimes est le dernier solde connu de la carte chez l'agrégateur.
// Aucun webhook de dépense n'existe côté HR-Skills Pay à ce jour : ce
// solde est tenu à jour par un job planifié qui interroge périodiquement
// l'agrégateur (voir CarteUseCase.SynchroniserSoldes) — jamais en temps
// réel.
//
// ProchaineVerificationAt et NiveauVerification pilotent la fréquence de
// ce sondage, carte par carte : dès qu'une dépense est détectée,
// l'intervalle revient au minimum (une carte qui vient de bouger a de
// bonnes chances de rebouger bientôt) ; sinon, il double à chaque passage
// sans dépense jusqu'à un plafond — même principe d'escalade que
// auth.VerrouConnexion, appliqué ici pour éviter de solliciter
// l'agrégateur au même rythme pour une carte dormante que pour une carte
// activement utilisée.
type Carte struct {
	ID                      string
	UtilisateurID           string
	WalletID                string
	IDExterne               string
	Label                   string
	Devise                  string
	MontantChargeCentimes   int64
	SoldeCentimes           int64
	ProchaineVerificationAt time.Time
	NiveauVerification      int
	Statut                  StatutCarte
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

// NouvelleCarte trace une carte déjà émise avec succès par l'agrégateur
// (idExterne obligatoire : cette fonction ne crée jamais la carte
// elle-même, voir carte.CarteUseCase.CreerCarte). Le solde initial
// correspond au montant chargé à l'émission.
func NouvelleCarte(utilisateurID, walletID, idExterne, label, devise string, montantChargeCentimes int64) (*Carte, error) {
	if utilisateurID == "" || walletID == "" || idExterne == "" || label == "" || devise == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if montantChargeCentimes <= 0 {
		return nil, commun.ErrMontantInvalide
	}

	maintenant := time.Now().UTC()
	return &Carte{
		ID:                      commun.NewID(),
		UtilisateurID:           utilisateurID,
		WalletID:                walletID,
		IDExterne:               idExterne,
		Label:                   label,
		Devise:                  devise,
		MontantChargeCentimes:   montantChargeCentimes,
		SoldeCentimes:           montantChargeCentimes,
		ProchaineVerificationAt: maintenant, // vérifiable dès l'émission
		NiveauVerification:      0,
		Statut:                  StatutCarteActive,
		CreatedAt:               maintenant,
		UpdatedAt:               maintenant,
	}, nil
}

// exposantVerificationMax borne le calcul de l'intervalle de vérification :
// au-delà, intervalleMax prend de toute façon le relais, ça évite juste un
// dépassement arithmétique si un même niveau persistait de façon
// anormalement longue (voir auth.exposantEscaladeMax, même principe).
const exposantVerificationMax = 20

// MettreAJourSolde compare le solde observé chez l'agrégateur au dernier
// solde connu, détecte une éventuelle dépense (montant retourné, 0 si le
// solde est stable ou en hausse — une hausse, ex: futur rechargement, n'est
// jamais traitée comme une dépense), et ajuste ProchaineVerificationAt :
// intervalleBase dès qu'une dépense est détectée, doublé sinon à chaque
// passage sans changement, plafonné à intervalleMax.
func (c *Carte) MettreAJourSolde(soldeObserveCentimes int64, maintenant time.Time, intervalleBase, intervalleMax time.Duration) (montantDepenseCentimes int64, err error) {
	if soldeObserveCentimes < 0 {
		return 0, commun.ErrMontantInvalide
	}
	if soldeObserveCentimes < c.SoldeCentimes {
		montantDepenseCentimes = c.SoldeCentimes - soldeObserveCentimes
	}
	c.SoldeCentimes = soldeObserveCentimes
	c.UpdatedAt = maintenant

	if montantDepenseCentimes > 0 {
		c.NiveauVerification = 0
	} else {
		c.NiveauVerification++
	}

	// exposant = NiveauVerification - 1 : le premier passage sans dépense
	// (NiveauVerification == 1) donne donc intervalleBase*2^0 = intervalleBase,
	// pas déjà le double (même décalage que auth.VerrouConnexion.EnregistrerEchec).
	exposant := c.NiveauVerification - 1
	if exposant > exposantVerificationMax {
		exposant = exposantVerificationMax
	}
	if exposant < 0 {
		exposant = 0
	}
	intervalle := intervalleBase * time.Duration(int64(1)<<uint(exposant))
	if intervalle <= 0 || intervalle > intervalleMax {
		intervalle = intervalleMax
	}
	c.ProchaineVerificationAt = maintenant.Add(intervalle)

	return montantDepenseCentimes, nil
}

// SynchroniserStatut aligne le statut local sur celui observé chez
// l'agrégateur — utile quand une carte est gelée ou annulée du côté de
// Cartevo sans passer par une action initiée depuis RAYCARD (ex: gel
// automatique pour fraude suspectée). Ne fait rien si le statut est déjà
// à jour. Une fois passée à Gelee ou Annulee, la carte sort naturellement
// du prochain sondage (voir CarteRepository.ListAVerifier, qui ne
// sélectionne que les cartes actives) : elle n'y revient que par une
// action explicite qui la repasse à StatutCarteActive (dégel).
func (c *Carte) SynchroniserStatut(statutObserve StatutCarte, maintenant time.Time) (change bool) {
	if c.Statut == statutObserve {
		return false
	}
	c.Statut = statutObserve
	c.UpdatedAt = maintenant
	return true
}

// Geler bloque la carte à la demande de l'utilisateur : plus aucune
// dépense possible tant qu'elle n'est pas dégelée. Ne s'applique qu'à une
// carte active.
func (c *Carte) Geler(maintenant time.Time) error {
	if c.Statut != StatutCarteActive {
		return ErrTransitionCarteInvalide
	}
	c.Statut = StatutCarteGelee
	c.UpdatedAt = maintenant
	return nil
}

// Degeler réactive une carte gelée. Réinitialise le sondage à
// l'intervalle de base (NiveauVerification à zéro, ProchaineVerificationAt
// à maintenant) : une carte tout juste réactivée mérite d'être vérifiée
// sans attendre l'échéance qu'elle avait avant d'être gelée, qui peut
// dater d'il y a longtemps puisque le gel l'excluait du sondage.
func (c *Carte) Degeler(maintenant time.Time) error {
	if c.Statut != StatutCarteGelee {
		return ErrTransitionCarteInvalide
	}
	c.Statut = StatutCarteActive
	c.NiveauVerification = 0
	c.ProchaineVerificationAt = maintenant
	c.UpdatedAt = maintenant
	return nil
}

// Recharger enregistre une recharge réussie : n'est permis que sur une
// carte active. soldeApresCentimes vient de la réponse de l'agrégateur
// (source de vérité, jamais un calcul local — la carte a pu bouger entre
// deux sondages).
func (c *Carte) Recharger(montantAjouteCentimes, soldeApresCentimes int64, maintenant time.Time) error {
	if c.Statut != StatutCarteActive {
		return ErrTransitionCarteInvalide
	}
	if montantAjouteCentimes <= 0 || soldeApresCentimes < 0 {
		return commun.ErrMontantInvalide
	}
	c.MontantChargeCentimes += montantAjouteCentimes
	c.SoldeCentimes = soldeApresCentimes
	c.UpdatedAt = maintenant
	return nil
}

// Annuler détruit définitivement la carte. Autorisé depuis Active ou
// Gelee (jamais depuis une carte déjà annulée). Remet le solde local à
// zéro : ce qui restait dessus est remboursé au wallet séparément, par
// l'appelant (voir carte.CarteUseCase.AnnulerCarte), une carte annulée
// n'ayant plus vocation à porter de solde.
func (c *Carte) Annuler(maintenant time.Time) error {
	if c.Statut != StatutCarteActive && c.Statut != StatutCarteGelee {
		return ErrTransitionCarteInvalide
	}
	c.Statut = StatutCarteAnnulee
	c.SoldeCentimes = 0
	c.UpdatedAt = maintenant
	return nil
}
