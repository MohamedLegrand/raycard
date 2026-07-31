package kyc

import (
	"time"

	"raycard/internal/core/domain/kyc"
)

// DocumentKycDTO expose le document et le texte que l'OCR local en a
// extrait. NomFichier est celui fourni par le client (informatif) ;
// CheminFichier n'est jamais exposé (détail de stockage serveur).
type DocumentKycDTO struct {
	ID           string    `json:"id"`
	NomFichier   string    `json:"nom_fichier"`
	TexteExtrait string    `json:"texte_extrait"`
	CreatedAt    time.Time `json:"created_at"`
}

func FromDocumentKyc(d *kyc.DocumentKyc) DocumentKycDTO {
	return DocumentKycDTO{
		ID:           d.ID,
		NomFichier:   d.NomFichier,
		TexteExtrait: d.TexteExtrait,
		CreatedAt:    d.CreatedAt,
	}
}

func FromDocumentsKyc(documents []*kyc.DocumentKyc) []DocumentKycDTO {
	resultat := make([]DocumentKycDTO, 0, len(documents))
	for _, d := range documents {
		resultat = append(resultat, FromDocumentKyc(d))
	}
	return resultat
}
