// Package commun fournit de faux repositories en mémoire, partagés par
// les tests des différents modules applicatifs (auth, kyc), pour les
// entités communes (utilisateur, wallet, règles KYC, transactions).
// Ce n'est volontairement pas un fichier _test.go : les tests d'un
// autre package ne peuvent pas importer les symboles d'un fichier de
// test, seulement ceux d'un paquet normal.
package commun

import (
	"context"
	"sort"
	"strings"

	domaincommun "raycard/internal/core/domain/commun"
	outputcommun "raycard/internal/core/ports/output/commun"
)

type UtilisateurRepoFake struct {
	parEmail     map[string]*domaincommun.Utilisateur
	parTelephone map[string]*domaincommun.Utilisateur
	parGoogleID  map[string]*domaincommun.Utilisateur
}

func NewUtilisateurRepoFake() *UtilisateurRepoFake {
	return &UtilisateurRepoFake{
		parEmail:     make(map[string]*domaincommun.Utilisateur),
		parTelephone: make(map[string]*domaincommun.Utilisateur),
		parGoogleID:  make(map[string]*domaincommun.Utilisateur),
	}
}

func (r *UtilisateurRepoFake) Create(_ context.Context, u *domaincommun.Utilisateur) error {
	r.parEmail[u.Email] = u
	r.parTelephone[u.Telephone] = u
	if u.GoogleID != "" {
		r.parGoogleID[u.GoogleID] = u
	}
	return nil
}

func (r *UtilisateurRepoFake) FindByID(_ context.Context, id string) (*domaincommun.Utilisateur, error) {
	for _, u := range r.parEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domaincommun.ErrUtilisateurIntrouvable
}

func (r *UtilisateurRepoFake) FindByEmail(_ context.Context, email string) (*domaincommun.Utilisateur, error) {
	if u, ok := r.parEmail[email]; ok {
		return u, nil
	}
	return nil, domaincommun.ErrUtilisateurIntrouvable
}

func (r *UtilisateurRepoFake) FindByTelephone(_ context.Context, telephone string) (*domaincommun.Utilisateur, error) {
	if u, ok := r.parTelephone[telephone]; ok {
		return u, nil
	}
	return nil, domaincommun.ErrUtilisateurIntrouvable
}

func (r *UtilisateurRepoFake) FindByGoogleID(_ context.Context, googleID string) (*domaincommun.Utilisateur, error) {
	if u, ok := r.parGoogleID[googleID]; ok {
		return u, nil
	}
	return nil, domaincommun.ErrUtilisateurIntrouvable
}

func (r *UtilisateurRepoFake) UpdateStatutKyc(_ context.Context, u *domaincommun.Utilisateur) error {
	r.parEmail[u.Email] = u
	return nil
}

func (r *UtilisateurRepoFake) UpdateMotDePasse(_ context.Context, u *domaincommun.Utilisateur) error {
	r.parEmail[u.Email] = u
	return nil
}

func (r *UtilisateurRepoFake) LierGoogleID(_ context.Context, u *domaincommun.Utilisateur) error {
	r.parEmail[u.Email] = u
	if u.GoogleID != "" {
		r.parGoogleID[u.GoogleID] = u
	}
	return nil
}

func (r *UtilisateurRepoFake) UpdateProfil(_ context.Context, u *domaincommun.Utilisateur) error {
	r.parEmail[u.Email] = u
	return nil
}

// UpdateEmail réindexe par la nouvelle adresse : la clé de parEmail
// doit rester cohérente avec le champ Email de l'objet stocké, sans
// quoi FindByEmail(nouvelEmail) échouerait juste après ce changement.
func (r *UtilisateurRepoFake) UpdateEmail(_ context.Context, u *domaincommun.Utilisateur) error {
	for email, existant := range r.parEmail {
		if existant.ID == u.ID {
			delete(r.parEmail, email)
			break
		}
	}
	r.parEmail[u.Email] = u
	return nil
}

func (r *UtilisateurRepoFake) ListAll(_ context.Context, filtre outputcommun.FiltreUtilisateurs) ([]*domaincommun.Utilisateur, error) {
	var resultat []*domaincommun.Utilisateur
	for _, u := range r.parEmail {
		if filtre.Recherche != "" &&
			!strings.Contains(strings.ToLower(u.Email), strings.ToLower(filtre.Recherche)) &&
			!strings.Contains(u.Telephone, filtre.Recherche) {
			continue
		}
		resultat = append(resultat, u)
	}
	sort.Slice(resultat, func(i, j int) bool { return resultat[i].CreatedAt.After(resultat[j].CreatedAt) })
	return resultat, nil
}

