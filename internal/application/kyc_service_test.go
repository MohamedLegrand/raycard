package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"raycard/internal/application"
	"raycard/internal/core/domain"
	"raycard/internal/core/ports/input"
)

// --- faux repositories en mémoire, sans dépendance à une vraie base ---

type utilisateurRepoFake struct {
	parEmail     map[string]*domain.Utilisateur
	parTelephone map[string]*domain.Utilisateur
}

func nouveauUtilisateurRepoFake() *utilisateurRepoFake {
	return &utilisateurRepoFake{
		parEmail:     make(map[string]*domain.Utilisateur),
		parTelephone: make(map[string]*domain.Utilisateur),
	}
}

func (r *utilisateurRepoFake) Create(_ context.Context, u *domain.Utilisateur) error {
	r.parEmail[u.Email] = u
	r.parTelephone[u.Telephone] = u
	return nil
}

func (r *utilisateurRepoFake) FindByID(_ context.Context, id string) (*domain.Utilisateur, error) {
	for _, u := range r.parEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrUtilisateurIntrouvable
}

func (r *utilisateurRepoFake) FindByEmail(_ context.Context, email string) (*domain.Utilisateur, error) {
	if u, ok := r.parEmail[email]; ok {
		return u, nil
	}
	return nil, domain.ErrUtilisateurIntrouvable
}

func (r *utilisateurRepoFake) FindByTelephone(_ context.Context, telephone string) (*domain.Utilisateur, error) {
	if u, ok := r.parTelephone[telephone]; ok {
		return u, nil
	}
	return nil, domain.ErrUtilisateurIntrouvable
}

func (r *utilisateurRepoFake) UpdateStatutKyc(_ context.Context, u *domain.Utilisateur) error {
	r.parEmail[u.Email] = u
	return nil
}

type walletRepoFake struct {
	parUtilisateurID map[string]*domain.Wallet
}

func nouveauWalletRepoFake() *walletRepoFake {
	return &walletRepoFake{parUtilisateurID: make(map[string]*domain.Wallet)}
}

func (r *walletRepoFake) Create(_ context.Context, w *domain.Wallet) error {
	r.parUtilisateurID[w.UtilisateurID] = w
	return nil
}

