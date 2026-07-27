package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

// Logger journalise chaque requête HTTP en structuré (zerolog), séparément
// des logs métier/audit.
func Logger(log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		debut := time.Now()
		err := c.Next()

		// Le ErrorHandler global (cmd/api/main.go) ne s'exécute qu'après le
		// retour de c.Next() : à ce stade, c.Response().StatusCode() ne
		// reflète pas encore le code réellement envoyé au client en cas
		// d'erreur. On le déduit donc de err, avec la même logique que le
		// ErrorHandler.
		statut := c.Response().StatusCode()
		if err != nil {
			statut = fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				statut = e.Code
			}
		}

		log.Info().
			Str("methode", c.Method()).
			Str("chemin", c.Path()).
			Int("statut", statut).
			Dur("duree", time.Since(debut)).
			Msg("requête traitée")

		return err
	}
}
