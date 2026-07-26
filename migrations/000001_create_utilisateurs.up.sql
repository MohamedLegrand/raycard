CREATE TABLE utilisateurs (
    id                 UUID PRIMARY KEY,
    nom                VARCHAR(100) NOT NULL,
    prenom             VARCHAR(100) NOT NULL,
    email              VARCHAR(255) NOT NULL,
    telephone          VARCHAR(20)  NOT NULL,
    pays_code          CHAR(2)      NOT NULL,
    mot_de_passe_hash  TEXT         NOT NULL,
    kyc_tier           SMALLINT     NOT NULL DEFAULT 0,
    kyc_statut         VARCHAR(20)  NOT NULL DEFAULT 'en_attente',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT uq_utilisateurs_email      UNIQUE (email),
    CONSTRAINT uq_utilisateurs_telephone  UNIQUE (telephone)
);

CREATE INDEX idx_utilisateurs_pays_code ON utilisateurs (pays_code);
