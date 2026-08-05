package hrpay

import (
	"context"
	"fmt"

	sdk "github.com/hr-skills/hrpay"

	domaincarte "raycard/internal/core/domain/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// CreerCarte implémente carte.AgregateurCarte. Émission synchrone : la
// carte existe déjà (ou l'erreur est définitive) au retour de cet appel,
// contrairement au cash-in/cash-out qui se confirment plus tard par
// webhook.
func (a *Adapter) CreerCarte(ctx context.Context, params outputcarte.CreerCarteParams) (*outputcarte.CreerCarteResultat, error) {
	resp, err := a.client.Cards.Create(ctx, sdk.VirtualCardCreateParams{
		Label:    params.Label,
		Currency: sdk.Currency(params.Devise),
		// Voir le commentaire équivalent dans InitierCashIn : centimes ==
		// unité majeure pour les devises actuellement supportées (XOF, XAF).
		Amount: float64(params.MontantCentimes),
	})
	if err != nil {
		return nil, fmt.Errorf("hrpay création carte: %w", err)
	}

	return &outputcarte.CreerCarteResultat{IDExterne: resp.ID}, nil
}

// ObtenirEtatCarte implémente carte.AgregateurCarte. Utilisé par le job
// de synchronisation (voir carte.CarteUseCase.SynchroniserSoldes), faute
// de webhook de transaction ou de changement de statut côté agrégateur.
func (a *Adapter) ObtenirEtatCarte(ctx context.Context, idExterne string) (int64, domaincarte.StatutCarte, error) {
	resp, err := a.client.Cards.Get(ctx, idExterne)
	if err != nil {
		return 0, "", fmt.Errorf("hrpay lecture état carte: %w", err)
	}
	// Voir le commentaire dans CreerCarte : centimes == unité majeure pour
	// les devises actuellement supportées.
	return int64(resp.Balance), statutDepuisSDK(resp.Status), nil
}

// GelerCarte implémente carte.AgregateurCarte.
func (a *Adapter) GelerCarte(ctx context.Context, idExterne string) error {
	if _, err := a.client.Cards.Freeze(ctx, idExterne); err != nil {
		return fmt.Errorf("hrpay gel carte: %w", err)
	}
	return nil
}

// DegelerCarte implémente carte.AgregateurCarte.
func (a *Adapter) DegelerCarte(ctx context.Context, idExterne string) error {
	if _, err := a.client.Cards.Unfreeze(ctx, idExterne); err != nil {
		return fmt.Errorf("hrpay dégel carte: %w", err)
	}
	return nil
}

// RechargerCarte implémente carte.AgregateurCarte. Topup renvoie la carte
// à jour : on en tire directement le solde résultant plutôt que de le
// recalculer localement.
func (a *Adapter) RechargerCarte(ctx context.Context, idExterne string, montantCentimes int64) (int64, error) {
	resp, err := a.client.Cards.Topup(ctx, idExterne, float64(montantCentimes))
	if err != nil {
		return 0, fmt.Errorf("hrpay recharge carte: %w", err)
	}
	return int64(resp.Balance), nil
}

// AnnulerCarte implémente carte.AgregateurCarte. Cancel renvoie la carte
// telle qu'elle était juste avant destruction : c'est de là que vient le
// solde à rembourser, jamais d'un calcul local qui pourrait être périmé.
func (a *Adapter) AnnulerCarte(ctx context.Context, idExterne string) (int64, error) {
	resp, err := a.client.Cards.Cancel(ctx, idExterne)
	if err != nil {
		return 0, fmt.Errorf("hrpay annulation carte: %w", err)
	}
	return int64(resp.Balance), nil
}

// statutDepuisSDK traduit le statut brut du SDK ("ACTIVE", "FROZEN",
// "CANCELLED" — non typé côté SDK) vers carte.StatutCarte. Tout code
// inconnu retombe sur Active plutôt que de faire disparaître la carte du
// sondage (voir CarteRepository.ListAVerifier).
func statutDepuisSDK(statutSDK string) domaincarte.StatutCarte {
	switch statutSDK {
	case "FROZEN":
		return domaincarte.StatutCarteGelee
	case "CANCELLED":
		return domaincarte.StatutCarteAnnulee
	default:
		return domaincarte.StatutCarteActive
	}
}
