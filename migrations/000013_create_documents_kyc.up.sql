-- Documents d'identité téléversés pour la revue KYC, et texte extrait
-- par OCR local (Tesseract) — une simple aide à la saisie pour
-- l'administrateur, jamais utilisé pour une décision automatique.
CREATE TABLE documents_kyc (
    id              UUID         PRIMARY KEY,
    utilisateur_id  UUID         NOT NULL REFERENCES utilisateurs (id) ON DELETE CASCADE,
    nom_fichier     VARCHAR(255) NOT NULL,
    chemin_fichier  TEXT         NOT NULL,
    texte_extrait   TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_kyc_utilisateur_id ON documents_kyc (utilisateur_id);
