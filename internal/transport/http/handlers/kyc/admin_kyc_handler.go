package kyc

import (
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	inputkyc "raycard/internal/core/ports/input/kyc"
	kycdto "raycard/internal/transport/http/dto/kyc"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type AdminKycHandler struct {
	adminKycUseCase inputkyc.AdminKycUseCase
	validate        *validator.Validate
}

func NewAdminKycHandler(adminKycUseCase inputkyc.AdminKycUseCase, validate *validator.Validate) *AdminKycHandler {
	return &AdminKycHandler{adminKycUseCase: adminKycUseCase, validate: validate}
}

// ListerDossiersEnAttente gère GET /api/v1/backoffice/kyc/dossiers.
//
//	@Summary		Liste des dossiers KYC en attente
//	@Tags			backoffice-kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		kyc.DossierKycDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers [get]
func (h *AdminKycHandler) ListerDossiersEnAttente(c *fiber.Ctx) error {
	dossiers, err := h.adminKycUseCase.ListerDossiersEnAttente(c.Context())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}
	return c.Status(fiber.StatusOK).JSON(kycdto.FromDossiersKyc(dossiers))
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
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"dossier introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"dossier déjà traité"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers/{id}/approuver [post]
func (h *AdminKycHandler) Approuver(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	dossierID := c.Params("id")

	if err := h.adminKycUseCase.ApprouverDossier(c.Context(), adminID, dossierID); err != nil {
		return handlerscommun.MapErreurDomaine(err)
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
//	@Param			rejet		body	kyc.RejeterDossierRequestDTO	true	"Motif du rejet"
//	@Success		204
//	@Failure		400	{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"dossier introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"dossier déjà traité"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers/{id}/rejeter [post]
func (h *AdminKycHandler) Rejeter(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	dossierID := c.Params("id")

	var req kycdto.RejeterDossierRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.adminKycUseCase.RejeterDossier(c.Context(), adminID, dossierID, req.Motif); err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListerDocuments gère GET /api/v1/backoffice/kyc/dossiers/:id/documents.
//
//	@Summary		Liste des documents d'une demande de passage de palier
//	@Description	Renvoie les documents téléversés pour cette demande précise (jamais ceux d'une éventuelle tentative précédente rejetée) et le texte que l'OCR local en a extrait.
//	@Tags			backoffice-kyc
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID du dossier"
//	@Success		200	{array}		kyc.DocumentKycDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/dossiers/{id}/documents [get]
func (h *AdminKycHandler) ListerDocuments(c *fiber.Ctx) error {
	dossierID := c.Params("id")

	documents, err := h.adminKycUseCase.ListerDocuments(c.Context(), dossierID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}
	return c.Status(fiber.StatusOK).JSON(kycdto.FromDocumentsKyc(documents))
}

// RecupererDocument gère GET /api/v1/backoffice/kyc/documents/:id.
//
//	@Summary		Récupération d'un document d'identité
//	@Description	Renvoie le contenu brut de l'image (jamais seulement le texte OCR) : c'est sur cette image que l'administrateur doit fonder sa décision d'approbation ou de rejet.
//	@Tags			backoffice-kyc
//	@Produce		image/jpeg,image/png
//	@Security		BearerAuth
//	@Param			id	path	string	true	"ID du document"
//	@Success		200	{file}		file
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"document introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/kyc/documents/{id} [get]
func (h *AdminKycHandler) RecupererDocument(c *fiber.Ctx) error {
	documentID := c.Params("id")

	_, contenu, err := h.adminKycUseCase.RecupererDocument(c.Context(), documentID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	c.Set(fiber.HeaderContentType, http.DetectContentType(contenu))
	return c.Send(contenu)
}
