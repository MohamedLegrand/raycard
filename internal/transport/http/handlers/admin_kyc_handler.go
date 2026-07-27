package handlers

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/ports/input"
	"raycard/internal/transport/http/dto"
	"raycard/internal/transport/http/middleware"
)

type AdminKycHandler struct {
	adminKycUseCase input.AdminKycUseCase
	validate        *validator.Validate
}

func NewAdminKycHandler(adminKycUseCase input.AdminKycUseCase, validate *validator.Validate) *AdminKycHandler {
	return &AdminKycHandler{adminKycUseCase: adminKycUseCase, validate: validate}
}

// ListerDossiersEnAttente gère GET /api/v1/backoffice/kyc/dossiers.
//
//	@Summary		Liste des dossiers KYC en attente
//	@Tags			backoffice-kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		dto.DossierKycDTO
//	@Failure		401	{object}	dto.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	dto.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers [get]
func (h *AdminKycHandler) ListerDossiersEnAttente(c *fiber.Ctx) error {
	dossiers, err := h.adminKycUseCase.ListerDossiersEnAttente(c.Context())
	if err != nil {
		return mapErreurDomaine(err)
	}
	return c.Status(fiber.StatusOK).JSON(dto.FromDossiersKyc(dossiers))
}

// Approuver gère POST /api/v1/backoffice/kyc/dossiers/:id/approuver.
//
//	@Summary		Approuve un dossier KYC
//	@Description	Fait passer l'utilisateur au Tier 2 et écrit une entrée d'audit
//	@Tags			backoffice-kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"ID du dossier"
//	@Success		204
//	@Failure		401	{object}	dto.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	dto.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	dto.ErreurDTO	"dossier introuvable"
//	@Failure		422	{object}	dto.ErreurDTO	"dossier déjà traité"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers/{id}/approuver [post]
func (h *AdminKycHandler) Approuver(c *fiber.Ctx) error {
	adminID, _ := c.Locals(middleware.CleContextUtilisateurID).(string)
	dossierID := c.Params("id")

	if err := h.adminKycUseCase.ApprouverDossier(c.Context(), adminID, dossierID); err != nil {
		return mapErreurDomaine(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Rejeter gère POST /api/v1/backoffice/kyc/dossiers/:id/rejeter.
//
//	@Summary		Rejette un dossier KYC
//	@Description	L'utilisateur reste au Tier 1 ; le motif lui est communiqué, il pourra resoumettre
//	@Tags			backoffice-kyc
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path	string							true	"ID du dossier"
//	@Param			rejet		body	dto.RejeterDossierRequestDTO	true	"Motif du rejet"
//	@Success		204
//	@Failure		400	{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		401	{object}	dto.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	dto.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	dto.ErreurDTO	"dossier introuvable"
//	@Failure		422	{object}	dto.ErreurDTO	"dossier déjà traité"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers/{id}/rejeter [post]
func (h *AdminKycHandler) Rejeter(c *fiber.Ctx) error {
	adminID, _ := c.Locals(middleware.CleContextUtilisateurID).(string)
	dossierID := c.Params("id")

	var req dto.RejeterDossierRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.adminKycUseCase.RejeterDossier(c.Context(), adminID, dossierID, req.Motif); err != nil {
		return mapErreurDomaine(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}
