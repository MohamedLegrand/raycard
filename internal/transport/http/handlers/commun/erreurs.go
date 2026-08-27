// Package commun regroupe les éléments transverses aux handlers HTTP de
// plusieurs modules, notamment la traduction des erreurs métier en
// erreurs HTTP.
package commun

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	authdomain "raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/carte"
	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/kyc"
	"raycard/internal/core/domain/wallet"
)

// MapErreurDomaine traduit une erreur métier en erreur HTTP Fiber.
// Les erreurs non reconnues restent des 500 : on ne fuite jamais de
// détail d'infrastructure (ex: erreur pgx brute) au client.
func MapErreurDomaine(err error) error {
	switch {
	case errors.Is(err, commun.ErrEmailDejaUtilise), errors.Is(err, commun.ErrTelephoneDejaUtilise), errors.Is(err, kyc.ErrDossierKycDejaEnAttente), errors.Is(err, wallet.ErrTransactionDejaEnCours):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, commun.ErrPaysNonSupporte), errors.Is(err, commun.ErrDonneesInvalides), errors.Is(err, commun.ErrTransitionKycInvalide), errors.Is(err, kyc.ErrFormatDocumentInvalide), errors.Is(err, kyc.ErrDossierKycNonModifiable),
		errors.Is(err, commun.ErrMontantInvalide), errors.Is(err, commun.ErrPlafondDepasse), errors.Is(err, commun.ErrSoldeInsuffisant), errors.Is(err, commun.ErrWalletGele), errors.Is(err, commun.ErrTransitionWalletInvalide), errors.Is(err, wallet.ErrOperateurNonSupporte),
		errors.Is(err, carte.ErrKycTierInsuffisant), errors.Is(err, carte.ErrEmissionEchouee), errors.Is(err, carte.ErrTransitionCarteInvalide), errors.Is(err, carte.ErrRechargeEchouee), errors.Is(err, carte.ErrAnnulationEchouee):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, authdomain.ErrIdentifiantsInvalides), errors.Is(err, authdomain.ErrTokenInvalide), errors.Is(err, wallet.ErrWebhookSignatureInvalide):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, commun.ErrUtilisateurIntrouvable), errors.Is(err, kyc.ErrDossierKycIntrouvable), errors.Is(err, kyc.ErrDocumentKycIntrouvable), errors.Is(err, authdomain.ErrCleAppareilIntrouvable),
		errors.Is(err, commun.ErrWalletIntrouvable), errors.Is(err, wallet.ErrTransactionIntrouvable), errors.Is(err, carte.ErrCarteIntrouvable), errors.Is(err, commun.ErrPhotoProfilAbsente):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, authdomain.ErrCompteVerrouille):
		return fiber.NewError(fiber.StatusTooManyRequests, err.Error())
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "erreur interne")
	}
}
