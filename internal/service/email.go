package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/smtp"
	"time"
)

// errNoMailer is returned when the server has neither Resend nor SMTP set up.
var errNoMailer = errors.New("server has no SMTP or Resend credentials configured")

// emailSender delivers one HTML email. The service's own is sendEmail; tests
// pass a fake so nothing leaves the machine.
type emailSender func(ctx context.Context, to, subject, htmlBody string) error

// mailConfigured reports whether the server can send email at all.
func (s *Service) mailConfigured() bool {
	return s.cfg.SMTPHost != "" || s.cfg.ResendAPIKey != ""
}

// notificationEmail is where a user's email goes: the address they set for
// notifications, or else the one they signed up with. Empty when neither is
// known.
func (s *Service) notificationEmail(ctx context.Context, userID string) string {
	if addr, _ := s.repo.GetUserSetting(ctx, userID, "notification_email"); addr != "" {
		return addr
	}
	if user, err := s.repo.GetUserByID(ctx, userID); err == nil && user != nil {
		return user.Email
	}
	return ""
}

// sendEmail delivers one HTML email through whichever provider is configured:
// Resend's HTTP API when it has a key, SMTP otherwise.
//
// Shared by the import notification, the test email and deadline reminders,
// which used to carry their own copies of both paths.
func (s *Service) sendEmail(ctx context.Context, to, subject, htmlBody string) error {
	switch {
	case s.cfg.ResendAPIKey != "":
		return s.sendViaResend(ctx, to, subject, htmlBody)
	case s.cfg.SMTPHost != "":
		return s.sendViaSMTP(to, subject, htmlBody)
	}
	return errNoMailer
}

func (s *Service) sendViaResend(ctx context.Context, to, subject, htmlBody string) error {
	// Marshalled rather than formatted: an address or subject with a quote in
	// it would otherwise produce invalid JSON.
	payload, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HTML    string   `json:"html"`
	}{s.cfg.ResendFromEmail, []string{to}, subject, htmlBody})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.cfg.ResendAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, string(errBody))
	}
	return nil
}

func (s *Service) sendViaSMTP(to, subject, htmlBody string) error {
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	msg := []byte("From: " + s.cfg.SMTPUser + "\r\n" +
		"To: " + to + "\r\n" +
		// Encoded so a subject outside ASCII survives the trip.
		"Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n" +
		htmlBody)
	return smtp.SendMail(s.cfg.SMTPHost+":"+s.cfg.SMTPPort, auth, s.cfg.SMTPUser, []string{to}, msg)
}
