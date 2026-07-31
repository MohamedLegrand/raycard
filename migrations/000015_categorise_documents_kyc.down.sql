DROP INDEX IF EXISTS idx_documents_kyc_dossier_kyc_id;
ALTER TABLE documents_kyc DROP COLUMN IF EXISTS type_document;
ALTER TABLE documents_kyc DROP COLUMN IF EXISTS dossier_kyc_id;
