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

func setupAdminKycService() (inputkyc.AdminKycUseCase, *testcommun.UtilisateurRepoFake, *dossierKycRepoFake, *documentKycRepoFake, *testcommun.AuditLogRepoFake) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	dossiers := nouveauDossierKycRepoFake()
	documents := nouveauDocumentKycRepoFake()
	auditLog := &testcommun.AuditLogRepoFake{}
	service := appkyc.NewAdminKycService(utilisateurs, dossiers, documents, testcommun.StockageFichierFake{}, auditLog, testcommun.TxManagerFake{})
	return service, utilisateurs, dossiers, documents, auditLog
}

// utilisateurTier1AvecDossier crée un utilisateur au Tier 1 avec un
// dossier en attente, comme le ferait KycService.Inscrire + DemanderTier2.
func utilisateurTier1AvecDossier(t *testing.T, utilisateurs *testcommun.UtilisateurRepoFake, dossiers *dossierKycRepoFake) (*commun.Utilisateur, *kyc.DossierKyc) {
	t.Helper()

	u, err := commun.NouveauUtilisateur("Koné", "Awa", "awa@example.com", "+2250700000000", "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, u.ValiderKycTier1())
	require.NoError(t, utilisateurs.Create(context.Background(), u))

	d, err := kyc.NouveauDossierKyc(u.ID)
	require.NoError(t, err)
	require.NoError(t, dossiers.Create(context.Background(), d))

	return u, d
}

func TestAdminKycService_ListerDossiersEnAttente(t *testing.T) {
	service, utilisateurs, dossiers, _, _ := setupAdminKycService()
	utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	liste, err := service.ListerDossiersEnAttente(context.Background())
	require.NoError(t, err)
	assert.Len(t, liste, 1)
}

func TestAdminKycService_ListerTousDossiers_InclutTraites(t *testing.T) {
	service, utilisateurs, dossiers, _, _ := setupAdminKycService()
	_, dEnAttente := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)
	_, dApprouve := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)
	require.NoError(t, service.ApprouverDossier(context.Background(), "admin-1", dApprouve.ID))

	// Contrairement à ListerDossiersEnAttente, doit inclure le dossier
	// déjà traité.
	toutes, err := service.ListerTousDossiers(context.Background())
	require.NoError(t, err)
	assert.Len(t, toutes, 2)

	enAttente, err := service.ListerDossiersEnAttente(context.Background())
	require.NoError(t, err)
	require.Len(t, enAttente, 1)
	assert.Equal(t, dEnAttente.ID, enAttente[0].ID)
}

func TestAdminKycService_ApprouverDossier(t *testing.T) {
	service, utilisateurs, dossiers, _, auditLog := setupAdminKycService()
	u, d := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	require.NoError(t, service.ApprouverDossier(context.Background(), "admin-1", d.ID))

	utilisateurMaj, err := utilisateurs.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, commun.KycTier2, utilisateurMaj.KycTier)

	dossierMaj, err := dossiers.FindByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, kyc.StatutDossierApprouve, dossierMaj.Statut)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "admin-1", auditLog.Entrees[0].AdminID)
	assert.Equal(t, "kyc_tier2_approuve", auditLog.Entrees[0].Action)
}

func TestAdminKycService_RejeterDossier(t *testing.T) {
	service, utilisateurs, dossiers, _, auditLog := setupAdminKycService()
	u, d := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	require.NoError(t, service.RejeterDossier(context.Background(), "admin-1", d.ID, "pièce illisible"))

	utilisateurInchange, err := utilisateurs.FindByID(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, commun.KycTier1, utilisateurInchange.KycTier, "un rejet ne change pas le tier de l'utilisateur")

	dossierMaj, err := dossiers.FindByID(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, kyc.StatutDossierRejete, dossierMaj.Statut)
	assert.Equal(t, "pièce illisible", dossierMaj.MotifRejet)

	require.Len(t, auditLog.Entrees, 1)
	assert.Equal(t, "kyc_tier2_rejete", auditLog.Entrees[0].Action)
}

func TestAdminKycService_ApprouverDossier_Introuvable(t *testing.T) {
	service, _, _, _, _ := setupAdminKycService()

	err := service.ApprouverDossier(context.Background(), "admin-1", "dossier-inconnu")
	assert.ErrorIs(t, err, kyc.ErrDossierKycIntrouvable)
}

func TestAdminKycService_ListerDocuments(t *testing.T) {
	service, utilisateurs, dossiers, documents, _ := setupAdminKycService()
	u, dossier := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	d, err := kyc.NouveauDocumentKyc(u.ID, dossier.ID, kyc.TypeDocumentRectoPieceIdentite, "cni.jpg", "/faux/chemin/cni.jpg", "NOM: KONE")
	require.NoError(t, err)
	require.NoError(t, documents.Create(context.Background(), d))

	liste, err := service.ListerDocuments(context.Background(), dossier.ID)
	require.NoError(t, err)
	require.Len(t, liste, 1)
	assert.Equal(t, "NOM: KONE", liste[0].TexteExtrait)
}

func TestAdminKycService_RecupererDocument_Succes(t *testing.T) {
	service, utilisateurs, dossiers, documents, _ := setupAdminKycService()
	u, dossier := utilisateurTier1AvecDossier(t, utilisateurs, dossiers)

	d, err := kyc.NouveauDocumentKyc(u.ID, dossier.ID, kyc.TypeDocumentRectoPieceIdentite, "cni.jpg", "/faux/chemin/cni.jpg", "NOM: KONE")
	require.NoError(t, err)
	require.NoError(t, documents.Create(context.Background(), d))

	document, contenu, err := service.RecupererDocument(context.Background(), d.ID)
	require.NoError(t, err)
	assert.Equal(t, "cni.jpg", document.NomFichier)
	assert.NotEmpty(t, contenu, "l'administrateur doit recevoir l'image elle-même, pas seulement le texte OCR")
}

func TestAdminKycService_RecupererDocument_Introuvable(t *testing.T) {
	service, _, _, _, _ := setupAdminKycService()

	_, _, err := service.RecupererDocument(context.Background(), "document-inconnu")
	assert.ErrorIs(t, err, kyc.ErrDocumentKycIntrouvable)
}
