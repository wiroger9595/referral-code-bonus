// Package mailer 負責把信送出去，不管信裡寫什麼 —— 內容由呼叫端組好再交進來。
//
// 兩種實作：本機開發預設用標準庫 net/smtp（不用先架一台 SMTP，留空就退回印 log）；
// 設了 RESEND_API_KEY 的話優先走 Resend 的 HTTP API。呼叫端只認 Mailer 介面，不用管是哪一種。
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
)

type Message struct {
	To      string
	Subject string
	Body    string // 純文字。收信端的 HTML 支援落差太大，驗證碼這種信不值得為此冒版型風險。
}

type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string

	// Resend。設了就優先於 SMTP —— 兩邊都設的話沒道理跑本機 SMTP 那條慢路。
	ResendAPIKey string
}

// New 依序挑實作：Resend > SMTP > 印進 log。本機開發兩個都不設也能跑，
// 正式環境兩個都不設會在啟動時被 config.Load 擋下來。
func New(cfg Config) Mailer {
	if cfg.ResendAPIKey != "" {
		return &resendMailer{
			client:   resend.NewClient(cfg.ResendAPIKey),
			from:     cfg.From,
			fromName: cfg.FromName,
		}
	}
	if cfg.Host == "" {
		slog.Warn("SMTP_HOST 與 RESEND_API_KEY 都未設定，信件只會印進 log 不會寄出")
		return logMailer{}
	}
	return &smtpMailer{cfg: cfg}
}

type resendMailer struct {
	client   *resend.Client
	from     string
	fromName string
}

func (m *resendMailer) Send(ctx context.Context, msg Message) error {
	from := m.from
	if m.fromName != "" {
		// Resend 吃的是 JSON API，UTF-8 字串直接放就好，不像 SMTP headers 要 encoded-word。
		from = fmt.Sprintf("%s <%s>", m.fromName, m.from)
	}

	_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		Text:    msg.Body,
	})
	if err != nil {
		return fmt.Errorf("Resend 寄信: %w", err)
	}
	return nil
}

type logMailer struct{}

func (logMailer) Send(_ context.Context, msg Message) error {
	slog.Info("（未寄出）信件內容", "to", msg.To, "subject", msg.Subject, "body", msg.Body)
	return nil
}

type smtpMailer struct {
	cfg Config
}

func (m *smtpMailer) Send(ctx context.Context, msg Message) error {
	addr := net.JoinHostPort(m.cfg.Host, fmt.Sprint(m.cfg.Port))

	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("連線 SMTP: %w", err)
	}

	// 465 是 implicit TLS（連上就要先握手），587 與 25 是先明文再 STARTTLS。
	if m.cfg.Port == 465 {
		conn = tls.Client(conn, &tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12})
	}

	c, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("建立 SMTP client: %w", err)
	}
	defer c.Close()

	if m.cfg.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				return fmt.Errorf("STARTTLS: %w", err)
			}
		}
	}

	// 本機的 Mailpit / MailHog 不需要帳密，有設才送 AUTH，否則會被回 502。
	if m.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("SMTP 認證失敗: %w", err)
		}
	}

	if err := c.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := c.Rcpt(msg.To); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write([]byte(m.compose(msg))); err != nil {
		return fmt.Errorf("寫入信件內容: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("結束 DATA: %w", err)
	}
	return c.Quit()
}

func (m *smtpMailer) compose(msg Message) string {
	from := m.cfg.From
	if m.cfg.FromName != "" {
		// 標題與寄件人名稱都是中文，一定要 encoded-word，直接塞會變亂碼。
		from = fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("UTF-8", m.cfg.FromName), m.cfg.From)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	// 行首單獨一個點會被 SMTP 當成內容結束，標準的做法是補成兩個點。
	b.WriteString(strings.ReplaceAll(msg.Body, "\r\n.", "\r\n.."))
	b.WriteString("\r\n")
	return b.String()
}
