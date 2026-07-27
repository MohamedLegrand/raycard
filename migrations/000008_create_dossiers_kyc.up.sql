-- Demandes de passage de palier KYC (Tier 1 -> Tier 2), toujours
-- revues manuellement par un administrateur (voir audit_log pour la
-- trace de la décision).
CREATE TABLE dossiers_kyc (
    id              UUID PRIMARY KEY,
    utilisateur_id  UUID        NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    tier_demande    SMALLINT    NOT NULL,
    statut          VARCHAR(20) NOT NULL DEFAULT 'en_attente',
    motif_rejet     TEXT,
    admin_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    traite_at       TIMESTAMPTZ
);

CREATE INDEX idx_dossiers_kyc_statut ON dossiers_kyc (statut);
CREATE INDEX idx_dossiers_kyc_utilisateur_id ON dossiers_kyc (utilisateur_id);

-- Un seul dossier en attente à la fois par utilisateur.
CREATE UNIQUE INDEX uq_dossiers_kyc_utilisateur_en_attente
    ON dossiers_kyc (utilisateur_id)
    WHERE statut = 'en_attente';
