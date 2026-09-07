package hrpay

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	domaincarte "raycard/internal/core/domain/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// carteBrute reflète l'objet "card" renvoyé par l'agrégateur à la
// création et à la lecture — sérialisé côté HR-Skills Pay sans tags JSON
// (PascalCase : "ID", "Status", "Balance"...), à la différence du reste
// de cette API qui est en snake_case partout ailleurs. Les tags
// minuscules ci-dessous suffisent quand même : encoding/json retombe sur
// une correspondance insensible à la casse quand aucune clé ne matche
// exactement le tag — vérifié, pas une supposition.
type carteBrute struct {
	ID     string  `json:"id"`
	Status string  `json:"status"`
	Balance float64 `json:"balance"`
}

type creerCarteReponse struct {
	Card carteBrute `json:"card"`
	// Réponse 202 ambiguë (échec réseau/5xx côté Cartevo) : ni card_id ni
	// card ne sont garantis présents en même temps, voir CreerCarte.
	CardID  string `json:"card_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreerCarte implémente carte.AgregateurCarte. Émission "synchrone dans
// le cas nominal" : une réponse 202 signale une confirmation différée
// (voir outputcarte.CreerCarteResultat.StatutProvisoire), jamais une
// erreur à traiter comme définitive.
func (a *Adapter) CreerCarte(ctx context.Context, params outputcarte.CreerCarteParams) (*outputcarte.CreerCarteResultat, error) {
	corps := map[string]any{
		"customer_id":  params.CustomerIDExterne,
		"brand":        params.Brand,
		"name_on_card": params.NomSurCarte,
		"label":        params.Label,
		"amount":       centimesVersDollars(params.MontantUSDCentimes),
	}

	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/virtual-cards", corps, true, true)
	if err != nil {
		return nil, fmt.Errorf("hrpay création carte: %w", err)
	}

	var reponse creerCarteReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return nil, fmt.Errorf("décodage réponse création carte: %w", err)
	}

	if reponse.Card.ID != "" {
		return &outputcarte.CreerCarteResultat{
			IDExterne:        reponse.Card.ID,
			SoldeUSDCentimes: dollarsVersCentimes(reponse.Card.Balance),
		}, nil
	}
	// Réponse 202 : carte pas encore confirmée, voir le commentaire de
	// CreerCarteResultat.StatutProvisoire — l'appelant décide quoi en
	// faire (jamais de nouvelle tentative aveugle ici, l'agrégateur n'a
	// pas d'idempotence serveur sur cette route selon sa documentation).
	if reponse.CardID != "" {
		return &outputcarte.CreerCarteResultat{IDExterne: reponse.CardID, StatutProvisoire: true}, nil
	}
	return nil, fmt.Errorf("hrpay création carte: réponse inattendue (%s)", string(corpsReponse))
}

// ObtenirEtatCarte implémente carte.AgregateurCarte. Utilisé par le job
// de synchronisation (voir carte.CarteUseCase.SynchroniserSoldes), faute
// de webhook de transaction ou de changement de statut côté agrégateur.
func (a *Adapter) ObtenirEtatCarte(ctx context.Context, idExterne string) (int64, domaincarte.StatutCarte, error) {
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodGet, "/api/v1/virtual-cards/"+idExterne, nil, false, false)
	if err != nil {
		return 0, "", fmt.Errorf("hrpay lecture état carte: %w", err)
	}
	var reponse struct {
		Card carteBrute `json:"card"`
	}
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, "", fmt.Errorf("décodage état carte: %w", err)
	}
	return dollarsVersCentimes(reponse.Card.Balance), statutDepuisSDK(reponse.Card.Status), nil
}

// GelerCarte implémente carte.AgregateurCarte.
func (a *Adapter) GelerCarte(ctx context.Context, idExterne string) error {
	if _, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/virtual-cards/"+idExterne+"/freeze", nil, false, false); err != nil {
		return fmt.Errorf("hrpay gel carte: %w", err)
	}
	return nil
}

// DegelerCarte implémente carte.AgregateurCarte.
func (a *Adapter) DegelerCarte(ctx context.Context, idExterne string) error {
	if _, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/virtual-cards/"+idExterne+"/unfreeze", nil, false, false); err != nil {
		return fmt.Errorf("hrpay dégel carte: %w", err)
	}
	return nil
}

type topupReponse struct {
	Balance float64 `json:"balance"`
}

// RechargerCarte implémente carte.AgregateurCarte. Topup renvoie le
// nouveau solde de la carte : on en tire directement le solde résultant
// plutôt que de le recalculer localement.
func (a *Adapter) RechargerCarte(ctx context.Context, idExterne string, montantUSDCentimes int64) (int64, error) {
	corps := map[string]any{"amount": centimesVersDollars(montantUSDCentimes)}
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/virtual-cards/"+idExterne+"/topup", corps, true, true)
	if err != nil {
		return 0, fmt.Errorf("hrpay recharge carte: %w", err)
	}
	var reponse topupReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, fmt.Errorf("décodage recharge carte: %w", err)
	}
	return dollarsVersCentimes(reponse.Balance), nil
}

type cancelReponse struct {
	Refunded float64 `json:"refunded"`
}

// AnnulerCarte implémente carte.AgregateurCarte. "cancel" et "terminate"
// déclenchent la même action côté agrégateur (voir sa documentation) —
// route "cancel" retenue ici, jamais les deux.
func (a *Adapter) AnnulerCarte(ctx context.Context, idExterne string) (int64, error) {
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/virtual-cards/"+idExterne+"/cancel", nil, false, false)
	if err != nil {
		return 0, fmt.Errorf("hrpay annulation carte: %w", err)
	}
	var reponse cancelReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, fmt.Errorf("décodage annulation carte: %w", err)
	}
	return dollarsVersCentimes(reponse.Refunded), nil
}

// statutDepuisSDK traduit le statut brut de l'agrégateur ("ACTIVE",
// "FROZEN", "SUSPENDED", "TERMINATED", "PENDING", "FAILED" — voir sa
// documentation) vers carte.StatutCarte. SUSPENDED (rare, contrôle
// fraude amont) et PENDING/FAILED (émission non aboutie, ne devrait
// jamais être observé ici — une carte PENDING/FAILED n'a pas encore
// d'IDExterne stable) retombent sur Active plutôt que de faire
// disparaître la carte du sondage (voir CarteRepository.ListAVerifier) ;
// seuls FROZEN et CANCELLED/TERMINATED ont une traduction dédiée.
func statutDepuisSDK(statutSDK string) domaincarte.StatutCarte {
	switch statutSDK {
	case "FROZEN":
		return domaincarte.StatutCarteGelee
	case "CANCELLED", "TERMINATED":
		return domaincarte.StatutCarteAnnulee
	default:
		return domaincarte.StatutCarteActive
	}
}
