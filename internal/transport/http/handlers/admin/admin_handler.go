// Package admin contient les handlers HTTP back-office transverses aux
// modules : utilisateurs et audit log.
package admin

import (
	"github.com/gofiber/fiber/v2"

	inputadmin "raycard/internal/core/ports/input/admin"
	outputcommun "raycard/internal/core/ports/output/commun"
	admindto "raycard/internal/transport/http/dto/admin"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
)

type AdminHandler struct {
	adminUseCase inputadmin.AdminUseCase
}

func NewAdminHandler(adminUseCase inputadmin.AdminUseCase) *AdminHandler {
	return &AdminHandler{adminUseCase: adminUseCase}
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
