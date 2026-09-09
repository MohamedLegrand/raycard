// Package carte fournit des fakes des use cases carte (inputcarte.CarteUseCase,
// inputcarte.AdminCarteUseCase) pour les tests HTTP de la couche transport
// — voir carte_handler_test.go. Contrairement à test/application/carte,
// qui fake les ports SORTANTS (repositories, agrégateur) pour tester le
// service applicatif, ce paquet fake directement les ports ENTRANTS : on
// ne reteste jamais la logique métier ici (déjà couverte côté service),
// seulement le routing, la validation des DTO et le mapping d'erreurs
// (voir test/transport/http/harnais).
package carte

import (
	"context"

	domaincarte "raycard/internal/core/domain/carte"
	inputcarte "raycard/internal/core/ports/input/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// CarteUseCaseFake implémente inputcarte.CarteUseCase. Chaque méthode a
// son résultat/erreur configurable indépendamment ; DernierUtilisateurID/
// DernierCarteID capturent le dernier appel (tous appels confondus,
// suffisant pour vérifier que le handler transmet bien l'ID venu du JWT
// et celui venu du paramètre d'URL — pas besoin d'un champ par méthode).
type CarteUseCaseFake struct {
	SoumettrePorteurCarteResultat *domaincarte.CardCustomer
	SoumettrePorteurCarteErr      error
	DerniereReqSoumettrePorteur   inputcarte.SoumettrePorteurCarteRequest

	ObtenirStatutPorteurCarteResultat *domaincarte.CardCustomer
	ObtenirStatutPorteurCarteErr      error

	CreerCarteResultat    *domaincarte.Carte
	CreerCarteErr         error
	DerniereReqCreerCarte inputcarte.CreerCarteRequest

	ListerCartesResultat []*domaincarte.Carte
	ListerCartesErr      error

	ObtenirCarteResultat *domaincarte.Carte
	ObtenirCarteErr      error

	GelerCarteResultat *domaincarte.Carte
	GelerCarteErr      error

	DegelerCarteResultat *domaincarte.Carte
	DegelerCarteErr      error

	RechargerCarteResultat *domaincarte.Carte
	RechargerCarteErr      error
	DerniereReqRecharger   inputcarte.RechargerCarteRequest

	RetirerCarteResultat *domaincarte.Carte
	RetirerCarteErr      error
	DerniereReqRetirer   inputcarte.RetirerCarteRequest

	AnnulerCarteResultat *domaincarte.Carte
	AnnulerCarteErr      error

	ListerDepensesResultat []*domaincarte.DepenseCarte
	ListerDepensesErr      error

	SynchroniserSoldesResultat int
	SynchroniserSoldesErr      error

	DernierUtilisateurID string
	DernierCarteID       string
}

func (f *CarteUseCaseFake) SoumettrePorteurCarte(_ context.Context, utilisateurID string, req inputcarte.SoumettrePorteurCarteRequest) (*domaincarte.CardCustomer, error) {
	f.DernierUtilisateurID = utilisateurID
	f.DerniereReqSoumettrePorteur = req
	return f.SoumettrePorteurCarteResultat, f.SoumettrePorteurCarteErr
}

func (f *CarteUseCaseFake) ObtenirStatutPorteurCarte(_ context.Context, utilisateurID string) (*domaincarte.CardCustomer, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirStatutPorteurCarteResultat, f.ObtenirStatutPorteurCarteErr
}

func (f *CarteUseCaseFake) CreerCarte(_ context.Context, utilisateurID string, req inputcarte.CreerCarteRequest) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID = utilisateurID
	f.DerniereReqCreerCarte = req
	return f.CreerCarteResultat, f.CreerCarteErr
}

func (f *CarteUseCaseFake) ListerCartes(_ context.Context, utilisateurID string) ([]*domaincarte.Carte, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ListerCartesResultat, f.ListerCartesErr
}

func (f *CarteUseCaseFake) ObtenirCarte(_ context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	return f.ObtenirCarteResultat, f.ObtenirCarteErr
}

func (f *CarteUseCaseFake) GelerCarte(_ context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	return f.GelerCarteResultat, f.GelerCarteErr
}

