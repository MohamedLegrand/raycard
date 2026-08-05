-- Données d'amorçage pour le Cameroun (XAF), mêmes plafonds que CI/SN à
-- titre d'exemple pour le développement — à faire valider par la
-- conformité avant toute mise en production (seuils réels).
INSERT INTO regles_kyc_pays (pays_code, tier, devise, plafond_solde_centimes, plafond_mensuel_centimes) VALUES
    ('CM', 1, 'XAF',  200000,  500000),
    ('CM', 2, 'XAF', 2000000, 10000000);
