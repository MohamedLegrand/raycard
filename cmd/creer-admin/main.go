// Commande cmd/creer-admin : bootstrap du tout premier compte
// administrateur RAYCARD. Volontairement séparée de l'API HTTP — il
// n'existe aucun endpoint public pour créer un admin, ce serait une
// faille de sécurité évidente. À exécuter manuellement sur le serveur.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"

	"raycard/internal/core/domain"
	"raycard/internal/infrastructure/config"
	"raycard/internal/infrastructure/database/postgres"
)

func main() {
	nom := flag.String("nom", "", "nom de l'administrateur")
	prenom := flag.String("prenom", "", "prénom de l'administrateur")
	email := flag.String("email", "", "email de connexion")
	telephone := flag.String("telephone", "", "numéro de téléphone")
	paysCode := flag.String("pays-code", "", "code pays ISO 3166-1 alpha-2, ex: CI")
	motDePasse := flag.String("mot-de-passe", "", "mot de passe (min. 8 caractères)")
	flag.Parse()

	if *nom == "" || *prenom == "" || *email == "" || *telephone == "" || *paysCode == "" || len(*motDePasse) < 8 {
		fmt.Fprintln(os.Stderr, "usage: creer-admin -nom=... -prenom=... -email=... -telephone=... -pays-code=CI -mot-de-passe=... (min. 8 caractères)")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "chargement configuration:", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connexion base de données:", err)
		os.Exit(1)
	}
	defer pool.Close()

	utilisateurRepo := postgres.NewUtilisateurRepository(pool)

	if _, err := utilisateurRepo.FindByEmail(ctx, *email); !errors.Is(err, domain.ErrUtilisateurIntrouvable) {
		if err == nil {
			fmt.Fprintln(os.Stderr, "un compte existe déjà avec cet email")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "vérification email:", err)
		os.Exit(1)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(*motDePasse), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hachage mot de passe:", err)
		os.Exit(1)
	}

	admin, err := domain.NouvelAdministrateur(*nom, *prenom, *email, *telephone, *paysCode, string(hash))
	if err != nil {
		fmt.Fprintln(os.Stderr, "construction administrateur:", err)
		os.Exit(1)
	}

	if err := utilisateurRepo.Create(ctx, admin); err != nil {
		fmt.Fprintln(os.Stderr, "création administrateur:", err)
		os.Exit(1)
	}

	fmt.Printf("Administrateur créé avec succès (id=%s, email=%s)\n", admin.ID, admin.Email)
}
