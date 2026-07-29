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
//	@Summary		Connexion (étape 1/2)
//	@Description	Vérifie email + mot de passe et déclenche la 2FA obligatoire : envoie un code par email et renvoie un ticket à présenter avec ce code sur /auth/connexion/verifier-code. Aucun token de session n'est émis ici.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			connexion	body		dto.ConnexionRequestDTO	true	"Identifiants"
//	@Success		200			{object}	dto.ConnexionResponseDTO
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

	return c.Status(fiber.StatusOK).JSON(dto.FromConnexionResultat(resultat))
}

// VerifierCode2FA gère POST /api/v1/auth/connexion/verifier-code.
//
//	@Summary		Connexion (étape 2/2) — vérification du code
//	@Description	Échange le ticket obtenu à l'étape 1 et le code reçu par email contre une session complète (access + refresh token). 5 tentatives maximum ; au-delà, le ticket est définitivement invalidé.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			verification	body		dto.VerifierCode2FARequestDTO	true	"Ticket et code reçu par email"
//	@Success		200				{object}	dto.SessionResponseDTO
//	@Failure		400				{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		401				{object}	dto.ErreurDTO	"ticket ou code invalide, expiré, ou tentatives épuisées"
//	@Failure		500				{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/connexion/verifier-code [post]
func (h *AuthHandler) VerifierCode2FA(c *fiber.Ctx) error {
	var req dto.VerifierCode2FARequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.authUseCase.VerifierCode2FA(c.Context(), req.Ticket, req.Code)
	if err != nil {
		return mapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(dto.FromSessionResultat(resultat))
}

// ConnexionGoogle gère POST /api/v1/auth/connexion-google.
//
//	@Summary		Connexion via Google
//	@Description	Vérifie l'ID token Google et émet directement une session (2FA sautée pour ce chemin). Crée le compte automatiquement s'il n'existe pas encore, ou lie ce compte Google à un compte existant de même email (si Google confirme l'email vérifié).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			connexion	body		dto.ConnexionGoogleRequestDTO	true	"ID token Google, téléphone et pays (utilisés seulement à la création)"
//	@Success		200			{object}	dto.SessionResponseDTO
//	@Failure		400			{object}	dto.ErreurDTO	"corps de requête invalide, ou téléphone/pays manquants pour une première connexion"
//	@Failure		401			{object}	dto.ErreurDTO	"id token invalide, ou email non vérifié pour lier un compte existant"
//	@Failure		500			{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/connexion-google [post]
func (h *AuthHandler) ConnexionGoogle(c *fiber.Ctx) error {
	var req dto.ConnexionGoogleRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	resultat, err := h.authUseCase.ConnexionGoogle(c.Context(), req.ToUseCaseRequest())
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

// DemanderReinitialisation gère POST /api/v1/auth/mot-de-passe-oublie.
//
//	@Summary		Demande de réinitialisation de mot de passe
//	@Description	Envoie un code par email si un compte existe pour cet email. Répond toujours le même succès générique, que le compte existe ou non (évite l'énumération de comptes).
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			demande	body	dto.DemanderReinitialisationRequestDTO	true	"Email du compte"
//	@Success		204
//	@Failure		400	{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/mot-de-passe-oublie [post]
func (h *AuthHandler) DemanderReinitialisation(c *fiber.Ctx) error {
	var req dto.DemanderReinitialisationRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.authUseCase.DemanderReinitialisation(c.Context(), req.Email); err != nil {
		return mapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Reinitialiser gère POST /api/v1/auth/reinitialiser-mot-de-passe.
//
//	@Summary		Réinitialisation du mot de passe
//	@Description	Change le mot de passe si le code est valide et révoque toutes les sessions actives de l'utilisateur
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			reinitialisation	body	dto.ReinitialiserRequestDTO	true	"Code reçu par email et nouveau mot de passe"
//	@Success		204
//	@Failure		400	{object}	dto.ErreurDTO	"corps de requête invalide"
//	@Failure		401	{object}	dto.ErreurDTO	"code invalide, expiré ou déjà utilisé"
//	@Failure		500	{object}	dto.ErreurDTO	"erreur interne"
//	@Router			/auth/reinitialiser-mot-de-passe [post]
func (h *AuthHandler) Reinitialiser(c *fiber.Ctx) error {
	var req dto.ReinitialiserRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.authUseCase.Reinitialiser(c.Context(), req.Token, req.NouveauMotDePasse); err != nil {
		return mapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}
