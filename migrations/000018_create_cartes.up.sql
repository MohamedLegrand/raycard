-- Cartes virtuelles émises via l'agrégateur de paiement (HR-Skills Pay /
-- Cartevo), financées depuis le wallet. Le PAN et le CVV ne sont jamais
-- stockés ici : le SDK actuel ne les expose d'ailleurs pas au-delà de la
-- création (voir carte.CarteUseCase).
CREATE TABLE cartes (
    id                        UUID         PRIMARY KEY,
    utilisateur_id            UUID         NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    wallet_id                 UUID         NOT NULL REFERENCES wallets (id) ON DELETE CASCADE,
    id_externe                VARCHAR(255) NOT NULL,
    label                     VARCHAR(100) NOT NULL,
    devise                    CHAR(3)      NOT NULL,
    montant_charge_centimes   BIGINT       NOT NULL,
    statut                    VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT chk_cartes_montant_positif       CHECK (montant_charge_centimes > 0),
    CONSTRAINT uq_cartes_id_externe             UNIQUE (id_externe)
);

CREATE INDEX idx_cartes_utilisateur_id ON cartes (utilisateur_id);
