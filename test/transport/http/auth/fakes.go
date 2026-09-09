// Package auth fournit un fake de inputauth.AuthUseCase pour les tests
// HTTP de la couche transport — même principe que
// test/transport/http/carte.
package auth

import (
	"context"

	"raycard/internal/core/domain/commun"
	inputauth "raycard/internal/core/ports/input/auth"
)

type AuthUseCaseFake struct {
	ConnexionResultat    *inputauth.ConnexionResultat
	ConnexionErr         error
	DerniereReqConnexion inputauth.ConnexionRequest

	VerifierCode2FAResultat *inputauth.SessionResultat
	VerifierCode2FAErr      error
	DernierTicket           string
	DernierCode             string

	ConnexionGoogleResultat *inputauth.ConnexionResultat
	ConnexionGoogleErr      error

	RafraichirTokenResultat *inputauth.SessionResultat
	RafraichirTokenErr      error
	DernierRefreshToken     string

	DeconnexionErr error

	DemanderReinitialisationErr error
	DernierEmail                string

	ReinitialiserErr error

	EnregistrerAppareilResultat *inputauth.AppareilResultat
	EnregistrerAppareilErr      error

	RevoquerAppareilErr error
	DernierAppareilID   string

	DemanderChallengeEmpreinteResultat *inputauth.ChallengeEmpreinteResultat
	DemanderChallengeEmpreinteErr      error

	ConnexionEmpreinteResultat *inputauth.SessionResultat
	ConnexionEmpreinteErr      error

	ObtenirProfilResultat *commun.Utilisateur
	ObtenirProfilErr      error

	ModifierProfilResultat *commun.Utilisateur
	ModifierProfilErr      error

	ModifierPhotoProfilResultat *commun.Utilisateur
	ModifierPhotoProfilErr      error

	ObtenirPhotoProfilContenu     []byte
	ObtenirPhotoProfilContentType string
	ObtenirPhotoProfilErr         error

	ChangerMotDePasseErr error

	DemanderChangementEmailErr error
	DernierNouvelEmail         string

	ConfirmerChangementEmailResultat *commun.Utilisateur
	ConfirmerChangementEmailErr      error

	DernierUtilisateurID string
}

func (f *AuthUseCaseFake) Connexion(_ context.Context, req inputauth.ConnexionRequest) (*inputauth.ConnexionResultat, error) {
	f.DerniereReqConnexion = req
	return f.ConnexionResultat, f.ConnexionErr
}

func (f *AuthUseCaseFake) VerifierCode2FA(_ context.Context, ticket, code string, _ inputauth.MetadonneesConnexion) (*inputauth.SessionResultat, error) {
	f.DernierTicket, f.DernierCode = ticket, code
	return f.VerifierCode2FAResultat, f.VerifierCode2FAErr
}

func (f *AuthUseCaseFake) ConnexionGoogle(_ context.Context, _ inputauth.ConnexionGoogleRequest) (*inputauth.ConnexionResultat, error) {
	return f.ConnexionGoogleResultat, f.ConnexionGoogleErr
}

func (f *AuthUseCaseFake) RafraichirToken(_ context.Context, refreshToken string) (*inputauth.SessionResultat, error) {
	f.DernierRefreshToken = refreshToken
	return f.RafraichirTokenResultat, f.RafraichirTokenErr
}

func (f *AuthUseCaseFake) Deconnexion(_ context.Context, refreshToken string) error {
	f.DernierRefreshToken = refreshToken
	return f.DeconnexionErr
}

func (f *AuthUseCaseFake) DemanderReinitialisation(_ context.Context, email string) error {
	f.DernierEmail = email
	return f.DemanderReinitialisationErr
}

func (f *AuthUseCaseFake) Reinitialiser(_ context.Context, _, _, _ string) error {
	return f.ReinitialiserErr
}

func (f *AuthUseCaseFake) EnregistrerAppareil(_ context.Context, utilisateurID string, _ inputauth.EnregistrerAppareilRequest) (*inputauth.AppareilResultat, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.EnregistrerAppareilResultat, f.EnregistrerAppareilErr
}

func (f *AuthUseCaseFake) RevoquerAppareil(_ context.Context, utilisateurID, appareilID string) error {
	f.DernierUtilisateurID, f.DernierAppareilID = utilisateurID, appareilID
	return f.RevoquerAppareilErr
}

func (f *AuthUseCaseFake) DemanderChallengeEmpreinte(_ context.Context, appareilID string) (*inputauth.ChallengeEmpreinteResultat, error) {
	f.DernierAppareilID = appareilID
	return f.DemanderChallengeEmpreinteResultat, f.DemanderChallengeEmpreinteErr
}

func (f *AuthUseCaseFake) ConnexionEmpreinte(_ context.Context, _ inputauth.VerifierEmpreinteRequest, _ inputauth.MetadonneesConnexion) (*inputauth.SessionResultat, error) {
	return f.ConnexionEmpreinteResultat, f.ConnexionEmpreinteErr
}

func (f *AuthUseCaseFake) ObtenirProfil(_ context.Context, utilisateurID string) (*commun.Utilisateur, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirProfilResultat, f.ObtenirProfilErr
}

func (f *AuthUseCaseFake) ModifierProfil(_ context.Context, utilisateurID string, _ inputauth.ModifierProfilRequest) (*commun.Utilisateur, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ModifierProfilResultat, f.ModifierProfilErr
}

func (f *AuthUseCaseFake) ModifierPhotoProfil(_ context.Context, utilisateurID, _ string, _ []byte) (*commun.Utilisateur, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ModifierPhotoProfilResultat, f.ModifierPhotoProfilErr
}

func (f *AuthUseCaseFake) ObtenirPhotoProfil(_ context.Context, utilisateurID string) ([]byte, string, error) {
	f.DernierUtilisateurID = utilisateurID
	return f.ObtenirPhotoProfilContenu, f.ObtenirPhotoProfilContentType, f.ObtenirPhotoProfilErr
}

func (f *AuthUseCaseFake) ChangerMotDePasse(_ context.Context, utilisateurID, _, _ string) error {
	f.DernierUtilisateurID = utilisateurID
	return f.ChangerMotDePasseErr
}

func (f *AuthUseCaseFake) DemanderChangementEmail(_ context.Context, utilisateurID, nouvelEmail string) error {
	f.DernierUtilisateurID, f.DernierNouvelEmail = utilisateurID, nouvelEmail
	return f.DemanderChangementEmailErr
}

func (f *AuthUseCaseFake) ConfirmerChangementEmail(_ context.Context, _ string) (*commun.Utilisateur, error) {
	return f.ConfirmerChangementEmailResultat, f.ConfirmerChangementEmailErr
}
