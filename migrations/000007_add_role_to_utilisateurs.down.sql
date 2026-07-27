DROP INDEX IF EXISTS idx_utilisateurs_role;
ALTER TABLE utilisateurs DROP COLUMN IF EXISTS role;
