package auth

import (
	"io"

	"github.com/gofiber/fiber/v2"

	authdto "raycard/internal/transport/http/dto/auth"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

// ObtenirProfil gère GET /api/v1/auth/profil (route protégée).
//
//	@Summary		Consultation du profil
//	@Description	Retourne le profil de l'utilisateur authentifié (nom, prénom, email, téléphone, palier KYC...) sans le modifier.
//	@Tags			"1. Client - Auth"
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	auth.UtilisateurDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"utilisateur introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil [get]
func (h *AuthHandler) ObtenirProfil(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	utilisateur, err := h.authUseCase.ObtenirProfil(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(authdto.FromUtilisateur(utilisateur))
}

// ModifierProfil gère PUT /api/v1/auth/profil (route protégée).
//
//	@Summary		Modification du profil
//	@Description	Met à jour le nom et le prénom de l'utilisateur authentifié.
//	@Tags			"1. Client - Auth"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			profil	body		auth.ModifierProfilRequestDTO	true	"Nouveau nom et prénom"
//	@Success		200		{object}	auth.UtilisateurDTO
//	@Failure		400		{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		500		{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil [put]
func (h *AuthHandler) ModifierProfil(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req authdto.ModifierProfilRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	utilisateur, err := h.authUseCase.ModifierProfil(c.Context(), utilisateurID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(authdto.FromUtilisateur(utilisateur))
}

// ModifierPhotoProfil gère POST /api/v1/auth/profil/photo (route
// protégée).
//
//	@Summary		Modification de la photo de profil
//	@Description	Stocke la nouvelle photo de profil. Formats acceptés : JPEG, PNG.
//	@Tags			"1. Client - Auth"
//	@Accept			multipart/form-data
//	@Produce		json
//	@Security		BearerAuth
//	@Param			photo	formData	file	true	"Nouvelle photo de profil"
//	@Success		200		{object}	auth.UtilisateurDTO
//	@Failure		400		{object}	commun.ErreurDTO	"fichier manquant ou illisible"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		422		{object}	commun.ErreurDTO	"format de fichier non supporté"
//	@Failure		500		{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil/photo [post]
func (h *AuthHandler) ModifierPhotoProfil(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	fichier, err := c.FormFile("photo")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "fichier 'photo' manquant")
	}

	f, err := fichier.Open()
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "impossible de lire le fichier")
	}
	defer func() { _ = f.Close() }()

	contenu, err := io.ReadAll(f)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "impossible de lire le fichier")
	}

	utilisateur, err := h.authUseCase.ModifierPhotoProfil(c.Context(), utilisateurID, fichier.Filename, contenu)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(authdto.FromUtilisateur(utilisateur))
}

// ObtenirPhotoProfil gère GET /api/v1/auth/profil/photo (route
// protégée).
//
//	@Summary		Consultation de la photo de profil
//	@Description	Retourne le contenu brut (image) de la photo de profil de l'utilisateur authentifié.
//	@Tags			"1. Client - Auth"
//	@Produce		image/jpeg
//	@Security		BearerAuth
//	@Success		200
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"aucune photo de profil"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil/photo [get]
func (h *AuthHandler) ObtenirPhotoProfil(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	contenu, contentType, err := h.authUseCase.ObtenirPhotoProfil(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	c.Set(fiber.HeaderContentType, contentType)
	return c.Send(contenu)
}

// ChangerMotDePasse gère POST /api/v1/auth/profil/mot-de-passe (route
// protégée).
//
//	@Summary		Changement de mot de passe
//	@Description	Change le mot de passe de l'utilisateur authentifié, après vérification du mot de passe actuel, et révoque toutes les autres sessions actives.
//	@Tags			"1. Client - Auth"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			motDePasse	body	auth.ChangerMotDePasseRequestDTO	true	"Mot de passe actuel et nouveau mot de passe"
//	@Success		204
//	@Failure		400	{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié, ou mot de passe actuel incorrect"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil/mot-de-passe [post]
func (h *AuthHandler) ChangerMotDePasse(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req authdto.ChangerMotDePasseRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.authUseCase.ChangerMotDePasse(c.Context(), utilisateurID, req.MotDePasseActuel, req.NouveauMotDePasse); err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// DemanderChangementEmail gère POST /api/v1/auth/profil/email (route
// protégée).
//
//	@Summary		Demande de changement d'email
//	@Description	Envoie un code de confirmation au NOUVEL email. Le changement ne prend effet qu'après confirmation (voir /auth/profil/email/confirmer).
//	@Tags			"1. Client - Auth"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			demande	body	auth.DemanderChangementEmailRequestDTO	true	"Nouvelle adresse email"
//	@Success		204
//	@Failure		400	{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		409	{object}	commun.ErreurDTO	"email déjà utilisé"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil/email [post]
func (h *AuthHandler) DemanderChangementEmail(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req authdto.DemanderChangementEmailRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if err := h.authUseCase.DemanderChangementEmail(c.Context(), utilisateurID, req.NouvelEmail); err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ConfirmerChangementEmail gère POST /api/v1/auth/profil/email/confirmer.
//
//	@Summary		Confirmation du changement d'email
//	@Description	Applique le changement d'email si le code reçu au nouvel email est valide. Notifie l'ancienne adresse du changement.
//	@Tags			"1. Client - Auth"
//	@Accept			json
//	@Produce		json
//	@Param			confirmation	body		auth.ConfirmerChangementEmailRequestDTO	true	"Code reçu au nouvel email"
//	@Success		200				{object}	auth.UtilisateurDTO
//	@Failure		400				{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401				{object}	commun.ErreurDTO	"code invalide, expiré ou déjà utilisé"
//	@Failure		409				{object}	commun.ErreurDTO	"email déjà utilisé"
//	@Failure		500				{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/auth/profil/email/confirmer [post]
func (h *AuthHandler) ConfirmerChangementEmail(c *fiber.Ctx) error {
	var req authdto.ConfirmerChangementEmailRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	utilisateur, err := h.authUseCase.ConfirmerChangementEmail(c.Context(), req.Code)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(authdto.FromUtilisateur(utilisateur))
}
