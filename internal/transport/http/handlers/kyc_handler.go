package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
	"raycard/internal/transport/http/dto"
	"raycard/internal/transport/http/middleware"
)

type KycHandler struct {
	kycUseCase input.KycUseCase
	validate   *validator.Validate
}

func NewKycHandler(kycUseCase input.KycUseCase, validate *validator.Validate) *KycHandler {
	return &KycHandler{kycUseCase: kycUseCase, validate: validate}
}

// Inscrire gère POST /api/v1/kyc/inscription.
//
//	@Summary		Inscription d'un nouvel utilisateur
//	@Description	Crée un utilisateur et son wallet associé (KYC Tier 1 auto-validé selon les règles du pays)
//	@Tags			kyc
//	@Accept			json
//	@Produce		json
//	@Param			inscription	body		dto.InscriptionRequestDTO	true	"Données d'inscription"
//	@Success		201			{object}	dto.InscriptionResponseDTO
//	@Failure		400			{object}	dto.ErreurDTO	"corps de requête invalide ou données de validation incorrectes"
//	@Failure		409			{object}	dto.ErreurDTO	"email ou téléphone déjà utilisé"
//	@Failure		422			{object}	dto.ErreurDTO	"pays non supporté ou données métier invalides"
//	@Failure		500			{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/kyc/inscription [post]
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

// DemanderTier2 gère POST /api/v1/kyc/demande-tier2 (route protégée :
// l'utilisateur authentifié demande son propre passage au Tier 2).
//
//	@Summary		Demande de passage au Tier 2
//	@Description	Crée un dossier KYC en attente de revue par un administrateur
//	@Tags			kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Success		201	{object}	dto.DossierKycDTO
//	@Failure		401	{object}	dto.ErreurDTO	"non authentifié"
//	@Failure		409	{object}	dto.ErreurDTO	"une demande est déjà en attente"
//	@Failure		422	{object}	dto.ErreurDTO	"l'utilisateur n'est pas au Tier 1"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/kyc/demande-tier2 [post]
func (h *KycHandler) DemanderTier2(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(middleware.CleContextUtilisateurID).(string)

	dossier, err := h.kycUseCase.DemanderTier2(c.Context(), utilisateurID)
	if err != nil {
		return mapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto.FromDossierKyc(dossier))
}

// mapErreurDomaine traduit une erreur métier en erreur HTTP Fiber.
// Les erreurs non reconnues restent des 500 : on ne fuite jamais de
// détail d'infrastructure (ex: erreur pgx brute) au client.
func mapErreurDomaine(err error) error {
	switch {
	case errors.Is(err, domain.ErrEmailDejaUtilise), errors.Is(err, domain.ErrTelephoneDejaUtilise), errors.Is(err, domain.ErrDossierKycDejaEnAttente):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrPaysNonSupporte), errors.Is(err, domain.ErrDonneesInvalides), errors.Is(err, domain.ErrTransitionKycInvalide):
		return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, domain.ErrIdentifiantsInvalides), errors.Is(err, domain.ErrTokenInvalide):
		return fiber.NewError(fiber.StatusUnauthorized, err.Error())
	case errors.Is(err, domain.ErrUtilisateurIntrouvable), errors.Is(err, domain.ErrDossierKycIntrouvable):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "erreur interne")
	}
}
