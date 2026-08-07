// Package brevo implémente commun.Notifieur via l'API transactionnelle
// email de Brevo (https://developers.brevo.com/reference/sendtransacemail).
package brevo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const urlEnvoiEmail = "https://api.brevo.com/v3/smtp/email"

type Notifieur struct {
	apiKey     string
	expediteur string
	client     *http.Client
}

func NewNotifieur(apiKey, expediteur string) *Notifieur {
	return &Notifieur{
		apiKey:     apiKey,
		expediteur: expediteur,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

type requeteEnvoi struct {
	Sender      adresse   `json:"sender"`
	To          []adresse `json:"to"`
	Subject     string    `json:"subject"`
	HTMLContent string    `json:"htmlContent"`
}

type adresse struct {
	Email string `json:"email"`
}

func (n *Notifieur) EnvoyerEmail(ctx context.Context, destinataire, sujet, corpsHTML string) error {
	corps, err := json.Marshal(requeteEnvoi{
		Sender:      adresse{Email: n.expediteur},
		To:          []adresse{{Email: destinataire}},
		Subject:     sujet,
		HTMLContent: corpsHTML,
	})
	if err != nil {
		return fmt.Errorf("encodage requête brevo: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlEnvoiEmail, bytes.NewReader(corps))
	if err != nil {
		return fmt.Errorf("construction requête brevo: %w", err)
	}
	req.Header.Set("api-key", n.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("appel brevo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		corpsErreur, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo a répondu %d: %s", resp.StatusCode, corpsErreur)
	}
	return nil
}