func (f *CarteUseCaseFake) DegelerCarte(_ context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	return f.DegelerCarteResultat, f.DegelerCarteErr
}

func (f *CarteUseCaseFake) RechargerCarte(_ context.Context, utilisateurID, carteID string, req inputcarte.RechargerCarteRequest) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	f.DerniereReqRecharger = req
	return f.RechargerCarteResultat, f.RechargerCarteErr
}

func (f *CarteUseCaseFake) AnnulerCarte(_ context.Context, utilisateurID, carteID string) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	return f.AnnulerCarteResultat, f.AnnulerCarteErr
}

func (f *CarteUseCaseFake) RetirerCarte(_ context.Context, utilisateurID, carteID string, req inputcarte.RetirerCarteRequest) (*domaincarte.Carte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	f.DerniereReqRetirer = req
	return f.RetirerCarteResultat, f.RetirerCarteErr
}

func (f *CarteUseCaseFake) ListerDepenses(_ context.Context, utilisateurID, carteID string) ([]*domaincarte.DepenseCarte, error) {
	f.DernierUtilisateurID, f.DernierCarteID = utilisateurID, carteID
	return f.ListerDepensesResultat, f.ListerDepensesErr
}

func (f *CarteUseCaseFake) SynchroniserSoldes(_ context.Context) (int, error) {
	return f.SynchroniserSoldesResultat, f.SynchroniserSoldesErr
}

// AdminCarteUseCaseFake implémente inputcarte.AdminCarteUseCase, même
// principe que CarteUseCaseFake.
type AdminCarteUseCaseFake struct {
	ListerCartesAdminResultat []*domaincarte.Carte
	ListerCartesAdminErr      error
	DernierFiltre             outputcarte.FiltreCartes

	GelerCarteAdminResultat *domaincarte.Carte
	GelerCarteAdminErr      error

	DegelerCarteAdminResultat *domaincarte.Carte
	DegelerCarteAdminErr      error

	AnnulerCarteAdminResultat *domaincarte.Carte
	AnnulerCarteAdminErr      error

	ObtenirCardWalletAdminResultat int64
	ObtenirCardWalletAdminErr      error

	AlimenterCardWalletAdminResultat int64
	AlimenterCardWalletAdminErr      error
	DernierMontantAlimenter          int64

	DernierAdminID string
	DernierCarteID string
}

func (f *AdminCarteUseCaseFake) ListerCartesAdmin(_ context.Context, filtre outputcarte.FiltreCartes) ([]*domaincarte.Carte, error) {
	f.DernierFiltre = filtre
	return f.ListerCartesAdminResultat, f.ListerCartesAdminErr
}

func (f *AdminCarteUseCaseFake) GelerCarteAdmin(_ context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	f.DernierAdminID, f.DernierCarteID = adminID, carteID
	return f.GelerCarteAdminResultat, f.GelerCarteAdminErr
}

func (f *AdminCarteUseCaseFake) DegelerCarteAdmin(_ context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	f.DernierAdminID, f.DernierCarteID = adminID, carteID
	return f.DegelerCarteAdminResultat, f.DegelerCarteAdminErr
}

func (f *AdminCarteUseCaseFake) AnnulerCarteAdmin(_ context.Context, adminID, carteID string) (*domaincarte.Carte, error) {
	f.DernierAdminID, f.DernierCarteID = adminID, carteID
	return f.AnnulerCarteAdminResultat, f.AnnulerCarteAdminErr
}

func (f *AdminCarteUseCaseFake) ObtenirCardWalletAdmin(_ context.Context) (int64, error) {
	return f.ObtenirCardWalletAdminResultat, f.ObtenirCardWalletAdminErr
}

func (f *AdminCarteUseCaseFake) AlimenterCardWalletAdmin(_ context.Context, adminID string, montantUSDCentimes int64) (int64, error) {
	f.DernierAdminID = adminID
	f.DernierMontantAlimenter = montantUSDCentimes
	return f.AlimenterCardWalletAdminResultat, f.AlimenterCardWalletAdminErr
}
