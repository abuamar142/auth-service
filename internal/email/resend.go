// Package email sends transactional email through Resend.
//
// Kept small on purpose: this service sends exactly one kind of message, and
// a generic template engine would be more surface than the feature needs.
package email

import (
	"fmt"
	"html"
	"log/slog"

	"github.com/resend/resend-go/v2"
)

// ResendSender sends password-reset mail through Resend.
type ResendSender struct {
	client    *resend.Client
	fromEmail string
	fromName  string
	// resetURL is the frontend page that accepts a token, e.g.
	// https://cafe.abuamar.online/auth/reset-password. Empty falls back to
	// printing the token in the body, which is what a dev environment wants.
	resetURL string
}

func NewResendSender(apiKey, fromEmail, fromName, resetURL string) *ResendSender {
	return &ResendSender{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
		resetURL:  resetURL,
	}
}

// SendPasswordReset delivers the reset token.
//
// The link carries the token in the query string rather than the body: the
// page needs it to call the API, and there is no session to look it up from.
// That does mean the token can land in browser history and referrer headers,
// which is why it is single-use and expires in an hour.
func (s *ResendSender) SendPasswordReset(to, token string) error {
	link := ""
	if s.resetURL != "" {
		link = fmt.Sprintf("%s?token=%s", s.resetURL, token)
	}

	subject := "Reset password"
	text := s.plainText(token, link)
	htmlBody := s.htmlBody(token, link)

	params := &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
		Text:    text,
	}

	if _, err := s.client.Emails.Send(params); err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	slog.Info("password reset email sent")
	return nil
}

func (s *ResendSender) plainText(token, link string) string {
	if link == "" {
		return fmt.Sprintf(
			"Kami menerima permintaan reset password untuk akun ini.\n\n"+
				"Gunakan kode berikut:\n\n%s\n\n"+
				"Kode ini berlaku 1 jam dan hanya bisa dipakai sekali.\n"+
				"Kalau kamu tidak meminta reset, abaikan email ini — password tidak berubah.",
			token,
		)
	}
	return fmt.Sprintf(
		"Kami menerima permintaan reset password untuk akun ini.\n\n"+
			"Buka tautan berikut untuk membuat password baru:\n\n%s\n\n"+
			"Tautan ini berlaku 1 jam dan hanya bisa dipakai sekali.\n"+
			"Kalau kamu tidak meminta reset, abaikan email ini — password tidak berubah.",
		link,
	)
}

func (s *ResendSender) htmlBody(token, link string) string {
	action := fmt.Sprintf(
		`<p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#3f3f46">
			Kami menerima permintaan reset password untuk akun ini.
		</p>`,
	)
	if link != "" {
		action += fmt.Sprintf(
			`<p style="margin:0 0 24px">
				<a href="%s" style="display:inline-block;padding:12px 24px;background:#18181b;color:#fafafa;text-decoration:none;border-radius:6px;font-size:15px">Buat password baru</a>
			</p>
			<p style="margin:0 0 16px;font-size:13px;line-height:1.6;color:#71717a">
				Atau salin tautan ini ke browser:<br>
				<span style="word-break:break-all">%s</span>
			</p>`,
			html.EscapeString(link), html.EscapeString(link),
		)
	} else {
		// No frontend URL configured: show the code itself.
		action += fmt.Sprintf(
			`<p style="margin:0 0 16px;font-size:13px;color:#71717a">Gunakan kode berikut:</p>
			<p style="margin:0 0 24px;font-family:ui-monospace,monospace;font-size:18px;font-weight:600;color:#18181b;word-break:break-all">%s</p>`,
			html.EscapeString(token),
		)
	}

	return fmt.Sprintf(`<!doctype html>
<html lang="id">
<body style="margin:0;padding:32px 16px;background:#f4f4f5;font-family:ui-sans-serif,system-ui,-apple-system,sans-serif">
	<div style="max-width:480px;margin:0 auto;padding:32px;background:#ffffff;border-radius:8px">
		<h1 style="margin:0 0 16px;font-size:20px;font-weight:600;color:#18181b">Reset password</h1>
		%s
		<p style="margin:0;font-size:13px;line-height:1.6;color:#71717a">
			Berlaku 1 jam dan hanya bisa dipakai sekali. Kalau kamu tidak meminta
			reset, abaikan email ini — password tidak berubah.
		</p>
	</div>
</body>
</html>`, action)
}
