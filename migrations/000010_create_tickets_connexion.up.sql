-- Connexions en attente de second facteur (2FA obligatoire). Le mot de
-- passe est déjà vérifié à ce stade : aucune session n'est émise tant
-- que le code reçu par email n'est pas confirmé via ce ticket.
CREATE TABLE tickets_connexion (
    id                    UUID PRIMARY KEY,
    utilisateur_id        UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    ticket_hash           TEXT        NOT NULL,
    code_hash             TEXT        NOT NULL,
    tentatives_restantes  SMALLINT    NOT NULL,
    expire_at             TIMESTAMPTZ NOT NULL,
    utilise_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_tickets_connexion_ticket_hash UNIQUE (ticket_hash)
);

CREATE INDEX idx_tickets_connexion_utilisateur_id ON tickets_connexion (utilisateur_id);
