// Package carte contient les handlers HTTP des cartes virtuelles :
// émission et consultation.
package carte

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	inputcarte "raycard/internal/core/ports/input/carte"
	cartedto "raycard/internal/transport/http/dto/carte"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type CarteHandler struct {
	carteUseCase inputcarte.CarteUseCase
	validate     *validator.Validate
}

func NewCarteHandler(carteUseCase inputcarte.CarteUseCase, validate *validator.Validate) *CarteHandler {
	return &CarteHandler{carteUseCase: carteUseCase, validate: validate}
}

// SoumettrePorteurCarte gère POST /api/v1/cartes/porteur (route protégée).
//
//	@Summary		Enrôlement comme porteur de carte
//	@Description	Soumet le KYC porteur de carte exigé par l'agrégateur avant de pouvoir émettre une carte (voir GET /cartes/porteur pour suivre le statut). Réutilise les pièces d'identité du dossier KYC Tier 2 déjà approuvé — jamais redemandées ici.
//	@Tags			"1. Client - Carte"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			porteur	body		carte.SoumettrePorteurCarteRequestDTO	true	"Profil du porteur"
//	@Success		201		{object}	carte.CardCustomerDTO
//	@Failure		400		{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		409		{object}	commun.ErreurDTO	"un dossier de porteur de carte existe déjà"
//	@Failure		422		{object}	commun.ErreurDTO	"palier KYC insuffisant, ou aucun document d'identité approuvé disponible"
//	@Failure		500		{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/porteur [post]
func (h *CarteHandler) SoumettrePorteurCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req cartedto.SoumettrePorteurCarteRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	porteur, err := h.carteUseCase.SoumettrePorteurCarte(c.Context(), utilisateurID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(cartedto.FromCardCustomer(porteur))
}

// ObtenirStatutPorteurCarte gère GET /api/v1/cartes/porteur (route protégée).
//
//	@Summary		Statut du porteur de carte
//	@Description	Retourne le dossier de porteur de carte de l'utilisateur authentifié (404 si jamais soumis — voir POST /cartes/porteur).
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	carte.CardCustomerDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"aucun dossier de porteur de carte"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/porteur [get]
func (h *CarteHandler) ObtenirStatutPorteurCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	porteur, err := h.carteUseCase.ObtenirStatutPorteurCarte(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCardCustomer(porteur))
}

// CreerCarte gère POST /api/v1/cartes (route protégée).
//
//	@Summary		Émission d'une carte virtuelle
//	@Description	Débite immédiatement le solde disponible du wallet puis émet la carte auprès de l'agrégateur (Tier 2 KYC requis). Le PAN et le CVV ne sont jamais renvoyés ni stockés.
//	@Tags			"1. Client - Carte"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			carte	body		carte.CreerCarteRequestDTO	true	"Nom de la carte et montant à charger"
//	@Success		201		{object}	carte.CarteDTO
//	@Failure		400		{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404		{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		409		{object}	commun.ErreurDTO	"une opération wallet est déjà en cours"
//	@Failure		422		{object}	commun.ErreurDTO	"wallet gelé, solde insuffisant, montant invalide ou palier KYC insuffisant"
//	@Failure		500		{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes [post]
func (h *CarteHandler) CreerCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req cartedto.CreerCarteRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	carteCreee, err := h.carteUseCase.CreerCarte(c.Context(), utilisateurID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusCreated).JSON(cartedto.FromCarte(carteCreee))
}

// ListerCartes gère GET /api/v1/cartes (route protégée).
//
//	@Summary		Liste des cartes virtuelles
//	@Description	Retourne les cartes virtuelles de l'utilisateur authentifié.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes [get]
func (h *CarteHandler) ListerCartes(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	cartes, err := h.carteUseCase.ListerCartes(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCartes(cartes))
}

// ObtenirCarte gère GET /api/v1/cartes/:id (route protégée).
//
//	@Summary		Détail d'une carte virtuelle
//	@Description	Retourne une carte précise de l'utilisateur authentifié.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id} [get]
func (h *CarteHandler) ObtenirCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteObtenue, err := h.carteUseCase.ObtenirCarte(c.Context(), utilisateurID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteObtenue))
}

// GelerCarte gère POST /api/v1/cartes/:id/gel (route protégée).
//
//	@Summary		Gel d'une carte virtuelle
//	@Description	Bloque une carte active : plus aucune dépense possible tant qu'elle n'est pas dégelée.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"la carte n'est pas active"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id}/gel [post]
func (h *CarteHandler) GelerCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteMiseAJour, err := h.carteUseCase.GelerCarte(c.Context(), utilisateurID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteMiseAJour))
}

// AnnulerCarte gère POST /api/v1/cartes/:id/annuler (route protégée).
//
//	@Summary		Annulation d'une carte virtuelle
//	@Description	Détruit définitivement une carte active ou gelée et rembourse au wallet ce qu'il restait dessus.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"carte ou wallet introuvable"
//	@Failure		409	{object}	commun.ErreurDTO	"une opération wallet est déjà en cours"
//	@Failure		422	{object}	commun.ErreurDTO	"carte déjà annulée, ou wallet gelé"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id}/annuler [post]
func (h *CarteHandler) AnnulerCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteAnnulee, err := h.carteUseCase.AnnulerCarte(c.Context(), utilisateurID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteAnnulee))
}

// DegelerCarte gère POST /api/v1/cartes/:id/degel (route protégée).
//
//	@Summary		Dégel d'une carte virtuelle
//	@Description	Réactive une carte gelée.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{object}	carte.CarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"la carte n'est pas gelée"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id}/degel [post]
func (h *CarteHandler) DegelerCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	carteMiseAJour, err := h.carteUseCase.DegelerCarte(c.Context(), utilisateurID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteMiseAJour))
}

// RechargerCarte gère POST /api/v1/cartes/:id/topup (route protégée).
//
//	@Summary		Recharge d'une carte virtuelle existante
//	@Description	Débite immédiatement le solde disponible du wallet puis ajoute les fonds à une carte active existante.
//	@Tags			"1. Client - Carte"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id			path		string							true	"ID de la carte"
//	@Param			recharge	body		carte.RechargerCarteRequestDTO	true	"Montant à ajouter"
//	@Success		200			{object}	carte.CarteDTO
//	@Failure		400			{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401			{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404			{object}	commun.ErreurDTO	"carte ou wallet introuvable"
//	@Failure		409			{object}	commun.ErreurDTO	"une opération wallet est déjà en cours"
//	@Failure		422			{object}	commun.ErreurDTO	"carte non active, wallet gelé, solde insuffisant ou montant invalide"
//	@Failure		500			{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id}/topup [post]
func (h *CarteHandler) RechargerCarte(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	var req cartedto.RechargerCarteRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	carteMiseAJour, err := h.carteUseCase.RechargerCarte(c.Context(), utilisateurID, carteID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromCarte(carteMiseAJour))
}

// ListerDepenses gère GET /api/v1/cartes/:id/depenses (route protégée).
//
//	@Summary		Dépenses détectées sur une carte
//	@Description	Retourne les dépenses détectées par rapprochement périodique de solde — jamais en temps réel, faute de webhook de transaction carte côté agrégateur.
//	@Tags			"1. Client - Carte"
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID de la carte"
//	@Success		200	{array}		carte.DepenseCarteDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"carte introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/cartes/{id}/depenses [get]
func (h *CarteHandler) ListerDepenses(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	carteID := c.Params("id")

	depenses, err := h.carteUseCase.ListerDepenses(c.Context(), utilisateurID, carteID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(cartedto.FromDepenses(depenses))
}
