// Package kyc contient les handlers HTTP de l'inscription et de la
// revue KYC.
package kyc

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	inputkyc "raycard/internal/core/ports/input/kyc"
	kycdto "raycard/internal/transport/http/dto/kyc"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type KycHandler struct {
	kycUseCase inputkyc.KycUseCase
	validate   *validator.Validate
}

func NewKycHandler(kycUseCase inputkyc.KycUseCase, validate *validator.Validate) *KycHandler {
	return &KycHandler{kycUseCase: kycUseCase, validate: validate}
}

// Inscrire gère POST /api/v1/kyc/inscription.
//
//	@Summary		Inscription d'un nouvel utilisateur
//	@Description	Crée un utilisateur et son wallet associé (KYC Tier 1 auto-validé selon les règles du pays)
//	@Tags			kyc
//	@Accept			json
//	@Produce		json
//	@Param			inscription	body		kyc.InscriptionRequestDTO	true	"Données d'inscription"
//	@Success		201			{object}	kyc.InscriptionResponseDTO
//	@Failure		400			{object}	commun.ErreurDTO	"corps de requête invalide ou données de validation incorrectes"
//	@Failure		409			{object}	commun.ErreurDTO	"email ou téléphone déjà utilisé"
//	@Failure		422			{object}	commun.ErreurDTO	"pays non supporté ou données métier invalides"
//	@Failure		500			{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/kyc/inscription [post]
func (h *KycHandler) Inscrire(c *fiber.Ctx) error {
	var req kycdto.InscriptionRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.kycUseCase.Inscrire(c.Context(), req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(kycdto.FromInscriptionResultat(resultat))
}

// DemanderTier2 gère POST /api/v1/kyc/demande-tier2 (route protégée :
// l'utilisateur authentifié demande son propre passage au Tier 2).
//
//	@Summary		Demande de passage au Tier 2
//	@Description	Crée un dossier KYC en attente de revue par un administrateur
//	@Tags			kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Success		201	{object}	kyc.DossierKycDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		409	{object}	commun.ErreurDTO	"une demande est déjà en attente"
//	@Failure		422	{object}	commun.ErreurDTO	"l'utilisateur n'est pas au Tier 1"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/kyc/demande-tier2 [post]
func (h *KycHandler) DemanderTier2(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	dossier, err := h.kycUseCase.DemanderTier2(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(kycdto.FromDossierKyc(dossier))
}
