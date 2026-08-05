DROP INDEX IF EXISTS idx_cartes_a_verifier;
CREATE INDEX idx_cartes_statut ON cartes (statut) WHERE statut = 'active';
ALTER TABLE cartes DROP COLUMN IF EXISTS niveau_verification;
ALTER TABLE cartes DROP COLUMN IF EXISTS prochaine_verification_at;
