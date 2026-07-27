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

type auditLogRepoFake struct {
	entrees []*domain.AuditLog
}

func (r *auditLogRepoFake) Create(_ context.Context, entry *domain.AuditLog) error {
	r.entrees = append(r.entrees, entry)
	return nil
}

func setupAdminKycService() (input.AdminKycUseCase, *utilisateurRepoFake, *dossierKycRepoFake, *auditLogRepoFake) {
	utilisateurs := nouveauUtilisateurRepoFake()
	dossiers := nouveauDossierKycRepoFake()
	auditLog := &auditLogRepoFake{}
	service := application.NewAdminKycService(utilisateurs, dossiers, auditLog, txManagerFake{})
	return service, utilisateurs, dossiers, auditLog
}

// utilisateurTier1Fake crée un utilisateur au Tier 1 avec un dossier en
// attente, comme le ferait KycService.Inscrire + DemanderTier2.
func utilisateurTier1AvecDossier(t *testing.T, utilisateurs *utilisateurRepoFake, dossiers *dossierKycRepoFake) (*domain.Utilisateur, *domain.DossierKyc) {
	t.Helper()

	u, err := domain.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, u.ValiderKycTier1())
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	d, err := domain.NouveauDossierKyc(u.ID)
	require.NoError(t, err)
	require.NoError(t, dossiers.Create(context.Background(), d))

	return u, d
}

func TestAdminKycService_ListerDossiersEnAttente(t *testing.T) {
	service, utilisateurs, dossiers, _ := setupAdminKycService()
	utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	liste, err := service.ListerDossiersEnAttente(context.Background())
	require.NoError(t, err)
	assert.Len(t, liste, 1)
}

func TestAdminKycService_ApprouverDossier(t *testing.T) {
	service, utilisateurs, dossiers, auditLog := setupAdminKycService()
	u, d := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	require.NoError(t, service.ApprouverDossier(context.Background(), "admin-1", d.ID))

	utilisateurMaj, err := utilisateurs.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.KycTier2, utilisateurMaj.KycTier)

	dossierMaj, err := dossiers.FindByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatutDossierApprouve, dossierMaj.Statut)

	require.Len(t, auditLog.entrees, 1)
	assert.Equal(t, "admin-1", auditLog.entrees[0].AdminID)
	assert.Equal(t, "kyc_tier2_approuve", auditLog.entrees[0].Action)
}

func TestAdminKycService_RejeterDossier(t *testing.T) {
	service, utilisateurs, dossiers, auditLog := setupAdminKycService()
	u, d := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	require.NoError(t, service.RejeterDossier(context.Background(), "admin-1", d.ID, "pièce illisible"))

	utilisateurInchange, err := utilisateurs.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.KycTier1, utilisateurInchange.KycTier, "un rejet ne change pas le tier de l'utilisateur")

	dossierMaj, err := dossiers.FindByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatutDossierRejete, dossierMaj.Statut)
	assert.Equal(t, "pièce illisible", dossierMaj.MotifRejet)

	require.Len(t, auditLog.entrees, 1)
	assert.Equal(t, "kyc_tier2_rejete", auditLog.entrees[0].Action)
}

func TestAdminKycService_ApprouverDossier_Introuvable(t *testing.T) {
	service, _, _, _ := setupAdminKycService()

	err := service.ApprouverDossier(context.Background(), "admin-1", "dossier-inconnu")
	assert.ErrorIs(t, err, domain.ErrDossierKycIntrouvable)
}