func (r *walletRepoFake) FindByID(_ context.Context, id string) (*domain.Wallet, error) {
	for _, w := range r.parUtilisateurID {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, domain.ErrWalletIntrouvable
}

func (r *walletRepoFake) FindByUtilisateurID(_ context.Context, utilisateurID string) (*domain.Wallet, error) {
	if w, ok := r.parUtilisateurID[utilisateurID]; ok {
		return w, nil
	}
	return nil, domain.ErrWalletIntrouvable
}

func (r *walletRepoFake) UpdateSolde(_ context.Context, w *domain.Wallet) error {
	r.parUtilisateurID[w.UtilisateurID] = w
	return nil
}

type reglesKycRepoFake struct {
	regles map[string]*domain.RegleKyc
}

func nouveauReglesKycRepoFake() *reglesKycRepoFake {
	return &reglesKycRepoFake{
		regles: map[string]*domain.RegleKyc{
			"CI-1": {PaysCode: "CI", Tier: domain.KycTier1, Devise: "XOF", PlafondSoldeCentimes: 200000, PlafondMensuelCentimes: 500000},
		},
	}
}

func (r *reglesKycRepoFake) FindByPaysEtTier(_ context.Context, paysCode string, tier domain.KycTier) (*domain.RegleKyc, error) {
	cle := paysCode + "-" + string(rune('0'+tier))
	if regle, ok := r.regles[cle]; ok {
		return regle, nil
	}
	return nil, domain.ErrPaysNonSupporte
}

// txManagerFake exécute fn directement, sans transaction réelle : les
// repositories fake ci-dessus ne participent à aucune transaction, donc
// il n'y a rien à isoler.
type txManagerFake struct{}

func (txManagerFake) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type dossierKycRepoFake struct {
	parID                   map[string]*domain.DossierKyc
	enAttenteParUtilisateur map[string]*domain.DossierKyc
}

func nouveauDossierKycRepoFake() *dossierKycRepoFake {
	return &dossierKycRepoFake{
		parID:                   make(map[string]*domain.DossierKyc),
		enAttenteParUtilisateur: make(map[string]*domain.DossierKyc),
	}
}

func (r *dossierKycRepoFake) Create(_ context.Context, d *domain.DossierKyc) error {
	r.parID[d.ID] = d
	if d.Statut == domain.StatutDossierEnAttente {
		r.enAttenteParUtilisateur[d.UtilisateurID] = d
	}
	return nil
}

func (r *dossierKycRepoFake) FindByID(_ context.Context, id string) (*domain.DossierKyc, error) {
	if d, ok := r.parID[id]; ok {
		return d, nil
	}
	return nil, domain.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) FindEnAttenteByUtilisateurID(_ context.Context, utilisateurID string) (*domain.DossierKyc, error) {
	if d, ok := r.enAttenteParUtilisateur[utilisateurID]; ok {
		return d, nil
	}
	return nil, domain.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) ListEnAttente(_ context.Context) ([]*domain.DossierKyc, error) {
	var resultat []*domain.DossierKyc
	for _, d := range r.parID {
		if d.Statut == domain.StatutDossierEnAttente {
			resultat = append(resultat, d)
		}
	}
	return resultat, nil
}

func (r *dossierKycRepoFake) Update(_ context.Context, d *domain.DossierKyc) error {
	if _, ok := r.parID[d.ID]; !ok {
		return domain.ErrDossierKycIntrouvable
	}
	r.parID[d.ID] = d
	if d.Statut != domain.StatutDossierEnAttente {
		delete(r.enAttenteParUtilisateur, d.UtilisateurID)
	}
	return nil
}

func setupService() (input.KycUseCase, *utilisateurRepoFake, *walletRepoFake, *dossierKycRepoFake) {
	utilisateurs := nouveauUtilisateurRepoFake()
	wallets := nouveauWalletRepoFake()
	regles := nouveauReglesKycRepoFake()
	dossiers := nouveauDossierKycRepoFake()
	service := application.NewKycService(utilisateurs, wallets, regles, dossiers, txManagerFake{})
	return service, utilisateurs, wallets, dossiers
}

func requeteValide() input.InscriptionRequest {
	return input.InscriptionRequest{
		Nom:        "Koné",
		Prenom:     "Awa",
		Email:      "awa@example.com",
		Telephone:  "+2250700000000",
		PaysCode:   "CI",
		MotDePasse: "motdepasse123",
	}
}

func TestKycService_Inscrire_Succes(t *testing.T) {
	service, utilisateurs, wallets, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	assert.Equal(t, domain.KycTier1, resultat.Utilisateur.KycTier)
	assert.Equal(t, domain.KycStatutVerifie, resultat.Utilisateur.KycStatut)
	assert.NotEqual(t, "motdepasse123", resultat.Utilisateur.MotDePasseHash, "le mot de passe doit être haché")

	assert.Equal(t, "XOF", resultat.Wallet.Devise)
	assert.Equal(t, int64(200000), resultat.Wallet.PlafondSoldeCentimes)
	assert.Equal(t, resultat.Utilisateur.ID, resultat.Wallet.UtilisateurID)

	// Persistance effective via les deux repositories, dans la même
	// "transaction" (fake).
	_, err = utilisateurs.FindByEmail(context.Background(), "awa@example.com")
	require.NoError(t, err)
	_, err = wallets.FindByUtilisateurID(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)
}

func TestKycService_Inscrire_EmailDejaUtilise(t *testing.T) {
	service, _, _, _ := setupService()

	_, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	_, err = service.Inscrire(context.Background(), requeteValide())
	assert.ErrorIs(t, err, domain.ErrEmailDejaUtilise)
}

func TestKycService_Inscrire_TelephoneDejaUtilise(t *testing.T) {
	service, _, _, _ := setupService()

	_, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	req := requeteValide()
	req.Email = "autre@example.com"
	_, err = service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrTelephoneDejaUtilise)
}

func TestKycService_Inscrire_PaysNonSupporte(t *testing.T) {
	service, _, _, _ := setupService()

	req := requeteValide()
	req.Email = "autre@example.com"
	req.Telephone = "+221700000000"
	req.PaysCode = "FR" // pas de règle Tier 1 pour FR dans le fake

	_, err := service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, domain.ErrPaysNonSupporte)
}

func TestKycService_DemanderTier2_Succes(t *testing.T) {
	service, _, _, dossiers := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	dossier, err := service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	assert.Equal(t, domain.StatutDossierEnAttente, dossier.Statut)
	assert.Equal(t, domain.KycTier2, dossier.TierDemande)
	assert.Len(t, dossiers.parID, 1)
}

func TestKycService_DemanderTier2_DejaEnAttente(t *testing.T) {
	service, _, _, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	assert.ErrorIs(t, err, domain.ErrDossierKycDejaEnAttente)
}

func TestKycService_DemanderTier2_PasEncoreTier1(t *testing.T) {
	service, utilisateurs, _, _ := setupService()

	// Un utilisateur qui n'a pas encore de wallet/tier (créé directement,
	// sans passer par Inscrire) ne peut pas demander le Tier 2.
	u, err := domain.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	_, err = service.DemanderTier2(context.Background(), u.ID)
	assert.ErrorIs(t, err, domain.ErrTransitionKycInvalide)
}
