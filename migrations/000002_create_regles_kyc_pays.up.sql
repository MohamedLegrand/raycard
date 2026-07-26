CREATE TABLE regles_kyc_pays (
    pays_code                 CHAR(2)     NOT NULL,
    tier                      SMALLINT    NOT NULL,
    devise                    CHAR(3)     NOT NULL,
    plafond_solde_centimes    BIGINT      NOT NULL,
    plafond_mensuel_centimes  BIGINT      NOT NULL,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (pays_code, tier)
);

-- Données d'amorçage à titre d'exemple pour le développement.
-- Les montants sont en unité mineure de la devise (ici XOF, 0 décimale,
-- donc l'unité mineure == l'unité principale). À faire valider par la
-- conformité avant toute mise en production (seuils UEMOA réels).
INSERT INTO regles_kyc_pays (pays_code, tier, devise, plafond_solde_centimes, plafond_mensuel_centimes) VALUES
    ('CI', 1, 'XOF',  200000,  500000),
    ('CI', 2, 'XOF', 2000000, 10000000),
    ('SN', 1, 'XOF',  200000,  500000),
    ('SN', 2, 'XOF', 2000000, 10000000);
