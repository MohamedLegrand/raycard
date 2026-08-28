// Package admin contient les handlers HTTP back-office transverses aux
// modules : utilisateurs et audit log.
package admin

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	domaincommun "raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	outputcommun "raycard/internal/core/ports/output/commun"
	admindto "raycard/internal/transport/http/dto/admin"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type AdminHandler struct {
	adminUseCase inputadmin.AdminUseCase
	validate     *validator.Validate
}

func NewAdminHandler(adminUseCase inputadmin.AdminUseCase, validate *validator.Validate) *AdminHandler {
	return &AdminHandler{adminUseCase: adminUseCase, validate: validate}
}

// ListerUtilisateurs gère GET /api/v1/backoffice/utilisateurs.
//
//	@Summary		Liste des utilisateurs (back-office)
//	@Description	Retourne les utilisateurs, filtrables par recherche partielle sur l'email ou le téléphone.
//	@Tags			"2. Admin - Utilisateurs"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			q	query		string	false	"Recherche (email ou téléphone)"
//	@Success		200	{array}		admin.UtilisateurDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/utilisateurs [get]
func (h *AdminHandler) ListerUtilisateurs(c *fiber.Ctx) error {
	filtre := outputcommun.FiltreUtilisateurs{Recherche: c.Query("q")}

	utilisateurs, err := h.adminUseCase.ListerUtilisateurs(c.Context(), filtre)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(admindto.FromUtilisateurs(utilisateurs))
}

// ObtenirUtilisateur gère GET /api/v1/backoffice/utilisateurs/:id.
//
//	@Summary		Fiche complète d'un utilisateur (back-office)
//	@Description	Retourne le profil, le wallet et les cartes d'un utilisateur.
//	@Tags			"2. Admin - Utilisateurs"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de l'utilisateur"
//	@Success		200	{object}	admin.UtilisateurDetailDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"utilisateur introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/utilisateurs/{id} [get]
func (h *AdminHandler) ObtenirUtilisateur(c *fiber.Ctx) error {
	utilisateurID := c.Params("id")

	detail, err := h.adminUseCase.ObtenirUtilisateur(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(admindto.FromUtilisateurDetail(detail))
}

// ListerAuditLogs gère GET /api/v1/backoffice/audit-logs.
//
//	@Summary		Historique d'audit (back-office)
//	@Description	Retourne les actions administrateur sensibles, filtrables par administrateur, type de cible et cible.
//	@Tags			"2. Admin - Utilisateurs"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			admin_id	query		string	false	"Filtre par administrateur"
//	@Param			cible_type	query		string	false	"Filtre par type de cible (utilisateur, wallet, carte)"
//	@Param			cible_id	query		string	false	"Filtre par cible"
//	@Success		200			{array}		admin.AuditLogDTO
//	@Failure		401			{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403			{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500			{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/audit-logs [get]
func (h *AdminHandler) ListerAuditLogs(c *fiber.Ctx) error {
	filtre := outputcommun.FiltreAuditLog{
		AdminID:   c.Query("admin_id"),
		CibleType: c.Query("cible_type"),
		CibleID:   c.Query("cible_id"),
	}

	entrees, err := h.adminUseCase.ListerAuditLogs(c.Context(), filtre)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(admindto.FromAuditLogs(entrees))
}

// ChangerRole gère PUT /api/v1/backoffice/utilisateurs/:id/role (route
// réservée au super_admin, voir middleware.RequireSuperAdmin).
//
//	@Summary		Change le rôle d'un utilisateur (super-admin uniquement)
//	@Description	Élève un client en admin/super_admin, ou rétrograde un admin — jamais son propre compte (évite un verrouillage accidentel). Tracé dans l'audit log.
//	@Tags			"2. Admin - Utilisateurs"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"ID de l'utilisateur"
//	@Param			role	body		admin.ChangerRoleRequestDTO	true	"Nouveau rôle"
//	@Success		200		{object}	admin.UtilisateurDTO
//	@Failure		400		{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403		{object}	commun.ErreurDTO	"réservé aux super-administrateurs"
//	@Failure		404		{object}	commun.ErreurDTO	"utilisateur introuvable"
//	@Failure		422		{object}	commun.ErreurDTO	"tentative de modifier son propre rôle"
//	@Router			/backoffice/utilisateurs/{id}/role [put]
func (h *AdminHandler) ChangerRole(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	utilisateurID := c.Params("id")

	var req admindto.ChangerRoleRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	utilisateur, err := h.adminUseCase.ChangerRoleUtilisateur(c.Context(), adminID, utilisateurID, domaincommun.RoleUtilisateur(req.Role))
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(admindto.FromUtilisateur(utilisateur))
}

// CreerAdministrateur gère POST /api/v1/backoffice/utilisateurs (route
// réservée au super_admin, voir middleware.RequireSuperAdmin).
//
//	@Summary		Crée un administrateur (super-admin uniquement)
//	@Description	Crée directement un compte admin ou super_admin, sans passer par l'inscription cliente — pour on-boarder un membre de l'équipe qui n'a jamais utilisé l'app mobile. Tracé dans l'audit log.
//	@Tags			"2. Admin - Utilisateurs"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			administrateur	body		admin.CreerAdministrateurRequestDTO	true	"Nouvel administrateur"
//	@Success		201				{object}	admin.UtilisateurDTO
//	@Failure		400				{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401				{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403				{object}	commun.ErreurDTO	"réservé aux super-administrateurs"
//	@Failure		409				{object}	commun.ErreurDTO	"email ou téléphone déjà utilisé"
//	@Router			/backoffice/utilisateurs [post]
func (h *AdminHandler) CreerAdministrateur(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req admindto.CreerAdministrateurRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	admin, err := h.adminUseCase.CreerAdministrateur(c.Context(), adminID, inputadmin.CreerAdministrateurRequest{
		Nom:        req.Nom,
		Prenom:     req.Prenom,
		Email:      req.Email,
		Telephone:  req.Telephone,
		PaysCode:   req.PaysCode,
		MotDePasse: req.MotDePasse,
		Role:       domaincommun.RoleUtilisateur(req.Role),
	})
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(admindto.FromUtilisateur(admin))
}
