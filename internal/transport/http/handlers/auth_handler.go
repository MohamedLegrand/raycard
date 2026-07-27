package handlers

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"raycard/internal/core/ports/input"
	"raycard/internal/transport/http/dto"
)

type AuthHandler struct {
	authUseCase input.AuthUseCase
	validate    *validator.Validate
}

func NewAuthHandler(authUseCase input.AuthUseCase, validate *validator.Validate) *AuthHandler {
	return &AuthHandler{authUseCase: authUseCase, validate: validate}
}

// Connexion gère POST /api/v1/auth/connexion.
//
//	@Summary		Connexion
//	@Description	Authentifie un utilisateur et émet un access token (15 min) et un refresh token (30 jours)
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			connexion	body		dto.ConnexionRequestDTO	true	"Identifiants"
//	@Success		200			{object}	dto.SessionResponseDTO
//	@Failure		400			{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		401			{object}	dto.ErreurDTO	"identifiants invalides"
//	@Failure		500			{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/connexion [post]
func (h *AuthHandler) Connexion(c *fiber.Ctx) error {
	var req dto.ConnexionRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.authUseCase.Connexion(c.Context(), req.ToUseCaseRequest())
	if err != nil {
		return mapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.FromSessionResultat(resultat))
}

// Rafraichir gère POST /api/v1/auth/rafraichir.
//
//	@Summary		Rafraîchissement de session
//	@Description	Échange un refresh token valide contre une nouvelle paire access/refresh token (rotation)
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			rafraichir	body		dto.RafraichirRequestDTO	true	"Refresh token"
//	@Success		200			{object}	dto.SessionResponseDTO
//	@Failure		400			{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		401			{object}	dto.ErreurDTO	"refresh token invalide, expiré ou révoqué"
//	@Failure		500			{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/rafraichir [post]
func (h *AuthHandler) Rafraichir(c *fiber.Ctx) error {
	var req dto.RafraichirRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.authUseCase.RafraichirToken(c.Context(), req.RefreshToken)
	if err != nil {
		return mapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.FromSessionResultat(resultat))
}

// Deconnexion gère POST /api/v1/auth/deconnexion.
//
//	@Summary		Déconnexion
//	@Description	Révoque le refresh token fourni (déconnexion idempotente : ne renvoie pas d'erreur si déjà invalide)
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			deconnexion	body	dto.RafraichirRequestDTO	true	"Refresh token à révoquer"
//	@Success		204
//	@Failure		400	{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/deconnexion [post]
func (h *AuthHandler) Deconnexion(c *fiber.Ctx) error {
	var req dto.RafraichirRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.authUseCase.Deconnexion(c.Context(), req.RefreshToken); err != nil {
		return mapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
