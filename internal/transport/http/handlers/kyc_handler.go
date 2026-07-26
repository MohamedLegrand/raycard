package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/transport/http/dto"
)

type KycHandler struct {
	kycUseCase input.KycUseCase
	validate   *validator.Validate
}

func NewKycHandler(kycUseCase input.KycUseCase, validate *validator.Validate) *KycHandler {
	return &KycHandler{kycUseCase: kycUseCase, validate: validate}
}

// Inscrire gère POST /api/v1/kyc/inscription.
func (h *KycHandler) Inscrire(c *fiber.Ctx) error {
	var req dto.InscriptionRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.kycUseCase.Inscrire(c.Context(), req.ToUseCaseRequest())
	if err != nil {
		return mapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.FromInscriptionResultat(resultat))
}

// mapErreurDomaine traduit une erreur métier en erreur HTTP Fiber.
// Les erreurs non reconnues restent des 500 : on ne fuite jamais de
// détail d'infrastructure (ex: erreur pgx brute) au client.
func mapErreurDomaine(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmailDejaUtilise), errors.Is(err, domain.ErrTelephoneDejaUtilise):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrPaysNonSupporte), errors.Is(err, domain.ErrDonneesInvalides), errors.Is(err, domain.ErrTransitionKycInvalide):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "erreur interne")
	}
}