type WalletRepoFake struct {
	parUtilisateurID map[string]*domaincommun.Wallet
}

func NewWalletRepoFake() *WalletRepoFake {
	return &WalletRepoFake{parUtilisateurID: make(map[string]*domaincommun.Wallet)}
}

func (r *WalletRepoFake) Create(_ context.Context, w *domaincommun.Wallet) error {
	r.parUtilisateurID[w.UtilisateurID] = w
	return nil
}

func (r *WalletRepoFake) FindByID(_ context.Context, id string) (*domaincommun.Wallet, error) {
	for _, w := range r.parUtilisateurID {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, domaincommun.ErrWalletIntrouvable
}

func (r *WalletRepoFake) FindByUtilisateurID(_ context.Context, utilisateurID string) (*domaincommun.Wallet, error) {
	if w, ok := r.parUtilisateurID[utilisateurID]; ok {
		return w, nil
	}
	return nil, domaincommun.ErrWalletIntrouvable
}

func (r *WalletRepoFake) UpdateSolde(_ context.Context, w *domaincommun.Wallet) error {
	r.parUtilisateurID[w.UtilisateurID] = w
	return nil
}

type ReglesKycRepoFake struct {
	regles map[string]*domaincommun.RegleKyc
}

func NewReglesKycRepoFake() *ReglesKycRepoFake {
	return &ReglesKycRepoFake{
		regles: map[string]*domaincommun.RegleKyc{
			"CI-1": {PaysCode: "CI", Tier: domaincommun.KycTier1, Devise: "XOF", PlafondSoldeCentimes: 200000, PlafondMensuelCentimes: 500000},
		},
	}
}

func (r *ReglesKycRepoFake) FindByPaysEtTier(_ context.Context, paysCode string, tier domaincommun.KycTier) (*domaincommun.RegleKyc, error) {
	cle := paysCode + "-" + string(rune('0'+tier))
	if regle, ok := r.regles[cle]; ok {
		return regle, nil
	}
	return nil, domaincommun.ErrPaysNonSupporte
}

// StockageFichierFake écrit "quelque part" sans toucher au disque : les
// tests n'ont pas besoin d'un vrai fichier, seulement d'un chemin.
type StockageFichierFake struct{}

func (StockageFichierFake) Sauvegarder(_ context.Context, nomFichier string, _ []byte) (string, error) {
	return "/faux/chemin/" + nomFichier, nil
}

func (StockageFichierFake) Lire(_ context.Context, _ string) ([]byte, error) {
	return []byte("contenu-fichier-factice"), nil
}

// TxManagerFake exécute fn directement, sans transaction réelle : les
// repositories fake ci-dessus ne participent à aucune transaction, donc
// il n'y a rien à isoler.
type TxManagerFake struct{}

func (TxManagerFake) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// EmailEnvoye trace un appel à NotifieurFake.EnvoyerEmail, pour que les
// tests puissent vérifier qu'une notification a bien (ou n'a pas) été
// déclenchée, sans jamais toucher un vrai fournisseur d'email.
type EmailEnvoye struct {
	Destinataire, Sujet, Corps string
}

type NotifieurFake struct {
	EmailsEnvoyes []EmailEnvoye
}

func (n *NotifieurFake) EnvoyerEmail(_ context.Context, destinataire, sujet, corpsHTML string) error {
	n.EmailsEnvoyes = append(n.EmailsEnvoyes, EmailEnvoye{destinataire, sujet, corpsHTML})
	return nil
}

// AuditLogRepoFake trace les entrées d'audit écrites par les services
// back-office (KYC, wallet, carte, admin), pour que les tests puissent
// vérifier qu'une action sensible a bien été tracée.
type AuditLogRepoFake struct {
	Entrees []*domaincommun.AuditLog
}

func (r *AuditLogRepoFake) Create(_ context.Context, entry *domaincommun.AuditLog) error {
	r.Entrees = append(r.Entrees, entry)
	return nil
}

func (r *AuditLogRepoFake) List(_ context.Context, filtre outputcommun.FiltreAuditLog) ([]*domaincommun.AuditLog, error) {
	var resultat []*domaincommun.AuditLog
	for _, e := range r.Entrees {
		if filtre.AdminID != "" && e.AdminID != filtre.AdminID {
			continue
		}
		if filtre.CibleType != "" && e.CibleType != filtre.CibleType {
			continue
		}
		if filtre.CibleID != "" && e.CibleID != filtre.CibleID {
			continue
		}
		resultat = append(resultat, e)
	}
	sort.Slice(resultat, func(i, j int) bool { return resultat[i].CreatedAt.After(resultat[j].CreatedAt) })
	return resultat, nil
}
