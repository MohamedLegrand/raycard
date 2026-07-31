-- Protection contre le bourrage de mot de passe (brute force) : une
-- ligne par utilisateur, mise à jour à chaque échec/succès de
-- connexion. La durée du verrou double à chaque nouveau cycle d'échecs
-- (voir domain/auth.VerrouConnexion), donc seul l'état courant est
-- nécessaire ici, pas un historique des tentatives.
CREATE TABLE verrous_connexion (
    utilisateur_id        UUID        PRIMARY KEY REFERENCES utilisateurs (id) ON DELETE CASCADE,
    nombre_echecs          SMALLINT    NOT NULL DEFAULT 0,
    niveau_escalade        SMALLINT    NOT NULL DEFAULT 0,
    derniere_activite_at   TIMESTAMPTZ,
    verrouille_jusqua      TIMESTAMPTZ
);
