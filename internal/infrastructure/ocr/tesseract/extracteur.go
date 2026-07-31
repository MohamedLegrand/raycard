// Package tesseract implémente kyc.OcrExtracteur en appelant le
// binaire `tesseract` via os/exec — pas le binding cgo (gosseract) —
// pour que le binaire Go reste 100% pur et se compile sans dépendance
// système. Seule la machine qui exécute le serveur a besoin du paquet
// tesseract-ocr installé (ex: `apt install tesseract-ocr
// tesseract-ocr-fra`), pas celle qui le compile.
package tesseract

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Extracteur struct {
	langue string
}

// NewExtracteur construit un Extracteur pour la langue donnée (code
// Tesseract, ex: "fra", "eng", ou "fra+eng" pour combiner plusieurs
// modèles entraînés).
func NewExtracteur(langue string) *Extracteur {
	return &Extracteur{langue: langue}
}

// ExtraireTexte lit le texte visible sur l'image au chemin donné.
// "stdout" comme nom de sortie demande à tesseract d'écrire le résultat
// sur sa sortie standard plutôt que dans un fichier.
func (e *Extracteur) ExtraireTexte(ctx context.Context, cheminImage string) (string, error) {
	cmd := exec.CommandContext(ctx, "tesseract", cheminImage, "stdout", "-l", e.langue)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("exécution tesseract: %w (%s)", err, stderr.String())
	}

	return stdout.String(), nil
}
