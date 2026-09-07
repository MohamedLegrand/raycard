package carte

import "errors"

// Erreurs spécifiques aux cartes virtuelles.
var (
	ErrCarteIntrouvable        = errors.New("carte introuvable")
	ErrKycTierInsuffisant      = errors.New("palier KYC insuffisant pour émettre une carte (Tier 2 requis)")
	ErrEmissionEchouee         = errors.New("émission de la carte échouée")
	ErrRechargeEchouee         = errors.New("recharge de la carte échouée")
	ErrAnnulationEchouee       = errors.New("annulation de la carte échouée")
	ErrRetraitEchoue           = errors.New("retrait de la carte échoué")
	ErrSoldeCarteInsuffisant   = errors.New("solde de la carte insuffisant pour ce retrait")
	ErrTransitionCarteInvalide = errors.New("transition de statut de carte invalide")

	// Erreurs du porteur de carte (card-customer) — le KYC distinct exigé
	// par l'agrégateur avant de pouvoir émettre une carte pour un
	// utilisateur (voir CardCustomer).
	ErrCardCustomerIntrouvable  = errors.New("porteur de carte introuvable")
	ErrCardCustomerDejaSoumis   = errors.New("un dossier de porteur de carte existe déjà pour cet utilisateur")
	ErrCardCustomerNonEnrole    = errors.New("le porteur de carte n'est pas encore validé par l'agrégateur")
	ErrCardCustomerRejete       = errors.New("le porteur de carte a été rejeté")
	ErrDocumentsIdentiteAbsents = errors.New("aucun document d'identité (recto/verso) trouvé sur le dossier KYC approuvé")

	// ErrCardWalletInsuffisant signale un solde insuffisant sur le
	// portefeuille USD dédié aux cartes (distinct du wallet XAF de
	// l'utilisateur, déjà vérifié avant tout appel agrégateur) — détecté
	// spécifiquement par l'adaptateur HR-Skills Pay (code machine
	// "insufficient_balance") pour permettre un financement automatique
	// borné (voir carteService.CreerCarte).
	ErrCardWalletInsuffisant = errors.New("solde du portefeuille cartes insuffisant")
)
