DROP TABLE IF EXISTS transactions_wallet;

ALTER TABLE wallets DROP CONSTRAINT IF EXISTS chk_wallets_solde_disponible_non_negatif;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS chk_wallets_solde_en_attente_non_negatif;
ALTER TABLE wallets DROP COLUMN IF EXISTS solde_en_attente_centimes;
ALTER TABLE wallets ADD CONSTRAINT chk_wallets_solde_non_negatif CHECK (solde_disponible_centimes >= 0);
ALTER TABLE wallets RENAME COLUMN solde_disponible_centimes TO solde_centimes;
