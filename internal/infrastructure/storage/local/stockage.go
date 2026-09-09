// Package local implémente commun.StockageFichier en écrivant les
// fichiers téléversés sur le disque du serveur, sous un répertoire
// configurable. Adaptée au développement (aucune dépendance externe),
// mais ne survit ni à un redéploiement ni à plusieurs instances sans
// volume partagé — voir internal/infrastructure/storage/s3, l'adaptateur
// recommandé en production, choisi à la place de celui-ci selon
// Config.S3Bucket (voir cmd/api/main.go). Le port
// output.StockageFichier isole ce choix du reste du code.
package local

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"raycard/internal/core/domain/commun"
)

type StockageFichier struct {
	repertoireBase string
}

func NewStockageFichier(repertoireBase string) *StockageFichier {
	return &StockageFichier{repertoireBase: repertoireBase}
}

// Sauvegarder écrit le contenu sous un nom généré (jamais le nom
// fourni par le client, pour éviter tout risque de traversée de
// répertoire ou d'écrasement) et renvoie le chemin absolu du fichier.
func (s *StockageFichier) Sauvegarder(_ context.Context, nomFichier string, contenu []byte) (string, error) {
	if err := os.MkdirAll(s.repertoireBase, 0o755); err != nil {
		return "", fmt.Errorf("création répertoire de stockage: %w", err)
	}

	nomGenere := commun.NewID() + filepath.Ext(nomFichier)
	chemin := filepath.Join(s.repertoireBase, nomGenere)

	if err := os.WriteFile(chemin, contenu, 0o644); err != nil {
		return "", fmt.Errorf("écriture fichier: %w", err)
	}

	chemin, err := filepath.Abs(chemin)
	if err != nil {
		return "", fmt.Errorf("résolution chemin absolu: %w", err)
	}
	return chemin, nil
}

// Lire relit le contenu d'un fichier précédemment sauvegardé.
func (s *StockageFichier) Lire(_ context.Context, chemin string) ([]byte, error) {
	contenu, err := os.ReadFile(chemin)
	if err != nil {
		return nil, fmt.Errorf("lecture fichier: %w", err)
	}
	return contenu, nil
}
