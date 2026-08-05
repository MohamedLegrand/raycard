DROP TABLE IF EXISTS depenses_carte;
DROP INDEX IF EXISTS idx_cartes_statut;
ALTER TABLE cartes DROP CONSTRAINT IF EXISTS chk_cartes_solde_non_negatif;
ALTER TABLE cartes DROP COLUMN IF EXISTS solde_centimes;
