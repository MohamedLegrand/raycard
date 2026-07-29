-- Codes de réinitialisation de mot de passe (OTP à 6 chiffres, courte
-- durée de vie), hachés comme les refresh tokens : jamais la valeur
-- brute envoyée par email n'est stockée.
CREATE TABLE tokens_reinitialisation (
    id              UUID PRIMARY KEY,
    utilisateur_id  UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    token_hash      TEXT        NOT NULL,
    expire_at       TIMESTAMPTZ NOT NULL,
    utilise_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tokens_reinitialisation_token_hash UNIQUE (token_hash)
);

CREATE INDEX idx_tokens_reinitialisation_utilisateur_id ON tokens_reinitialisation (utilisateur_id);
