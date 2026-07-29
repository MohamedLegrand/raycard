DROP INDEX IF EXISTS uq_utilisateurs_google_id;

ALTER TABLE utilisateurs
    DROP COLUMN IF EXISTS google_id,
    ALTER COLUMN mot_de_passe_hash SET NOT NULL;
