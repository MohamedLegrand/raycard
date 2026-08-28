package middleware

import (
	"runtime/debug"
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

		statut := c.Response().StatusCode()
		if err != nil {
			statut = fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				statut = e.Code
			}
		}

		if statut >= 500 {
			logEvt := log.Error().
				Str("methode", c.Method()).
				Str("chemin", c.Path()).
				Int("statut", statut).
				Dur("duree", time.Since(debut))

			if err != nil {
				logEvt = logEvt.Err(err).Str("stack", string(debug.Stack()))
			}

			logEvt.Msg("erreur serveur HTTP (500)")
		} else if statut >= 400 {
			logEvt := log.Warn().
				Str("methode", c.Method()).
				Str("chemin", c.Path()).
				Int("statut", statut).
				Dur("duree", time.Since(debut))

			if err != nil {
				logEvt = logEvt.Err(err)
			}

			logEvt.Msg("requête HTTP rejetée (4xx)")
		} else {
			log.Info().
				Str("methode", c.Method()).
				Str("chemin", c.Path()).
				Int("statut", statut).
				Dur("duree", time.Since(debut)).
				Msg("requête traitée")
		}

		return err
	}
}
