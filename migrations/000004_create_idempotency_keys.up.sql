-- Garantit qu'une opération financière (top-up, débit carte, ...) rejouée
-- avec la même clé d'idempotence ne produit jamais de double effet.
CREATE TABLE idempotency_keys (
    cle           VARCHAR(255) PRIMARY KEY,
    statut        VARCHAR(20)  NOT NULL DEFAULT 'en_cours', -- en_cours | termine | echoue
    reponse_json  JSONB,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expire_at     TIMESTAMPTZ  NOT NULL
);

CREATE INDEX idx_idempotency_keys_expire_at ON idempotency_keys (expire_at);
