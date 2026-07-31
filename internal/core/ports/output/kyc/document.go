package kyc

import "context"

// OcrExtracteur extrait le texte visible d'une image de document.
// Purement une aide à la saisie pour la revue manuelle : aucune
// décision n'est prise à partir de son résultat. Isoler ce port permet
// de remplacer le moteur OCR (Tesseract local aujourd'hui, service
// spécialisé demain) sans toucher au reste du code.
type OcrExtracteur interface {
	ExtraireTexte(ctx context.Context, cheminImage string) (string, error)
}
