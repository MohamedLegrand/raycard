-- Protection contre le bourrage du code de réinitialisation de mot de
-- passe (6 chiffres, 1M de valeurs possibles, cherché par égalité de
-- hash exacte sans identifiant de compte dans la requête) — une ligne
-- par adresse IP, jamais par utilisateur : contrairement à
-- verrous_connexion, aucune clé étrangère vers utilisateurs (une IP
-- n'appartient à personne en particulier). Voir
-- domain/auth.VerrouReinitialisation.
CREATE TABLE verrous_reinitialisation (
    adresse_ip             TEXT        PRIMARY KEY,
    nombre_echecs          SMALLINT    NOT NULL DEFAULT 0,
    niveau_escalade        SMALLINT    NOT NULL DEFAULT 0,
    derniere_activite_at   TIMESTAMPTZ,
    verrouille_jusqua      TIMESTAMPTZ
);
