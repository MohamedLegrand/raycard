package commun

import "time"

// IDSysteme identifie un acteur non-humain (job planifié, financement
// automatique déclenché par une erreur agrégateur...) dans une entrée
// d'audit — jamais une chaîne libre comme "systeme" : admin_id est une
// colonne UUID (voir la migration audit_log), toute valeur qui n'est pas
// un UUID syntaxiquement valide fait échouer l'écriture (silencieusement,
// l'écriture d'audit étant best-effort — voir les appelants de
// NouvelleEntreeAuditLog). Le nil UUID est réservé à cet usage, jamais
// généré par NewID() (qui produit un UUID v4, jamais tout-zéro).
const IDSysteme = "00000000-0000-0000-0000-000000000000"

// AuditLog trace une action administrateur sensible (validation KYC,
// gel de compte, modification de règle de cashback...), séparément des
// logs applicatifs — exigence du cahier des charges V1. Partagé par
// tous les modules qui exposent des actions administrateur.
type AuditLog struct {
	ID          string
	AdminID     string
	Action      string
	CibleType   string
	CibleID     string
	DetailsJSON string // JSON brut ; le domaine ne connaît pas de bibliothèque de sérialisation spécifique
	CreatedAt   time.Time
}

// NouvelleEntreeAuditLog crée une entrée d'audit prête à être persistée.
func NouvelleEntreeAuditLog(adminID, action, cibleType, cibleID, detailsJSON string) (*AuditLog, error) {
	if adminID == "" || action == "" {
		return nil, ErrDonneesInvalides
	}

	return &AuditLog{
		ID:          NewID(),
		AdminID:     adminID,
		Action:      action,
		CibleType:   cibleType,
		CibleID:     cibleID,
		DetailsJSON: detailsJSON,
		CreatedAt:   time.Now().UTC(),
	}, nil
}
