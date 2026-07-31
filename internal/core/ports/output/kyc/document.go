package kyc

import "context"

// StockageFichier persiste le contenu brut d'un fichier téléversé (ex:
// document d'identité) et renvoie un chemin permettant de le relire
// ensuite (ex: pour l'OCR, ou pour l'affichage côté back-office).
type StockageFichier interface {
	Sauvegarder(ctx context.Context, nomFichier string, contenu []byte) (chemin string, err error)
}

// OcrExtracteur extrait le texte visible d'une image de document.
// Purement une aide à la saisie pour la revue manuelle : aucune
// décision n'est prise à partir de son résultat. Isoler ce port permet
// de remplacer le moteur OCR (Tesseract local aujourd'hui, service
// spécialisé demain) sans toucher au reste du code.
type OcrExtracteur interface {
	ExtraireTexte(ctx context.Context, cheminImage string) (string, error)
}
