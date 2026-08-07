package carte

import (
	"github.com/gofiber/fiber/v2"

	inputcarte "raycard/internal/core/ports/input/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
	cartedto "raycard/internal/transport/http/dto/carte"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type AdminCarteHandler struct {
	adminCarteUseCase inputcarte.AdminCarteUseCase
}

func NewAdminCarteHandler(adminCarteUseCase inputcarte.AdminCarteUseCase) *AdminCarteHandler {
	return &AdminCarteHandler{adminCarteUseCase: adminCarteUseCase}
}

// ListerCartes gère GET /api/v1/backoffice/cartes.
//
//	@Summary		Liste des cartes (back-office)
//	@Description	Retourne les cartes tous utilisateurs confondus, filtrables par utilisateur et par statut.
//	@Tags			"2. Admin - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			utilisateur_id	query		string	false	"Filtre par utilisateur"
//	@Param			statut			query		string	false	"Filtre par statut (active, gelee, annulee)"
//	@Success		200				{array}		carte.CarteDTO
//	@Failure		401				{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403				{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500				{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/cartes [get]
func (h *AdminCarteHandler) ListerCartes(c *fiber.Ctx) error {
	filtre := outputcarte.FiltreCartes{
		UtilisateurID: c.Query("utilisateur_id"),
		Statut:        c.Query("statut"),
	}

	cartes, err := h.adminCarteUseCase.ListerCartesAdmin(c.Context(), filtre)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCartes(cartes))
}

// GelerCarte gère POST /api/v1/backoffice/cartes/:id/gel.
//
//	@Summary		Gel d'une carte (back-office)
//	@Description	Bloque n'importe quelle carte active, sans vérification de propriétaire. Tracé dans l'audit log.
//	@Tags			"2. Admin - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"la carte n'est pas active"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/cartes/{id}/gel [post]
func (h *AdminCarteHandler) GelerCarte(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteMiseAJour, err := h.adminCarteUseCase.GelerCarteAdmin(c.Context(), adminID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteMiseAJour))
}

// DegelerCarte gère POST /api/v1/backoffice/cartes/:id/degel.
//
//	@Summary		Dégel d'une carte (back-office)
//	@Description	Réactive n'importe quelle carte gelée, sans vérification de propriétaire. Tracé dans l'audit log.
//	@Tags			"2. Admin - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"la carte n'est pas gelée"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/cartes/{id}/degel [post]
func (h *AdminCarteHandler) DegelerCarte(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteMiseAJour, err := h.adminCarteUseCase.DegelerCarteAdmin(c.Context(), adminID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteMiseAJour))
}

// AnnulerCarte gère POST /api/v1/backoffice/cartes/:id/annuler.
//
//	@Summary		Annulation d'une carte (back-office)
//	@Description	Détruit définitivement n'importe quelle carte active ou gelée et rembourse au wallet ce qu'il restait dessus. Tracé dans l'audit log.
//	@Tags			"2. Admin - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"carte ou wallet introuvable"
//	@Failure		409	{object}	commun.ErreurDTO	"une opération wallet est déjà en cours"
//	@Failure		422	{object}	commun.ErreurDTO	"carte déjà annulée, ou wallet gelé"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/cartes/{id}/annuler [post]
func (h *AdminCarteHandler) AnnulerCarte(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteAnnulee, err := h.adminCarteUseCase.AnnulerCarteAdmin(c.Context(), adminID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteAnnulee))
}
