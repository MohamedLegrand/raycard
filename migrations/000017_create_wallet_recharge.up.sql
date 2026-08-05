-- Scinde le solde du wallet en disponible (utilisable immédiatement) et
-- en attente (crédité mais bloqué, ex: délai de retenue de 48h imposé
-- par l'agrégateur de paiement après une recharge Mobile Money). Le
-- plafond réglementaire du palier KYC s'applique au total des deux.
ALTER TABLE wallets RENAME COLUMN solde_centimes TO solde_disponible_centimes;
ALTER TABLE wallets ADD COLUMN solde_en_attente_centimes BIGINT NOT NULL DEFAULT 0;

ALTER TABLE wallets DROP CONSTRAINT chk_wallets_solde_non_negatif;
ALTER TABLE wallets ADD CONSTRAINT chk_wallets_solde_disponible_non_negatif CHECK (solde_disponible_centimes >= 0);
ALTER TABLE wallets ADD CONSTRAINT chk_wallets_solde_en_attente_non_negatif CHECK (solde_en_attente_centimes >= 0);

-- Trace chaque mouvement de fonds initié via un agrégateur de paiement
-- externe (recharge Mobile Money aujourd'hui, retrait/financement de
-- carte plus tard). Le cycle en_attente -> envoyee -> succes/echouee
-- reflète le principe "créer en attente, ne créditer qu'après
-- confirmation externe" appliqué à toute opération financière du wallet.
CREATE TABLE transactions_wallet (
    id                  UUID         PRIMARY KEY,
    wallet_id           UUID         NOT NULL REFERENCES wallets (id) ON DELETE CASCADE,
    utilisateur_id      UUID         NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    type                VARCHAR(30)  NOT NULL,
    statut              VARCHAR(20)  NOT NULL DEFAULT 'en_attente',
    montant_centimes    BIGINT       NOT NULL,
    frais_centimes      BIGINT       NOT NULL DEFAULT 0,
    devise              CHAR(3)      NOT NULL,
    operateur           VARCHAR(50)  NOT NULL,
    telephone           VARCHAR(20)  NOT NULL,
    reference_externe   VARCHAR(255),
    disponible_le       TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_transactions_wallet_montant_positif CHECK (montant_centimes > 0),
    CONSTRAINT chk_transactions_wallet_frais_non_negatif CHECK (frais_centimes >= 0),
    CONSTRAINT uq_transactions_wallet_reference_externe UNIQUE (reference_externe)
);

CREATE INDEX idx_transactions_wallet_wallet_id ON transactions_wallet (wallet_id);
CREATE INDEX idx_transactions_wallet_reference_externe ON transactions_wallet (reference_externe);
-- Accélère la vérification "une recharge est-elle déjà en cours sur ce
-- wallet ?" à chaque nouvelle demande.
CREATE INDEX idx_transactions_wallet_en_cours ON transactions_wallet (wallet_id) WHERE statut IN ('en_attente', 'envoyee');
-- Accélère le scan périodique des transactions dont le délai de retenue
-- est écoulé (bascule en_attente -> disponible).
CREATE INDEX idx_transactions_wallet_disponible_le ON transactions_wallet (disponible_le) WHERE disponible_le IS NOT NULL;
