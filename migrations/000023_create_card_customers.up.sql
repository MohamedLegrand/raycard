-- Porteurs de carte : le KYC distinct exigé par l'agrégateur (Cartevo,
-- via HR-Skills Pay) avant de pouvoir émettre une carte virtuelle pour un
-- utilisateur — séparé du KYC Tier 2 de RAYCARD lui-même. Un seul dossier
-- par utilisateur (voir la contrainte d'unicité) : un rejet ne supprime
-- jamais la ligne, son statut change simplement.
CREATE TABLE card_customers (
    id              UUID PRIMARY KEY,
    utilisateur_id  UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    id_externe      TEXT,
    statut          TEXT        NOT NULL,
    motif_rejet     TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_card_customers_utilisateur_id UNIQUE (utilisateur_id)
);

CREATE INDEX idx_card_customers_statut ON card_customers (statut);
