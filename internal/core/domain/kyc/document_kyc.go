package kyc

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// DocumentKyc trace un document d'identité téléversé par un utilisateur
// (CNI, passeport...) et le texte que l'OCR local en a extrait. Ce
// texte n'est qu'une aide à la saisie pour l'administrateur qui traite
// le dossier de passage de palier : aucune décision n'est prise
// automatiquement à partir de son contenu.
type DocumentKyc struct {
	ID            string
	UtilisateurID string
	NomFichier    string
	CheminFichier string
	TexteExtrait  string
	CreatedAt     time.Time
}

// NouveauDocumentKyc trace un document déjà stocké et déjà passé par
// l'OCR : chemin et texte extrait sont donc obligatoires (à la
// différence de nomFichier, purement informatif).
func NouveauDocumentKyc(utilisateurID, nomFichier, cheminFichier, texteExtrait string) (*DocumentKyc, error) {
	if utilisateurID == "" || cheminFichier == "" {
		return nil, commun.ErrDonneesInvalides
	}

	return &DocumentKyc{
		ID:            commun.NewID(),
		UtilisateurID: utilisateurID,
		NomFichier:    nomFichier,
		CheminFichier: cheminFichier,
		TexteExtrait:  texteExtrait,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
