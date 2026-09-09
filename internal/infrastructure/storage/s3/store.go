// Package s3 implémente commun.StockageFichier au-dessus d'un stockage
// objet compatible S3 — AWS S3, mais aussi Cloudflare R2, Backblaze B2 ou
// MinIO auto-hébergé, qui exposent tous une API compatible S3. Le choix
// du fournisseur se fait entièrement par configuration (endpoint, région,
// style d'URL) — voir NewStockageFichier — jamais dans ce fichier.
//
// Remplace local.StockageFichier en production : celle-ci écrit sur le
// disque du serveur, ce qui ne survit ni à un redéploiement ni à
// plusieurs instances sans volume partagé. Le port
// output.StockageFichier isole ce détail : basculer d'une implémentation
// à l'autre ne touche à rien d'autre dans le code (voir cmd/api/main.go,
// qui choisit l'une ou l'autre selon que S3_BUCKET est configuré).
package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"raycard/internal/core/domain/commun"
	outputcommun "raycard/internal/core/ports/output/commun"
)

type StockageFichier struct {
	client *awss3.Client
	bucket string
}

// Vérification à la compilation : StockageFichier implémente bien le
// port attendu par la couche application, comme local.StockageFichier.
var _ outputcommun.StockageFichier = (*StockageFichier)(nil)

// NewStockageFichier construit l'adaptateur. endpoint vide = AWS S3
// natif (résolution d'URL standard AWS) ; endpoint renseigné = n'importe
// quel fournisseur S3-compatible (ex: "https://<compte>.r2.cloudflarestorage.com"
// pour Cloudflare R2). usePathStyle doit être vrai pour la plupart des
// fournisseurs non-AWS (R2, MinIO...) — voir leur documentation
// respective ; faux pour AWS S3 natif (style virtual-hosted par défaut).
func NewStockageFichier(bucket, region, endpoint, accessKeyID, secretAccessKey string, usePathStyle bool) *StockageFichier {
	client := awss3.New(awss3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, ""),
		BaseEndpoint: nonVideOuNil(endpoint),
		UsePathStyle: usePathStyle,
	})
	return &StockageFichier{client: client, bucket: bucket}
}

func nonVideOuNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Sauvegarder implémente commun.StockageFichier. Même politique que
// local.StockageFichier : nom généré (jamais le nom fourni par le
// client, pour ne jamais dépendre d'une entrée utilisateur dans une clé
// d'objet). Le "chemin" renvoyé est ici une clé d'objet S3, pas un
// chemin disque — c'est cette valeur qui est persistée telle quelle sur
// kyc.DocumentKyc.CheminFichier, sans que le domaine ait besoin de savoir
// laquelle des deux implémentations est active.
func (s *StockageFichier) Sauvegarder(ctx context.Context, nomFichier string, contenu []byte) (string, error) {
	cle := commun.NewID() + filepath.Ext(nomFichier)
	_, err := s.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(cle),
		Body:   bytes.NewReader(contenu),
	})
	if err != nil {
		return "", fmt.Errorf("envoi fichier vers le stockage objet: %w", err)
	}
	return cle, nil
}

// Lire relit le contenu d'un objet précédemment sauvegardé, désigné par
// sa clé (voir Sauvegarder).
func (s *StockageFichier) Lire(ctx context.Context, cle string) ([]byte, error) {
	resp, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(cle),
	})
	if err != nil {
		return nil, fmt.Errorf("lecture fichier depuis le stockage objet: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	contenu, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("lecture contenu fichier: %w", err)
	}
	return contenu, nil
}
