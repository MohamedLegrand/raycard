-- Connexion via Google : un compte peut n'avoir aucun mot de passe
-- (créé directement via Google), d'où le NOT NULL relâché.
ALTER TABLE utilisateurs
    ALTER COLUMN mot_de_passe_hash DROP NOT NULL,
    ADD COLUMN google_id TEXT;

-- L'unicité ignore les NULL (comportement standard Postgres) : plusieurs
-- comptes sans Google lié coexistent sans problème.
CREATE UNIQUE INDEX uq_utilisateurs_google_id ON utilisateurs (google_id) WHERE google_id IS NOT NULL;
