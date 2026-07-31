-- Connexion par empreinte digitale : chaque appareil génère une paire de
-- clés après déverrouillage biométrique (empreinte jamais transmise au
-- serveur) ; seule la clé publique est stockée ici. La connexion se fait
-- par preuve de possession de la clé privée (signature d'un challenge),
-- sans repasser par le code de vérification par email.
CREATE TABLE cles_appareil (
    id                       UUID         PRIMARY KEY,
    utilisateur_id           UUID         NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    cle_publique             TEXT         NOT NULL,
    nom_appareil             VARCHAR(100) NOT NULL,
    derniere_utilisation_at  TIMESTAMPTZ,
    revoked_at               TIMESTAMPTZ,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_cles_appareil_utilisateur_id ON cles_appareil (utilisateur_id);

-- Challenges (nonces) à signer par la clé privée de l'appareil pour
-- prouver sa possession, à usage unique et de courte durée de vie.
CREATE TABLE challenges_empreinte (
    id               UUID        PRIMARY KEY,
    cle_appareil_id  UUID        NOT NULL REFERENCES cles_appareil (id) ON DELETE CASCADE,
    nonce            TEXT        NOT NULL,
    expire_at        TIMESTAMPTZ NOT NULL,
    utilise_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_challenges_empreinte_cle_appareil_id ON challenges_empreinte (cle_appareil_id);
