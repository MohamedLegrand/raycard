ALTER TABLE utilisateurs
    ADD COLUMN role VARCHAR(20) NOT NULL DEFAULT 'utilisateur';

CREATE INDEX idx_utilisateurs_role ON utilisateurs (role);
