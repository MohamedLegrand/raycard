-- Rattache chaque document à une demande de passage de palier précise
-- (jamais seulement à l'utilisateur) : en cas de rejet puis de
-- resoumission, les pièces de chaque tentative restent séparées.
ALTER TABLE documents_kyc ADD COLUMN dossier_kyc_id UUID NOT NULL REFERENCES dossiers_kyc (id) ON DELETE CASCADE;

-- Catégorise le document (recto/verso de pièce d'identité, justificatif
-- de domicile, selfie) pour que l'administrateur sache ce qu'il regarde
-- sans avoir à deviner à partir de l'image.
ALTER TABLE documents_kyc ADD COLUMN type_document VARCHAR(30) NOT NULL;

CREATE INDEX idx_documents_kyc_dossier_kyc_id ON documents_kyc (dossier_kyc_id);
