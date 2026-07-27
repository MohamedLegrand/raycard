package domain

import "time"

// AuditLog trace une action administrateur sensible (validation KYC,
// gel de compte, modification de règle de cashback...), séparément des
// logs applicatifs — exigence du cahier des charges V1.
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
