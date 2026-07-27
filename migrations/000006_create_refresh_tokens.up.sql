-- Refresh tokens opaques (jamais le token brut n'est stocké, seulement son
-- hash) pour permettre la révocation immédiate (déconnexion, compromission)
-- et la rotation à chaque utilisation.
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY,
    utilisateur_id  UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    token_hash      TEXT        NOT NULL,
    expire_at       TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_refresh_tokens_token_hash UNIQUE (token_hash)
);

CREATE INDEX idx_refresh_tokens_utilisateur_id ON refresh_tokens (utilisateur_id);
