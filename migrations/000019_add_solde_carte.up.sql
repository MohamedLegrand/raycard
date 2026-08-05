-- Dernier solde connu de la carte chez l'agrégateur, tenu à jour par un
-- job planifié (pas de webhook de transaction carte disponible). Initialisé
-- au montant chargé pour les cartes déjà existantes.
ALTER TABLE cartes ADD COLUMN solde_centimes BIGINT NOT NULL DEFAULT 0;
UPDATE cartes SET solde_centimes = montant_charge_centimes;
ALTER TABLE cartes ADD CONSTRAINT chk_cartes_solde_non_negatif CHECK (solde_centimes >= 0);

-- Accélère le scan périodique des cartes actives par le job de
-- synchronisation des soldes.
CREATE INDEX idx_cartes_statut ON cartes (statut) WHERE statut = 'active';

-- Dépenses détectées par rapprochement de solde — jamais une
-- autorisation en temps réel (voir carte.DepenseCarte).
CREATE TABLE depenses_carte (
    id                     UUID        PRIMARY KEY,
    carte_id               UUID        NOT NULL REFERENCES cartes (id) ON DELETE CASCADE,
    montant_centimes       BIGINT      NOT NULL,
    solde_avant_centimes   BIGINT      NOT NULL,
    solde_apres_centimes   BIGINT      NOT NULL,
    detected_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_depenses_carte_montant_positif CHECK (montant_centimes > 0)
);

CREATE INDEX idx_depenses_carte_carte_id ON depenses_carte (carte_id);
