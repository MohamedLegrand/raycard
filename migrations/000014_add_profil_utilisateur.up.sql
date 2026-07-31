-- Photo de profil, auto-gérée par l'utilisateur.
ALTER TABLE utilisateurs ADD COLUMN photo_profil TEXT;

-- Codes de confirmation de changement d'email : envoyés au NOUVEL
-- email, jamais à l'ancien, pour prouver sa propriété avant que le
-- changement ne prenne effet.
CREATE TABLE tokens_changement_email (
    id              UUID        PRIMARY KEY,
    utilisateur_id  UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    nouvel_email    VARCHAR(255) NOT NULL,
    token_hash      TEXT        NOT NULL,
    expire_at       TIMESTAMPTZ NOT NULL,
    utilise_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tokens_changement_email_token_hash UNIQUE (token_hash)
);

CREATE INDEX idx_tokens_changement_email_utilisateur_id ON tokens_changement_email (utilisateur_id);
