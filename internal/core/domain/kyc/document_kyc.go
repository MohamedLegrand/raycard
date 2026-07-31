package kyc

import (
	"time"

	"raycard/internal/core/domain/commun"
)

// TypeDocument catégorise la pièce téléversée, pour que l'administrateur
// sache ce qu'il regarde sans avoir à deviner à partir de l'image.
type TypeDocument string

const (
	TypeDocumentRectoPieceIdentite   TypeDocument = "recto_piece_identite"
	TypeDocumentVersoPieceIdentite   TypeDocument = "verso_piece_identite"
	TypeDocumentJustificatifDomicile TypeDocument = "justificatif_domicile"
	TypeDocumentSelfie               TypeDocument = "selfie"
)

var typesDocumentValides = map[TypeDocument]bool{
	TypeDocumentRectoPieceIdentite:   true,
	TypeDocumentVersoPieceIdentite:   true,
	TypeDocumentJustificatifDomicile: true,
	TypeDocumentSelfie:               true,
}

// DocumentKyc trace un document d'identité téléversé par un utilisateur
// (CNI, passeport, justificatif de domicile...) et le texte que l'OCR
// local en a extrait. Ce texte n'est qu'une aide à la saisie pour
// l'administrateur qui traite le dossier : aucune décision n'est prise
// automatiquement à partir de son contenu.
//
// Rattaché à un DossierKyc précis (pas seulement à l'utilisateur) : en
// cas de rejet puis de resoumission, les documents de chaque tentative
// restent séparés — l'administrateur qui traite la nouvelle demande ne
// doit jamais se retrouver avec les pièces de l'ancienne demande
// rejetée mélangées aux nouvelles.
type DocumentKyc struct {
	ID            string
	UtilisateurID string
	DossierKycID  string
	TypeDocument  TypeDocument
	NomFichier    string
	CheminFichier string
	TexteExtrait  string
	CreatedAt     time.Time
}

// NouveauDocumentKyc trace un document déjà stocké et déjà passé par
// l'OCR : chemin et texte extrait sont donc obligatoires (à la
// différence de nomFichier, purement informatif).
func NouveauDocumentKyc(utilisateurID, dossierKycID string, typeDocument TypeDocument, nomFichier, cheminFichier, texteExtrait string) (*DocumentKyc, error) {
	if utilisateurID == "" || dossierKycID == "" || cheminFichier == "" {
		return nil, commun.ErrDonneesInvalides
	}
	if !typesDocumentValides[typeDocument] {
		return nil, commun.ErrDonneesInvalides
	}

	return &DocumentKyc{
		ID:            commun.NewID(),
		UtilisateurID: utilisateurID,
		DossierKycID:  dossierKycID,
		TypeDocument:  typeDocument,
		NomFichier:    nomFichier,
		CheminFichier: cheminFichier,
		TexteExtrait:  texteExtrait,
		CreatedAt:     time.Now().UTC(),
	}, nil
}
