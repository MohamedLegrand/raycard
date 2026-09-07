// Ce fichier appelle directement l'API HTTP HR-Skills Pay pour les routes
// que la version vendorisée du SDK (github.com/hr-skills/hrpay v0.1.0) ne
// connaît pas encore : porteurs de carte (card-customers) et portefeuille
// USD dédié aux cartes (card-wallet) — voir la documentation marchande
// "Cartes Virtuelles" (§9), absente du SDK à ce jour.
//
// Réutilise l'authentification du SDK (client.Auth.GetToken, exporté) au
// lieu de redupliquer l'échange de transaction-token : mêmes garanties de
// cache/renouvellement (TTL 45 min) que le reste des appels agrégateur.
package hrpay

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	sdk "github.com/hr-skills/hrpay"

	domaincarte "raycard/internal/core/domain/carte"
	domaincommun "raycard/internal/core/domain/commun"
	outputcarte "raycard/internal/core/ports/output/carte"
)

// erreurAPI reflète {"error"/"code", "message", ...} — la documentation
// marchande utilise tantôt "error" (ex: transaction-token), tantôt "code"
// (ex: erreurs de validation) comme clé du code machine ; on lit les deux
// plutôt que de parier sur l'une des deux formes.
type erreurAPI struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e erreurAPI) codeMachine() string {
	if e.Error != "" {
		return e.Error
	}
	return e.Code
}

// requeteAuthentifiee exécute un appel JSON authentifié (Bearer clé
// publique + X-Transaction-Token, comme le reste de la plateforme — voir
// le commentaire de paquet) vers l'API HR-Skills Pay. corps peut être nil
// (GET). idempotent doit être vrai pour tout POST qui déplace des fonds
// (voir HeaderIdempotencyKey documenté) — jamais pour un GET.
//
// soldeCarteWalletSiInsuffisant distingue les deux sens différents que
// prend le code "insufficient_balance" selon l'endpoint (voir la
// documentation de l'agrégateur, §9) : sur /virtual-cards (création,
// topup) il signifie "portefeuille USD cartes insuffisant" — vrai ici,
// traduit en domaincarte.ErrCardWalletInsuffisant. Sur /card-wallet/fund
// en revanche, le même code signifie l'inverse : "wallet XAF principal
// du marchand insuffisant" — faux ici, laissé en erreur générique pour
// ne jamais déclencher à tort le financement automatique borné (voir
// carteService.creerCarteAvecFinancementAutomatique, qui réagit
// spécifiquement au sentinel).
func (a *Adapter) requeteAuthentifiee(ctx context.Context, methode, chemin string, corps any, idempotent, soldeCarteWalletSiInsuffisant bool) ([]byte, error) {
	jeton, err := a.client.Auth.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtention transaction token: %w", err)
	}

	var lecteurCorps io.Reader
	if corps != nil {
		b, err := json.Marshal(corps)
		if err != nil {
			return nil, fmt.Errorf("encodage requête: %w", err)
		}
		lecteurCorps = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, methode, sdk.DefaultBaseURL+chemin, lecteurCorps)
	if err != nil {
		return nil, fmt.Errorf("construction requête: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+a.publicKey)
	req.Header.Set(sdk.HeaderTransactionToken, jeton)
	if corps != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotent {
		// commun.NewID() génère un UUID v4 en crypto/rand (voir ce
		// fichier) : même garantie qu'un uuid.New() dédié, sans ajouter de
		// dépendance directe supplémentaire au module.
		req.Header.Set(sdk.HeaderIdempotencyKey, domaincommun.NewID())
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("appel hrpay %s %s: %w", methode, chemin, err)
	}
	defer func() { _ = resp.Body.Close() }()

	corpsReponse, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lecture réponse hrpay: %w", err)
	}

	if resp.StatusCode >= 400 {
		var e erreurAPI
		_ = json.Unmarshal(corpsReponse, &e)
		msg := e.Message
		if msg == "" {
			msg = string(corpsReponse)
		}
		// insufficient_balance n'est traduit en sentinel que là où il
		// signifie vraiment "portefeuille USD cartes insuffisant" (voir le
		// commentaire de soldeCarteWalletSiInsuffisant ci-dessus) : le
		// service applicatif s'en sert pour déclencher un financement
		// automatique borné (voir carteService.CreerCarte). Ailleurs
		// (notamment /card-wallet/fund, où le même code signifie l'inverse),
		// reste une erreur générique.
		if soldeCarteWalletSiInsuffisant && e.codeMachine() == "insufficient_balance" {
			return nil, fmt.Errorf("%w: %s", domaincarte.ErrCardWalletInsuffisant, msg)
		}
		return nil, fmt.Errorf("hrpay %s %s: %d %s (%s)", methode, chemin, resp.StatusCode, msg, e.codeMachine())
	}

	return corpsReponse, nil
}

