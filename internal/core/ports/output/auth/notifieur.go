package auth

import "context"

// Notifieur envoie des emails transactionnels (OTP, mot de passe
// oublié, alertes de sécurité...). Le contenu métier (sujet, corps) est
// construit par l'application ; ce port ne connaît que l'envoi lui-même —
// l'isoler permet de changer de fournisseur (Brevo aujourd'hui, autre
// chose demain) sans toucher au reste du code.
type Notifieur interface {
	EnvoyerEmail(ctx context.Context, destinataire, sujet, corpsHTML string) error
}
