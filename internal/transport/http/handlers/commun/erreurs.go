// Package commun regroupe les éléments transverses aux handlers HTTP de
// plusieurs modules, notamment la traduction des erreurs métier en
// erreurs HTTP.
package commun

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	authdomain "raycard/internal/core/domain/auth"
	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/kyc"
)

// MapErreurDomaine traduit une erreur métier en erreur HTTP Fiber.
// Les erreurs non reconnues restent des 500 : on ne fuite jamais de
// détail d'infrastructure (ex: erreur pgx brute) au client.
func MapErreurDomaine(err error) error {
	switch {
	case errors.Is(err, commun.ErrEmailDejaUtilise), errors.Is(err, commun.ErrTelephoneDejaUtilise), errors.Is(err, kyc.ErrDossierKycDejaEnAttente):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, commun.ErrPaysNonSupporte), errors.Is(err, commun.ErrDonneesInvalides), errors.Is(err, commun.ErrTransitionKycInvalide), errors.Is(err, kyc.ErrFormatDocumentInvalide):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, authdomain.ErrIdentifiantsInvalides), errors.Is(err, authdomain.ErrTokenInvalide):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, commun.ErrUtilisateurIntrouvable), errors.Is(err, kyc.ErrDossierKycIntrouvable), errors.Is(err, authdomain.ErrCleAppareilIntrouvable):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "erreur interne")
	}
}
