package commun

import "context"

// Notifieur envoie des emails transactionnels (OTP, alertes de sécurité,
// confirmations de recharge/retrait/carte...). Le contenu métier (sujet,
// corps) est construit par l'application ; ce port ne connaît que
// l'envoi lui-même — l'isoler permet de changer de fournisseur (Brevo
// aujourd'hui, autre chose demain) sans toucher au reste du code.
type Notifieur interface {
	EnvoyerEmail(ctx context.Context, destinataire, sujet, corpsHTML string) error
}
