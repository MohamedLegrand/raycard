package wallet

import (
	"github.com/gofiber/fiber/v2"

	inputwallet "raycard/internal/core/ports/input/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
	walletdto "raycard/internal/transport/http/dto/wallet"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"
)

type AdminWalletHandler struct {
	adminWalletUseCase inputwallet.AdminWalletUseCase
}

func NewAdminWalletHandler(adminWalletUseCase inputwallet.AdminWalletUseCase) *AdminWalletHandler {
	return &AdminWalletHandler{adminWalletUseCase: adminWalletUseCase}
}

// GelerWallet gère POST /api/v1/backoffice/wallets/:id/gel.
//
//	@Summary		Gel d'un wallet (back-office)
//	@Description	Bloque n'importe quel wallet actif : plus aucun débit ni crédit possible. Tracé dans l'audit log.
//	@Tags			backoffice-wallet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID du wallet"
//	@Success		200	{object}	wallet.WalletDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"le wallet n'est pas actif"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/wallets/{id}/gel [post]
func (h *AdminWalletHandler) GelerWallet(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	walletID := c.Params("id")

	w, err := h.adminWalletUseCase.GelerWalletAdmin(c.Context(), adminID, walletID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(walletdto.FromWallet(w))
}

// DegelerWallet gère POST /api/v1/backoffice/wallets/:id/degel.
//
//	@Summary		Dégel d'un wallet (back-office)
//	@Description	Réactive n'importe quel wallet gelé. Tracé dans l'audit log.
//	@Tags			backoffice-wallet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"ID du wallet"
//	@Success		200	{object}	wallet.WalletDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403	{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		404	{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		422	{object}	commun.ErreurDTO	"le wallet n'est pas gelé"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/wallets/{id}/degel [post]
func (h *AdminWalletHandler) DegelerWallet(c *fiber.Ctx) error {
	adminID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)
	walletID := c.Params("id")

	w, err := h.adminWalletUseCase.DegelerWalletAdmin(c.Context(), adminID, walletID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(walletdto.FromWallet(w))
}

// ListerTransactions gère GET /api/v1/backoffice/transactions.
//
//	@Summary		Liste des transactions (back-office)
//	@Description	Retourne les transactions tous wallets confondus, filtrables par utilisateur, statut et type.
//	@Tags			backoffice-wallet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			utilisateur_id	query		string	false	"Filtre par utilisateur"
//	@Param			statut			query		string	false	"Filtre par statut (en_attente, envoyee, succes, echouee)"
//	@Param			type			query		string	false	"Filtre par type (recharge, retrait, financement_carte, annulation_carte)"
//	@Success		200				{array}		wallet.TransactionDTO
//	@Failure		401				{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		403				{object}	commun.ErreurDTO	"réservé aux administrateurs"
//	@Failure		500				{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/backoffice/transactions [get]
func (h *AdminWalletHandler) ListerTransactions(c *fiber.Ctx) error {
	filtre := outputwallet.FiltreTransactions{
		UtilisateurID: c.Query("utilisateur_id"),
		Statut:        c.Query("statut"),
		Type:          c.Query("type"),
	}

	transactions, err := h.adminWalletUseCase.ListerTransactionsAdmin(c.Context(), filtre)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(walletdto.FromTransactions(transactions))
}
