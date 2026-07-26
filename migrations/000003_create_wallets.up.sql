CREATE TABLE wallets (
    id                       UUID PRIMARY KEY,
    utilisateur_id           UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    pays_code                CHAR(2)     NOT NULL,
    devise                   CHAR(3)     NOT NULL,
    solde_centimes           BIGINT      NOT NULL DEFAULT 0,
    plafond_solde_centimes   BIGINT      NOT NULL,
    statut                   VARCHAR(20) NOT NULL DEFAULT 'actif',
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- V1 : un seul wallet par utilisateur.
    CONSTRAINT uq_wallets_utilisateur_id      UNIQUE (utilisateur_id),
    CONSTRAINT chk_wallets_solde_non_negatif  CHECK (solde_centimes >= 0)
);
