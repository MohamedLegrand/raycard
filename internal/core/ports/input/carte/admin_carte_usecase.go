package carte

import (
	"context"

	"raycard/internal/core/domain/carte"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// AdminCarteUseCase orchestre les actions back-office sur les cartes,
// sans vérification de propriétaire (voir middleware.RequireAdmin) —
// séparé de CarteUseCase pour ne jamais confondre un accès client et un
// accès administrateur, même si l'implémentation est partagée (voir
// carteService, qui implémente les deux interfaces).
type AdminCarteUseCase interface {
	ListerCartesAdmin(ctx context.Context, filtre outputcarte.FiltreCartes) ([]*carte.Carte, error)

	// GelerCarteAdmin, DegelerCarteAdmin et AnnulerCarteAdmin agissent sur
	// n'importe quelle carte du système (jamais restreint à un
	// propriétaire, contrairement à CarteUseCase) et écrivent une entrée
	// d'audit. Le suffixe Admin évite toute collision de nom avec
	// CarteUseCase sur le type qui implémente les deux interfaces (voir
	// carteService).
	GelerCarteAdmin(ctx context.Context, adminID, carteID string) (*carte.Carte, error)
	DegelerCarteAdmin(ctx context.Context, adminID, carteID string) (*carte.Carte, error)
	AnnulerCarteAdmin(ctx context.Context, adminID, carteID string) (*carte.Carte, error)

	// ObtenirCardWalletAdmin renvoie le solde actuel du portefeuille USD
	// dédié aux cartes — le wallet marchand chez l'agrégateur, distinct de
	// tout wallet utilisateur RAYCARD (voir outputcarte.AgregateurCarte).
	// Jusqu'ici sans aucune visibilité back-office : ce portefeuille n'était
	// touché qu'en réactif, dans le financement automatique borné de
	// CreerCarte/RechargerCarte (voir carteService.creerCarteAvecFinancementAutomatique).
	ObtenirCardWalletAdmin(ctx context.Context) (soldeUSDCentimes int64, err error)

	// AlimenterCardWalletAdmin alimente proactivement le portefeuille USD
	// cartes depuis le wallet XAF principal du marchand chez l'agrégateur,
	// et écrit une entrée d'audit (conversion de fonds réels, action
	// sensible). Permet d'anticiper une utilisation intensive plutôt que de
	// subir des échecs clients en attendant que le financement automatique
	// réagisse carte par carte.
	AlimenterCardWalletAdmin(ctx context.Context, adminID string, montantUSDCentimes int64) (nouveauSoldeUSDCentimes int64, err error)
}
