-- Trace toute action administrateur sensible (validation KYC, gel de
-- compte, modification de règle de cashback...), séparément des logs
-- applicatifs. admin_id n'a volontairement pas de clé étrangère : la
-- table des comptes administrateurs n'existe pas encore dans cette V1.
CREATE TABLE audit_log (
    id            UUID PRIMARY KEY,
    admin_id      UUID        NOT NULL,
    action        VARCHAR(100) NOT NULL,
    cible_type    VARCHAR(50),
    cible_id      UUID,
    details_json  JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_log_admin_id ON audit_log (admin_id);
CREATE INDEX idx_audit_log_created_at ON audit_log (created_at);
