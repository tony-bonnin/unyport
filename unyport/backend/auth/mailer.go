package auth

import (
	"log/slog"
	"net/textproto"
	"os/exec"
	"strings"
	"time"

	"unyport/config"
)

type Mailer struct {
	enabled  bool
	from     string
	sendmail string
	logger   *slog.Logger
}

func NewMailer(settings *config.Settings, logger *slog.Logger) *Mailer {
	m := settings.Mail
	from := strings.TrimSpace(m.From)
	if from == "" {
		from = strings.TrimSpace(m.Username)
	}
	sendmailPath := strings.TrimSpace(m.SendmailPath)
	if sendmailPath == "" {
		sendmailPath = "/usr/sbin/sendmail"
	}
	return &Mailer{
		enabled:  m.Enabled,
		from:     from,
		sendmail: sendmailPath,
		logger:   logger,
	}
}

func (m *Mailer) Enabled() bool {
	return m != nil && m.enabled && m.from != ""
}

func (m *Mailer) SendLoginNotification(to, username, ip string, ts time.Time) {
	if !m.Enabled() || strings.TrimSpace(to) == "" {
		return
	}
	go func() {
		if err := m.sendLoginNotification(to, username, ip, ts); err != nil {
			m.logger.Warn("login notification email failed", "to", to, "err", err)
		}
	}()
}

func (m *Mailer) sendLoginNotification(to, username, ip string, ts time.Time) error {
	subject := "UnyPort login notification"
	body := strings.Join([]string{
		"UnyPort login detected.",
		"",
		"Username: " + username,
		"Email: " + to,
		"IP: " + ip,
		"Timestamp: " + ts.Format(time.RFC3339),
		"",
		"If this was not you, change your password immediately.",
	}, "\r\n")

	headers := textproto.MIMEHeader{}
	headers.Set("From", m.from)
	headers.Set("To", to)
	headers.Set("Subject", subject)
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", "text/plain; charset=utf-8")

	var msg strings.Builder
	for k, vals := range headers {
		for _, v := range vals {
			msg.WriteString(k)
			msg.WriteString(": ")
			msg.WriteString(v)
			msg.WriteString("\r\n")
		}
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")

	cmd := exec.Command(m.sendmail, "-t", "-i")
	cmd.Stdin = strings.NewReader(msg.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		if len(out) > 0 {
			return errWithOutput(err, string(out))
		}
		return err
	}
	return nil
}

func errWithOutput(err error, out string) error {
	return &mailSendError{err: err, out: strings.TrimSpace(out)}
}

type mailSendError struct {
	err error
	out string
}

func (e *mailSendError) Error() string {
	return e.err.Error() + ": " + e.out
}