// dataURI encode un document en data-URI base64, format attendu par les
// champs document* de l'agrégateur (voir SoumettreCardCustomer).
func dataURI(mimeType string, contenu []byte) string {
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(contenu))
}

type cardCustomerReponse struct {
	Customer struct {
		ID string `json:"id"`
	} `json:"customer"`
}

// SoumettreCardCustomer implémente carte.AgregateurCarte. Toujours
// PENDING_REVIEW en retour — jamais d'enrôlement synchrone, la revue est
// humaine côté HR-Skills Pay (voir carte.NouveauCardCustomer).
func (a *Adapter) SoumettreCardCustomer(ctx context.Context, p outputcarte.SoumettreCardCustomerParams) (*outputcarte.SoumettreCardCustomerResultat, error) {
	corps := map[string]any{
		"first_name":            p.Prenom,
		"last_name":             p.Nom,
		"email":                 p.Email,
		"country":               p.PaysNomComplet,
		"country_iso_code":      p.PaysCodeISO,
		"country_phone_code":    p.IndicatifPays,
		"phone_number":          p.TelephoneLocal,
		"street":                p.Rue,
		"city":                  p.Ville,
		"state":                 p.Region,
		"postal_code":           p.CodePostal,
		"identification_number": p.NumeroIdentification,
		"id_document_type":      p.TypeDocument,
		"date_of_birth":         p.DateNaissance,
		"id_document_front":     dataURI(p.DocumentRectoMimeType, p.DocumentRecto),
		"id_document_back":      dataURI(p.DocumentVersoMimeType, p.DocumentVerso),
	}

	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/card-customers", corps, false, false)
	if err != nil {
		return nil, fmt.Errorf("hrpay soumission porteur de carte: %w", err)
	}

	var reponse cardCustomerReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return nil, fmt.Errorf("décodage réponse porteur de carte: %w", err)
	}
	return &outputcarte.SoumettreCardCustomerResultat{IDExterne: reponse.Customer.ID}, nil
}

type cardWalletReponse struct {
	Wallet struct {
		Balance float64 `json:"balance"`
	} `json:"wallet"`
}

// ObtenirCardWallet implémente carte.AgregateurCarte. Le solde est
// renvoyé en dollars par l'agrégateur (pas en centimes) : converti ici,
// jamais côté domaine (voir CreerCarteParams.MontantUSDCentimes).
func (a *Adapter) ObtenirCardWallet(ctx context.Context) (int64, error) {
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodGet, "/api/v1/card-wallet", nil, false, false)
	if err != nil {
		return 0, fmt.Errorf("hrpay lecture portefeuille cartes: %w", err)
	}
	var reponse cardWalletReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, fmt.Errorf("décodage portefeuille cartes: %w", err)
	}
	return dollarsVersCentimes(reponse.Wallet.Balance), nil
}

type quoteReponse struct {
	Quote struct {
		AmountUSD float64 `json:"amount_usd"`
	} `json:"quote"`
}

// CoterConversion implémente carte.AgregateurCarte.
func (a *Adapter) CoterConversion(ctx context.Context, montantXAFCentimes int64) (int64, error) {
	chemin := fmt.Sprintf("/api/v1/card-wallet/quote?amount_source=%d&source_currency=XAF&direction=fund", montantXAFCentimes)
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodGet, chemin, nil, false, false)
	if err != nil {
		return 0, fmt.Errorf("hrpay cotation conversion: %w", err)
	}
	var reponse quoteReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, fmt.Errorf("décodage cotation: %w", err)
	}
	return dollarsVersCentimes(reponse.Quote.AmountUSD), nil
}

type fundReponse struct {
	UsdBalance float64 `json:"usd_balance"`
}

// AlimenterCardWallet implémente carte.AgregateurCarte.
func (a *Adapter) AlimenterCardWallet(ctx context.Context, montantUSDCentimes int64) (int64, error) {
	corps := map[string]any{"amount_usd": centimesVersDollars(montantUSDCentimes)}
	corpsReponse, err := a.requeteAuthentifiee(ctx, http.MethodPost, "/api/v1/card-wallet/fund", corps, true, false)
	if err != nil {
		return 0, fmt.Errorf("hrpay alimentation portefeuille cartes: %w", err)
	}
	var reponse fundReponse
	if err := json.Unmarshal(corpsReponse, &reponse); err != nil {
		return 0, fmt.Errorf("décodage alimentation portefeuille: %w", err)
	}
	return dollarsVersCentimes(reponse.UsdBalance), nil
}

func dollarsVersCentimes(montant float64) int64 {
	return int64(montant*100 + 0.5) // arrondi au plus proche, jamais tronqué vers le bas
}

func centimesVersDollars(centimes int64) float64 {
	return float64(centimes) / 100
}
