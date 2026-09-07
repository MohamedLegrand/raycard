// Package carte fournit de faux repositories et un faux agrégateur de
// cartes en mémoire pour les tests du service applicatif carte. Ce n'est
// volontairement pas un fichier _test.go : carte_service_test.go vit dans
// le package carte_test et doit pouvoir importer ces symboles.
package carte

import (
	"context"
	"errors"
	"time"

	domaincarte "raycard/internal/core/domain/carte"
	domainkyc "raycard/internal/core/domain/kyc"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// CardCustomerRepoFake stocke un porteur de carte par utilisateur, comme
// la contrainte d'unicité réelle (voir la migration card_customers).
type CardCustomerRepoFake struct {
	parUtilisateurID map[string]*domaincarte.CardCustomer
}

func NewCardCustomerRepoFake() *CardCustomerRepoFake {
	return &CardCustomerRepoFake{parUtilisateurID: make(map[string]*domaincarte.CardCustomer)}
}

func (r *CardCustomerRepoFake) Create(_ context.Context, c *domaincarte.CardCustomer) error {
	r.parUtilisateurID[c.UtilisateurID] = c
	return nil
}

func (r *CardCustomerRepoFake) FindByUtilisateurID(_ context.Context, utilisateurID string) (*domaincarte.CardCustomer, error) {
	if c, ok := r.parUtilisateurID[utilisateurID]; ok {
		return c, nil
	}
	return nil, domaincarte.ErrCardCustomerIntrouvable
}

func (r *CardCustomerRepoFake) Update(_ context.Context, c *domaincarte.CardCustomer) error {
	if _, ok := r.parUtilisateurID[c.UtilisateurID]; !ok {
		return domaincarte.ErrCardCustomerIntrouvable
	}
	r.parUtilisateurID[c.UtilisateurID] = c
	return nil
}

// DossierKycRepoFake et DocumentKycRepoFake : sous-ensemble minimal des
// interfaces kyc.DossierKycRepository / kyc.DocumentKycRepository,
// suffisant pour carteService.recuperationPiecesIdentiteApprouvees —
// jamais partagé avec test/application/kyc (fakes propres à des fichiers
// _test.go là-bas, non importables ici).
type DossierKycRepoFake struct {
	DernierParUtilisateurID map[string]*domainkyc.DossierKyc
}

func NewDossierKycRepoFake() *DossierKycRepoFake {
	return &DossierKycRepoFake{DernierParUtilisateurID: make(map[string]*domainkyc.DossierKyc)}
}

func (r *DossierKycRepoFake) Create(_ context.Context, d *domainkyc.DossierKyc) error {
	r.DernierParUtilisateurID[d.UtilisateurID] = d
	return nil
}
func (r *DossierKycRepoFake) FindByID(_ context.Context, _ string) (*domainkyc.DossierKyc, error) {
	return nil, errors.New("DossierKycRepoFake: FindByID non implémenté")
}
func (r *DossierKycRepoFake) FindEnAttenteByUtilisateurID(_ context.Context, _ string) (*domainkyc.DossierKyc, error) {
	return nil, errors.New("DossierKycRepoFake: FindEnAttenteByUtilisateurID non implémenté")
}
func (r *DossierKycRepoFake) FindDernierByUtilisateurID(_ context.Context, utilisateurID string) (*domainkyc.DossierKyc, error) {
	if d, ok := r.DernierParUtilisateurID[utilisateurID]; ok {
		return d, nil
	}
	return nil, domainkyc.ErrDossierKycIntrouvable
}
func (r *DossierKycRepoFake) ListEnAttente(_ context.Context) ([]*domainkyc.DossierKyc, error) {
	return nil, nil
}
func (r *DossierKycRepoFake) ListAll(_ context.Context) ([]*domainkyc.DossierKyc, error) {
	return nil, nil
}
func (r *DossierKycRepoFake) Update(_ context.Context, d *domainkyc.DossierKyc) error {
	r.DernierParUtilisateurID[d.UtilisateurID] = d
	return nil
}

type DocumentKycRepoFake struct {
	ParDossierKycID map[string][]*domainkyc.DocumentKyc
}

func NewDocumentKycRepoFake() *DocumentKycRepoFake {
	return &DocumentKycRepoFake{ParDossierKycID: make(map[string][]*domainkyc.DocumentKyc)}
}

func (r *DocumentKycRepoFake) Create(_ context.Context, d *domainkyc.DocumentKyc) error {
	r.ParDossierKycID[d.DossierKycID] = append(r.ParDossierKycID[d.DossierKycID], d)
	return nil
}
func (r *DocumentKycRepoFake) FindByID(_ context.Context, _ string) (*domainkyc.DocumentKyc, error) {
	return nil, errors.New("DocumentKycRepoFake: FindByID non implémenté")
}
func (r *DocumentKycRepoFake) ListByDossierKycID(_ context.Context, dossierKycID string) ([]*domainkyc.DocumentKyc, error) {
	return r.ParDossierKycID[dossierKycID], nil
}

type CarteRepoFake struct {
	parID map[string]*domaincarte.Carte
}

func NewCarteRepoFake() *CarteRepoFake {
	return &CarteRepoFake{parID: make(map[string]*domaincarte.Carte)}
}

func (r *CarteRepoFake) Create(_ context.Context, c *domaincarte.Carte) error {
	r.parID[c.ID] = c
	return nil
}

func (r *CarteRepoFake) FindByID(_ context.Context, id string) (*domaincarte.Carte, error) {
	if c, ok := r.parID[id]; ok {
		return c, nil
	}
	return nil, domaincarte.ErrCarteIntrouvable
}

func (r *CarteRepoFake) ListByUtilisateurID(_ context.Context, utilisateurID string) ([]*domaincarte.Carte, error) {
	var cartes []*domaincarte.Carte
	for _, c := range r.parID {
		if c.UtilisateurID == utilisateurID {
			cartes = append(cartes, c)
		}
	}
	return cartes, nil
}

func (r *CarteRepoFake) ListAVerifier(_ context.Context, avant time.Time) ([]*domaincarte.Carte, error) {
	var cartes []*domaincarte.Carte
	for _, c := range r.parID {
		if c.Statut == domaincarte.StatutCarteActive && !c.ProchaineVerificationAt.After(avant) {
			cartes = append(cartes, c)
		}
	}
	return cartes, nil
}

func (r *CarteRepoFake) ListToutes(_ context.Context, filtre outputcarte.FiltreCartes) ([]*domaincarte.Carte, error) {
	var cartes []*domaincarte.Carte
	for _, c := range r.parID {
		if filtre.UtilisateurID != "" && c.UtilisateurID != filtre.UtilisateurID {
			continue
		}
		if filtre.Statut != "" && string(c.Statut) != filtre.Statut {
			continue
		}
		cartes = append(cartes, c)
	}
	return cartes, nil
}

func (r *CarteRepoFake) Update(_ context.Context, c *domaincarte.Carte) error {
	if _, ok := r.parID[c.ID]; !ok {
		return domaincarte.ErrCarteIntrouvable
	}
	r.parID[c.ID] = c
	return nil
}

type DepenseCarteRepoFake struct {
	parCarteID map[string][]*domaincarte.DepenseCarte
}

func NewDepenseCarteRepoFake() *DepenseCarteRepoFake {
	return &DepenseCarteRepoFake{parCarteID: make(map[string][]*domaincarte.DepenseCarte)}
}

func (r *DepenseCarteRepoFake) Create(_ context.Context, d *domaincarte.DepenseCarte) error {
	r.parCarteID[d.CarteID] = append(r.parCarteID[d.CarteID], d)
	return nil
}

func (r *DepenseCarteRepoFake) ListByCarteID(_ context.Context, carteID string) ([]*domaincarte.DepenseCarte, error) {
	return r.parCarteID[carteID], nil
}

// AgregateurCarteFake simule l'agrégateur de cartes. ErreurEmission, si
// non nil, est renvoyée par CreerCarte (simule un échec réseau ou une
// erreur métier de l'agrégateur). SoldesParIDExterne fournit le solde
// renvoyé par ObtenirEtatCarte pour chaque carte ; StatutsParIDExterne le
// statut (StatutCarteActive si absent — pas besoin de le préciser dans
// les tests qui ne portent pas sur un changement de statut).
//
// EchecsAvantSucces, si strictement positif, borne le nombre de fois où
// CreerCarte renvoie ErreurEmission avant de réussir — simule le
// financement automatique borné du portefeuille cartes (voir
// carteService.creerCarteAvecFinancementAutomatique) : mettre
// ErreurEmission = domaincarte.ErrCardWalletInsuffisant et
// EchecsAvantSucces = 1 reproduit un premier appel en solde insuffisant,
// suivi d'un second qui réussit après financement. Laissé à zéro (défaut),
// ErreurEmission échoue à chaque appel, sans limite — comportement
// historique attendu par la plupart des tests de ce paquet.
type AgregateurCarteFake struct {
	IDExterneGenere       string
	ErreurEmission        error
	EchecsAvantSucces     int
	AppelsCreerCarte      int
	AppelsObtenirEtat     int
	SoldesParIDExterne    map[string]int64
	StatutsParIDExterne   map[string]domaincarte.StatutCarte
	ErreurObtenirEtat     error
	ErreurGel             error
	ErreurDegel           error
	AppelsGeler           int
	AppelsDegeler         int
	ErreurRecharge        error
	AppelsRecharger       int
	SoldeApresRecharge    int64
	ErreurAnnulation      error
	AppelsAnnuler         int
	SoldeRestantAnnule    int64
	ErreurSoumission      error
	IDExterneCustomer     string
	AppelsSoumettre       int
	SoldeCardWallet       int64
	ErreurCardWallet      error
	TauxConversion        float64 // XAF -> USD, 1.0 par défaut (voir CoterConversion)
	ErreurCotation        error
	AppelsAlimenter       int
	SoldeApresAlimenter   int64
	ErreurAlimentation    error
	TauxConversionInverse float64 // USD -> XAF, 1.0 par défaut (voir CoterConversionInverse)
	ErreurCotationInverse error
	AppelsRetirer         int
	SoldeApresRetrait     int64
	MontantCrediteRetrait int64 // 0 par défaut = pas de frais simulé, renvoie le montant demandé
	ErreurRetrait         error
}

func (a *AgregateurCarteFake) CreerCarte(_ context.Context, _ outputcarte.CreerCarteParams) (*outputcarte.CreerCarteResultat, error) {
	a.AppelsCreerCarte++
	if a.ErreurEmission != nil && (a.EchecsAvantSucces == 0 || a.AppelsCreerCarte <= a.EchecsAvantSucces) {
		return nil, a.ErreurEmission
	}
	idExterne := a.IDExterneGenere
	if idExterne == "" {
		idExterne = "card-fake-1"
	}
	return &outputcarte.CreerCarteResultat{IDExterne: idExterne}, nil
}

func (a *AgregateurCarteFake) SoumettreCardCustomer(_ context.Context, _ outputcarte.SoumettreCardCustomerParams) (*outputcarte.SoumettreCardCustomerResultat, error) {
	a.AppelsSoumettre++
	if a.ErreurSoumission != nil {
		return nil, a.ErreurSoumission
	}
	idExterne := a.IDExterneCustomer
	if idExterne == "" {
		idExterne = "cust-fake-1"
	}
	return &outputcarte.SoumettreCardCustomerResultat{IDExterne: idExterne}, nil
}

func (a *AgregateurCarteFake) ObtenirCardWallet(_ context.Context) (int64, error) {
	if a.ErreurCardWallet != nil {
		return 0, a.ErreurCardWallet
	}
	return a.SoldeCardWallet, nil
}

func (a *AgregateurCarteFake) CoterConversion(_ context.Context, montantXAFCentimes int64) (int64, error) {
	if a.ErreurCotation != nil {
		return 0, a.ErreurCotation
	}
	taux := a.TauxConversion
	if taux == 0 {
		taux = 1
	}
	return int64(float64(montantXAFCentimes) * taux), nil
}

func (a *AgregateurCarteFake) AlimenterCardWallet(_ context.Context, _ int64) (int64, error) {
	a.AppelsAlimenter++
	if a.ErreurAlimentation != nil {
		return 0, a.ErreurAlimentation
	}
	return a.SoldeApresAlimenter, nil
}

func (a *AgregateurCarteFake) ObtenirEtatCarte(_ context.Context, idExterne string) (int64, domaincarte.StatutCarte, error) {
	a.AppelsObtenirEtat++
	if a.ErreurObtenirEtat != nil {
		return 0, "", a.ErreurObtenirEtat
	}
	solde, ok := a.SoldesParIDExterne[idExterne]
	if !ok {
		return 0, "", errors.New("AgregateurCarteFake: aucun solde configuré pour " + idExterne)
	}
	statut, ok := a.StatutsParIDExterne[idExterne]
	if !ok {
		statut = domaincarte.StatutCarteActive
	}
	return solde, statut, nil
}

func (a *AgregateurCarteFake) GelerCarte(_ context.Context, _ string) error {
	a.AppelsGeler++
	return a.ErreurGel
}

func (a *AgregateurCarteFake) DegelerCarte(_ context.Context, _ string) error {
	a.AppelsDegeler++
	return a.ErreurDegel
}

func (a *AgregateurCarteFake) RechargerCarte(_ context.Context, _ string, _ int64) (int64, error) {
	a.AppelsRecharger++
	if a.ErreurRecharge != nil {
		return 0, a.ErreurRecharge
	}
	return a.SoldeApresRecharge, nil
}

func (a *AgregateurCarteFake) AnnulerCarte(_ context.Context, _ string) (int64, error) {
	a.AppelsAnnuler++
	if a.ErreurAnnulation != nil {
		return 0, a.ErreurAnnulation
	}
	return a.SoldeRestantAnnule, nil
}

func (a *AgregateurCarteFake) CoterConversionInverse(_ context.Context, montantUSDCentimes int64) (int64, error) {
	if a.ErreurCotationInverse != nil {
		return 0, a.ErreurCotationInverse
	}
	taux := a.TauxConversionInverse
	if taux == 0 {
		taux = 1
	}
	return int64(float64(montantUSDCentimes) * taux), nil
}

func (a *AgregateurCarteFake) RetirerCarte(_ context.Context, _ string, montantUSDCentimes int64) (int64, int64, error) {
	a.AppelsRetirer++
	if a.ErreurRetrait != nil {
		return 0, 0, a.ErreurRetrait
	}
	montantCredite := a.MontantCrediteRetrait
	if montantCredite == 0 {
		montantCredite = montantUSDCentimes // par défaut, aucun frais simulé
	}
	return a.SoldeApresRetrait, montantCredite, nil
}
