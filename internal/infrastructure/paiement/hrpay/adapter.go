// Package hrpay adapte le SDK officiel HR-Skills Pay
// (github.com/hr-skills/hrpay) au port wallet.AgregateurPaiement. C'est
// la seule couche du projet qui connaît le SDK concret : remplacer
// d'agrégateur un jour ne devrait toucher que ce fichier.
package hrpay

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	sdk "github.com/hr-skills/hrpay"

	domainwallet "raycard/internal/core/domain/wallet"
	outputwallet "raycard/internal/core/ports/output/wallet"
)

type Adapter struct {
	client        *sdk.Client
	webhookSecret string
}

// delaiMinEntreAppels impose un espacement minimal entre deux appels au
// SDK, quel que soit l'appelant (recharge, retrait, sondage de solde de
// carte...). Nécessaire depuis que le sondage de carte tourne toutes les
// 20s : sans ça, plusieurs cartes à vérifier au même instant pourraient
// riper la limite de débit de l'agrégateur (voir hrpay.RateLimitError).
const delaiMinEntreAppels = 200 * time.Millisecond

// NewAdapter construit l'adaptateur. publicKey/secretKey déterminent
// l'environnement (préfixe hrsk_..._test_ = sandbox, hrsk_..._live_ =
// production) — jamais l'URL, qui est la même dans les deux cas.
// webhookSecret sert uniquement à vérifier la signature des webhooks
// entrants (distinct des clés API, fourni séparément par HR-Skills Pay).
func NewAdapter(publicKey, secretKey, webhookSecret string) (*Adapter, error) {
	client, err := sdk.NewClient(
		sdk.WithAPIKeys(publicKey, secretKey),
		sdk.WithThrottle(delaiMinEntreAppels),
	)
	if err != nil {
		return nil, fmt.Errorf("initialisation client hrpay: %w", err)
	}
	return &Adapter{client: client, webhookSecret: webhookSecret}, nil
}

// telephoneSansPlus retire le "+" du format E.164 utilisé partout côté
// RAYCARD : le SDK valide le numéro avec une regexp chiffres uniquement
// (^\d+$) et rejette tout numéro préfixé par "+" avant même d'atteindre
// le réseau.
func telephoneSansPlus(telephone string) string {
	return strings.TrimPrefix(telephone, "+")
}

func (a *Adapter) InitierCashIn(ctx context.Context, params outputwallet.InitierCashInParams) (*outputwallet.InitierCashInResultat, error) {
	resp, err := a.client.CashIn.MobileMoney(ctx, sdk.CashInMobileMoneyParams{
		PhoneNumber: telephoneSansPlus(params.Telephone),
		Operator:    sdk.Operator(params.Operateur),
		Country:     sdk.Country(params.PaysCode),
		Currency:    sdk.Currency(params.Devise),
		// Le SDK exprime les montants en unité majeure. Les devises que
		// RAYCARD supporte aujourd'hui (XOF, XAF) n'ont pas de décimale, donc
		// centimes == unité majeure (voir le commentaire sur commun.Wallet).
		// À revoir si une devise à décimales est ajoutée un jour.
		Amount: float64(params.MontantCentimes),
	})
	if err != nil {
		return nil, fmt.Errorf("hrpay cash-in: %w", err)
	}

	return &outputwallet.InitierCashInResultat{ReferenceExterne: resp.Reference}, nil
}

func (a *Adapter) InitierCashOut(ctx context.Context, params outputwallet.InitierCashOutParams) (*outputwallet.InitierCashOutResultat, error) {
	resp, err := a.client.CashOut.MobileMoney(ctx, sdk.CashOutMobileMoneyParams{
		PhoneNumber: telephoneSansPlus(params.Telephone),
		Operator:    sdk.Operator(params.Operateur),
		Country:     sdk.Country(params.PaysCode),
		Currency:    sdk.Currency(params.Devise),
		Amount:      float64(params.MontantCentimes),
	})
	if err != nil {
		return nil, fmt.Errorf("hrpay cash-out: %w", err)
	}

	return &outputwallet.InitierCashOutResultat{ReferenceExterne: resp.Reference}, nil
}

func (a *Adapter) ConstruireEvenementWebhook(corps []byte, signature string) (*outputwallet.EvenementWebhook, error) {
	evt, err := a.client.Webhooks.ConstructEvent(string(corps), signature, a.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domainwallet.ErrWebhookSignatureInvalide, err)
	}

	// La transaction associée vit dans evt.Data quel que soit le type
	// d'évènement — on la décode systématiquement ; c'est le service
	// applicatif (wallet.TraiterWebhook) qui décide quoi faire de chaque
	// type, y compris ignorer ceux qu'il ne gère pas encore (ex:
	// payment.hold, payment.refunded).
	var tx sdk.Transaction
	if err := json.Unmarshal(evt.Data, &tx); err != nil {
		return nil, fmt.Errorf("décodage évènement webhook: %w", err)
	}

	return &outputwallet.EvenementWebhook{
		Type:             outputwallet.TypeEvenementWebhook(evt.Type),
		ReferenceExterne: tx.Reference,
		FraisCentimes:    int64(tx.Fees),
	}, nil
}
