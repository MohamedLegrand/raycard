package kyc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appkyc "raycard/internal/application/kyc"
	"raycard/internal/core/domain/commun"
	"raycard/internal/core/domain/kyc"
	inputkyc "raycard/internal/core/ports/input/kyc"
	testcommun "raycard/test/application/commun"
)

// --- faux repository en mémoire, propre au module kyc ---

type dossierKycRepoFake struct {
	parID                   map[string]*kyc.DossierKyc
	enAttenteParUtilisateur map[string]*kyc.DossierKyc
}

func nouveauDossierKycRepoFake() *dossierKycRepoFake {
	return &dossierKycRepoFake{
		parID:                   make(map[string]*kyc.DossierKyc),
		enAttenteParUtilisateur: make(map[string]*kyc.DossierKyc),
	}
}

func (r *dossierKycRepoFake) Create(_ context.Context, d *kyc.DossierKyc) error {
	r.parID[d.ID] = d
	if d.Statut == kyc.StatutDossierEnAttente {
		r.enAttenteParUtilisateur[d.UtilisateurID] = d
	}
	return nil
}

func (r *dossierKycRepoFake) FindByID(_ context.Context, id string) (*kyc.DossierKyc, error) {
	if d, ok := r.parID[id]; ok {
		return d, nil
	}
	return nil, kyc.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) FindEnAttenteByUtilisateurID(_ context.Context, utilisateurID string) (*kyc.DossierKyc, error) {
	if d, ok := r.enAttenteParUtilisateur[utilisateurID]; ok {
		return d, nil
	}
	return nil, kyc.ErrDossierKycIntrouvable
}

func (r *dossierKycRepoFake) ListEnAttente(_ context.Context) ([]*kyc.DossierKyc, error) {
	var resultat []*kyc.DossierKyc
	for _, d := range r.parID {
		if d.Statut == kyc.StatutDossierEnAttente {
			resultat = append(resultat, d)
		}
	}
	return resultat, nil
}

func (r *dossierKycRepoFake) Update(_ context.Context, d *kyc.DossierKyc) error {
	if _, ok := r.parID[d.ID]; !ok {
		return kyc.ErrDossierKycIntrouvable
	}
	r.parID[d.ID] = d
	if d.Statut != kyc.StatutDossierEnAttente {
		delete(r.enAttenteParUtilisateur, d.UtilisateurID)
	}
	return nil
}

func setupService() (inputkyc.KycUseCase, *testcommun.UtilisateurRepoFake, *testcommun.WalletRepoFake, *dossierKycRepoFake) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	regles := testcommun.NewReglesKycRepoFake()
	dossiers := nouveauDossierKycRepoFake()
	service := appkyc.NewKycService(utilisateurs, wallets, regles, dossiers, testcommun.TxManagerFake{})
	return service, utilisateurs, wallets, dossiers
}

func requeteValide() inputkyc.InscriptionRequest {
	return inputkyc.InscriptionRequest{
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

	assert.Equal(t, commun.KycTier1, resultat.Utilisateur.KycTier)
	assert.Equal(t, commun.KycStatutVerifie, resultat.Utilisateur.KycStatut)
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
	assert.ErrorIs(t, err, commun.ErrEmailDejaUtilise)
}

func TestKycService_Inscrire_TelephoneDejaUtilise(t *testing.T) {
	service, _, _, _ := setupService()

	_, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	req := requeteValide()
	req.Email = "autre@example.com"
	_, err = service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, commun.ErrTelephoneDejaUtilise)
}

func TestKycService_Inscrire_PaysNonSupporte(t *testing.T) {
	service, _, _, _ := setupService()

	req := requeteValide()
	req.Email = "autre@example.com"
	req.Telephone = "+221700000000"
	req.PaysCode = "FR" // pas de règle Tier 1 pour FR dans le fake

	_, err := service.Inscrire(context.Background(), req)
	assert.ErrorIs(t, err, commun.ErrPaysNonSupporte)
}

func TestKycService_DemanderTier2_Succes(t *testing.T) {
	service, _, _, dossiers := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	dossier, err := service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	assert.Equal(t, kyc.StatutDossierEnAttente, dossier.Statut)
	assert.Equal(t, commun.KycTier2, dossier.TierDemande)
	assert.Len(t, dossiers.parID, 1)
}

func TestKycService_DemanderTier2_DejaEnAttente(t *testing.T) {
	service, _, _, _ := setupService()

	resultat, err := service.Inscrire(context.Background(), requeteValide())
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	require.NoError(t, err)

	_, err = service.DemanderTier2(context.Background(), resultat.Utilisateur.ID)
	assert.ErrorIs(t, err, kyc.ErrDossierKycDejaEnAttente)
}

func TestKycService_DemanderTier2_PasEncoreTier1(t *testing.T) {
	service, utilisateurs, _, _ := setupService()

	// Un utilisateur qui n'a pas encore de wallet/tier (créé directement,
	// sans passer par Inscrire) ne peut pas demander le Tier 2.
	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	_, err = service.DemanderTier2(context.Background(), u.ID)
	assert.ErrorIs(t, err, commun.ErrTransitionKycInvalide)
}
