// Package wallet contient les handlers HTTP du wallet : consultation du
// solde, recharge Mobile Money, et réception des webhooks de
// confirmation de l'agrégateur de paiement.
package wallet

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	inputwallet "raycard/internal/core/ports/input/wallet"
	handlerscommun "raycard/internal/transport/http/handlers/commun"
	authmw "raycard/internal/transport/http/middleware/auth"

	walletdto "raycard/internal/transport/http/dto/wallet"
)

type WalletHandler struct {
	walletUseCase inputwallet.WalletUseCase
	validate      *validator.Validate
}

func NewWalletHandler(walletUseCase inputwallet.WalletUseCase, validate *validator.Validate) *WalletHandler {
	return &WalletHandler{walletUseCase: walletUseCase, validate: validate}
}

// ObtenirWallet gère GET /api/v1/wallet (route protégée).
//
//	@Summary		Consultation du wallet
//	@Description	Retourne le wallet de l'utilisateur authentifié, avec le détail disponible/en attente.
//	@Tags			"1. Client - Wallet"
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	wallet.WalletDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/wallet [get]
func (h *WalletHandler) ObtenirWallet(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	w, err := h.walletUseCase.ObtenirWallet(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(walletdto.FromWallet(w))
}

// ListerTransactions gère GET /api/v1/wallet/transactions (route protégée).
//
//	@Summary		Historique des transactions du wallet
//	@Description	Retourne toutes les transactions (recharge, retrait, financement de carte...) du wallet de l'utilisateur authentifié, les plus récentes d'abord.
//	@Tags			"1. Client - Wallet"
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{array}		wallet.TransactionDTO
//	@Failure		401	{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404	{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/wallet/transactions [get]
func (h *WalletHandler) ListerTransactions(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	transactions, err := h.walletUseCase.ListerTransactions(c.Context(), utilisateurID)
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusOK).JSON(walletdto.FromTransactions(transactions))
}

// InitierRecharge gère POST /api/v1/wallet/topup (route protégée).
//
//	@Summary		Recharge du wallet par Mobile Money
//	@Description	Initie une collecte Mobile Money auprès de l'agrégateur de paiement. Le wallet n'est crédité (en attente, puis disponible sous 48h) qu'à la confirmation du paiement, jamais à cet appel.
//	@Tags			"1. Client - Wallet"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			recharge	body		wallet.InitierRechargeRequestDTO	true	"Opérateur, téléphone et montant"
//	@Success		202			{object}	wallet.TransactionDTO
//	@Failure		400			{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401			{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404			{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		409			{object}	commun.ErreurDTO	"une recharge est déjà en cours"
//	@Failure		422			{object}	commun.ErreurDTO	"wallet gelé, montant invalide, plafond dépassé ou opérateur non supporté"
//	@Failure		500			{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/wallet/topup [post]
func (h *WalletHandler) InitierRecharge(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req walletdto.InitierRechargeRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	transaction, err := h.walletUseCase.InitierRecharge(c.Context(), utilisateurID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusAccepted).JSON(walletdto.FromTransaction(transaction))
}

// InitierRetrait gère POST /api/v1/wallet/cashout (route protégée).
//
//	@Summary		Retrait du wallet par Mobile Money
//	@Description	Débite immédiatement le solde disponible puis déclenche le décaissement Mobile Money auprès de l'agrégateur. Contrairement à la recharge, le débit précède la confirmation.
//	@Tags			"1. Client - Wallet"
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			retrait	body		wallet.InitierRetraitRequestDTO	true	"Opérateur, téléphone et montant"
//	@Success		202		{object}	wallet.TransactionDTO
//	@Failure		400		{object}	commun.ErreurDTO	"corps de requête invalide"
//	@Failure		401		{object}	commun.ErreurDTO	"non authentifié"
//	@Failure		404		{object}	commun.ErreurDTO	"wallet introuvable"
//	@Failure		409		{object}	commun.ErreurDTO	"un retrait est déjà en cours"
//	@Failure		422		{object}	commun.ErreurDTO	"wallet gelé, solde insuffisant, montant invalide ou opérateur non supporté"
//	@Failure		500		{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/wallet/cashout [post]
func (h *WalletHandler) InitierRetrait(c *fiber.Ctx) error {
	utilisateurID, _ := c.Locals(authmw.CleContextUtilisateurID).(string)

	var req walletdto.InitierRetraitRequestDTO
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "corps de requête invalide")
	}
	if err := h.validate.Struct(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	transaction, err := h.walletUseCase.InitierRetrait(c.Context(), utilisateurID, req.ToUseCaseRequest())
	if err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.Status(fiber.StatusAccepted).JSON(walletdto.FromTransaction(transaction))
}

// WebhookHrPay gère POST /api/v1/webhooks/hrpay. Non authentifiée par
// JWT : l'authenticité vient exclusivement de la signature HMAC vérifiée
// par le use case (voir wallet.AgregateurPaiement.ConstruireEvenementWebhook).
//
//	@Summary		Webhook HR-Skills Pay
//	@Description	Reçoit les notifications de confirmation de paiement de l'agrégateur. Vérifie la signature HMAC avant tout traitement.
//	@Tags			"3. Système - Webhooks"
//	@Accept			json
//	@Produce		json
//	@Param			X-Hub-Signature	header	string	true	"Signature HMAC-SHA256 du corps brut"
//	@Success		200
//	@Failure		401	{object}	commun.ErreurDTO	"signature invalide"
//	@Failure		404	{object}	commun.ErreurDTO	"transaction introuvable"
//	@Failure		500	{object}	commun.ErreurDTO	"erreur interne"
//	@Router			/webhooks/hrpay [post]
func (h *WalletHandler) WebhookHrPay(c *fiber.Ctx) error {
	signature := c.Get("X-Hub-Signature")

	if err := h.walletUseCase.TraiterWebhook(c.Context(), c.Body(), signature); err != nil {
		return handlerscommun.MapErreurDomaine(err)
	}

	return c.SendStatus(fiber.StatusOK)
}
