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
	outputcarte "raycard/internal/core/ports/output/carte"
)

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
type AgregateurCarteFake struct {
	IDExterneGenere     string
	ErreurEmission      error
	AppelsCreerCarte    int
	AppelsObtenirEtat   int
	SoldesParIDExterne  map[string]int64
	StatutsParIDExterne map[string]domaincarte.StatutCarte
	ErreurObtenirEtat   error
	ErreurGel           error
	ErreurDegel         error
	AppelsGeler         int
	AppelsDegeler       int
	ErreurRecharge      error
	AppelsRecharger     int
	SoldeApresRecharge  int64
	ErreurAnnulation    error
	AppelsAnnuler       int
	SoldeRestantAnnule  int64
}

func (a *AgregateurCarteFake) CreerCarte(_ context.Context, _ outputcarte.CreerCarteParams) (*outputcarte.CreerCarteResultat, error) {
	a.AppelsCreerCarte++
	if a.ErreurEmission != nil {
		return nil, a.ErreurEmission
	}
	idExterne := a.IDExterneGenere
	if idExterne == "" {
		idExterne = "card-fake-1"
	}
	return &outputcarte.CreerCarteResultat{IDExterne: idExterne}, nil
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
