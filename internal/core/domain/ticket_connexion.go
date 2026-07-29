package domain

import "time"

// TicketConnexion représente une connexion en attente de second facteur :
// le mot de passe a déjà été vérifié, mais aucune session n'est émise
// tant que le code reçu par email n'a pas été confirmé. Deux secrets
// indépendants sont liés à ce ticket — son propre hash (connu du seul
// client qui vient de prouver son mot de passe) et le hash du code
// (connu du seul client qui a accès à la boîte email) — aucun des deux
// seuls ne suffit à obtenir une session.
type TicketConnexion struct {
	ID                  string
	UtilisateurID       string
	TicketHash          string
	CodeHash            string
	TentativesRestantes int
	ExpireAt            time.Time
	UtiliseAt           *time.Time
	CreatedAt           time.Time
}

// NouveauTicketConnexion crée un ticket de connexion en attente de
// second facteur, valide pour la durée donnée et limité à
// tentativesMax essais de code.
func NouveauTicketConnexion(utilisateurID, ticketHash, codeHash string, tentativesMax int, duree time.Duration) (*TicketConnexion, error) {
	if utilisateurID == "" || ticketHash == "" || codeHash == "" {
		return nil, ErrDonneesInvalides
	}
	if tentativesMax <= 0 || duree <= 0 {
		return nil, ErrDonneesInvalides
	}

	return &TicketConnexion{
		ID:                  NewID(),
		UtilisateurID:       utilisateurID,
		TicketHash:          ticketHash,
		CodeHash:            codeHash,
		TentativesRestantes: tentativesMax,
		ExpireAt:            time.Now().UTC().Add(duree),
		CreatedAt:           time.Now().UTC(),
	}, nil
}

// EstValide indique si le ticket peut encore servir à vérifier un code :
// ni déjà consommé, ni expiré, ni à court de tentatives.
func (t *TicketConnexion) EstValide() bool {
	if t.UtiliseAt != nil {
		return false
	}
	if t.TentativesRestantes <= 0 {
		return false
	}
	return time.Now().UTC().Before(t.ExpireAt)
}

// EnregistrerEchec décrémente les tentatives restantes après un code
// incorrect. Une fois à zéro, EstValide renvoie définitivement false :
// il faut recommencer depuis la connexion (nouveau mot de passe, nouveau
// code).
func (t *TicketConnexion) EnregistrerEchec() {
	if t.TentativesRestantes > 0 {
		t.TentativesRestantes--
	}
}

// Consommer marque le ticket comme utilisé (usage unique, à l'issue
// d'une vérification de code réussie).
func (t *TicketConnexion) Consommer() {
	maintenant := time.Now().UTC()
	t.UtiliseAt = &maintenant
}
