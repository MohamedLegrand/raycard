-- Pilote la fréquence de sondage du solde d'une carte (voir
-- carte.Carte.MettreAJourSolde) : sondage adaptatif plutôt que
-- systématique, pour ne pas solliciter l'agrégateur au même rythme pour
-- une carte dormante que pour une carte activement utilisée.
ALTER TABLE cartes ADD COLUMN prochaine_verification_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE cartes ADD COLUMN niveau_verification INT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_cartes_statut;
-- Accélère la requête "quelles cartes actives sont dues pour un sondage ?"
CREATE INDEX idx_cartes_a_verifier ON cartes (prochaine_verification_at) WHERE statut = 'active';
