package admin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appadmin "raycard/internal/application/admin"
	domaincommun "raycard/internal/core/domain/commun"
	inputadmin "raycard/internal/core/ports/input/admin"
	outputcommun "raycard/internal/core/ports/output/commun"
	testcarte "raycard/test/application/carte"
	testcommun "raycard/test/application/commun"
)

func nouveauService(utilisateurs *testcommun.UtilisateurRepoFake, wallets *testcommun.WalletRepoFake, cartes *testcarte.CarteRepoFake, auditLog *testcommun.AuditLogRepoFake) inputadmin.AdminUseCase {
	return appadmin.NewAdminService(utilisateurs, wallets, cartes, auditLog)
}

func nouvelUtilisateurTest(t *testing.T, utilisateurs *testcommun.UtilisateurRepoFake, email, telephone string) *domaincommun.Utilisateur {
	t.Helper()
	u, err := domaincommun.NouveauUtilisateur("Koné", "Awa", email, telephone, "CI", "hash")
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), u))
	return u
}

func TestAdminService_ListerUtilisateurs_SansFiltre(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), &testcommun.AuditLogRepoFake{})

	nouvelUtilisateurTest(t, utilisateurs, "awa@example.com", "+2250700000000")
	nouvelUtilisateurTest(t, utilisateurs, "koffi@example.com", "+2250700000001")

	liste, err := service.ListerUtilisateurs(context.Background(), outputcommun.FiltreUtilisateurs{})
	require.NoError(t, err)
	assert.Len(t, liste, 2)
}

func TestAdminService_ListerUtilisateurs_Recherche(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), &testcommun.AuditLogRepoFake{})

	nouvelUtilisateurTest(t, utilisateurs, "awa@example.com", "+2250700000000")
	nouvelUtilisateurTest(t, utilisateurs, "koffi@example.com", "+2250700000001")

	liste, err := service.ListerUtilisateurs(context.Background(), outputcommun.FiltreUtilisateurs{Recherche: "awa"})
	require.NoError(t, err)
	require.Len(t, liste, 1)
	assert.Equal(t, "awa@example.com", liste[0].Email)
}

func TestAdminService_ObtenirUtilisateur_AvecWalletEtCartes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	wallets := testcommun.NewWalletRepoFake()
	cartes := testcarte.NewCarteRepoFake()
	service := nouveauService(utilisateurs, wallets, cartes, &testcommun.AuditLogRepoFake{})

	u := nouvelUtilisateurTest(t, utilisateurs, "awa@example.com", "+2250700000000")
	w, err := domaincommun.NouveauWallet(u.ID, "CI", "XOF", 200000)
	require.NoError(t, err)
	require.NoError(t, wallets.Create(context.Background(), w))

	detail, err := service.ObtenirUtilisateur(context.Background(), u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.ID, detail.Utilisateur.ID)
	require.NotNil(t, detail.Wallet)
	assert.Equal(t, w.ID, detail.Wallet.ID)
	assert.Empty(t, detail.Cartes)
}

func TestAdminService_ObtenirUtilisateur_SansWallet(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), &testcommun.AuditLogRepoFake{})

	// Un compte administrateur n'a pas de wallet (voir
	// commun.NouvelAdministrateur) : son absence ne doit jamais faire
	// échouer la consultation de la fiche.
	admin, err := domaincommun.NouvelAdministrateur("Admin", "RAYCARD", "admin@example.com", "+2250700000099", "CI", "hash", domaincommun.RoleAdmin)
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), admin))

	detail, err := service.ObtenirUtilisateur(context.Background(), admin.ID)
	require.NoError(t, err)
	assert.Nil(t, detail.Wallet)
}

func TestAdminService_ObtenirUtilisateur_Introuvable(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), &testcommun.AuditLogRepoFake{})

	_, err := service.ObtenirUtilisateur(context.Background(), "inconnu")
	assert.ErrorIs(t, err, domaincommun.ErrUtilisateurIntrouvable)
}

func TestAdminService_ListerAuditLogs_Filtres(t *testing.T) {
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(testcommun.NewUtilisateurRepoFake(), testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), auditLog)

	entree1, err := domaincommun.NouvelleEntreeAuditLog("admin-1", "carte_gelee_admin", "carte", "carte-1", "")
	require.NoError(t, err)
	entree2, err := domaincommun.NouvelleEntreeAuditLog("admin-2", "wallet_gele_admin", "wallet", "wallet-1", "")
	require.NoError(t, err)
	require.NoError(t, auditLog.Create(context.Background(), entree1))
	require.NoError(t, auditLog.Create(context.Background(), entree2))

	toutes, err := service.ListerAuditLogs(context.Background(), outputcommun.FiltreAuditLog{})
	require.NoError(t, err)
	assert.Len(t, toutes, 2)

	filtrees, err := service.ListerAuditLogs(context.Background(), outputcommun.FiltreAuditLog{AdminID: "admin-1"})
	require.NoError(t, err)
	require.Len(t, filtrees, 1)
	assert.Equal(t, "carte_gelee_admin", filtrees[0].Action)
}

func TestAdminService_ChangerRoleUtilisateur_Succes(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	auditLog := &testcommun.AuditLogRepoFake{}
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), auditLog)

	superAdmin, err := domaincommun.NouvelAdministrateur("Legrand", "Mohamed", "super@example.com", "+2250700000098", "CI", "hash", domaincommun.RoleSuperAdmin)
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), superAdmin))
	client := nouvelUtilisateurTest(t, utilisateurs, "awa@example.com", "+2250700000000")

	maj, err := service.ChangerRoleUtilisateur(context.Background(), superAdmin.ID, client.ID, domaincommun.RoleAdmin)
	require.NoError(t, err)
	assert.Equal(t, domaincommun.RoleAdmin, maj.Role)

	entrees, err := service.ListerAuditLogs(context.Background(), outputcommun.FiltreAuditLog{})
	require.NoError(t, err)
	require.Len(t, entrees, 1)
	assert.Equal(t, "role_utilisateur_modifie", entrees[0].Action)
	assert.Equal(t, superAdmin.ID, entrees[0].AdminID)
}

func TestAdminService_ChangerRoleUtilisateur_RefusePourSoiMeme(t *testing.T) {
	utilisateurs := testcommun.NewUtilisateurRepoFake()
	service := nouveauService(utilisateurs, testcommun.NewWalletRepoFake(), testcarte.NewCarteRepoFake(), &testcommun.AuditLogRepoFake{})

	superAdmin, err := domaincommun.NouvelAdministrateur("Legrand", "Mohamed", "super@example.com", "+2250700000098", "CI", "hash", domaincommun.RoleSuperAdmin)
	require.NoError(t, err)
	require.NoError(t, utilisateurs.Create(context.Background(), superAdmin))

	_, err = service.ChangerRoleUtilisateur(context.Background(), superAdmin.ID, superAdmin.ID, domaincommun.RoleAdmin)
	assert.ErrorIs(t, err, domaincommun.ErrAutoModificationRole)
}
