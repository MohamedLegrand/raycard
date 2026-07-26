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

		log.Info().
			Str("methode", c.Method()).
			Str("chemin", c.Path()).
			Int("statut", c.Response().StatusCode()).
			Dur("duree", time.Since(debut)).
			Msg("requête traitée")

		return err
	}
}
