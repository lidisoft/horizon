// Package mail sends plain-text email through the SMTP server in
// HORIZON_SMTP_URL (smtp:// with STARTTLS when offered, smtps:// for implicit
// TLS).
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Message is one plain-text email.
type Message struct {
	To      *mail.Address
	Subject string
	Body    string
}

// Sender sends messages. Modules depend on this interface; tests use Memory.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// SMTP sends through an SMTP server.
type SMTP struct {
	url  *url.URL
	from *mail.Address
}

// NewSMTP returns a sender for an smtp:// or smtps:// URL with optional
// user:password credentials.
func NewSMTP(u *url.URL, from *mail.Address) *SMTP {
	return &SMTP{url: u, from: from}
}

// timeout bounds a whole delivery.
const timeout = 30 * time.Second

// Send delivers m, honouring ctx cancellation.
func (s *SMTP) Send(ctx context.Context, m Message) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	host := s.url.Hostname()
	port := s.url.Port()
	if port == "" {
		port = "587"
		if s.url.Scheme == "smtps" {
			port = "465"
		}
	}
	addr := net.JoinHostPort(host, port)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if s.url.Scheme == "smtps" {
		conn = tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("mail: %s: %w", addr, err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); ok && s.url.Scheme == "smtp" {
		if err := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("mail: starttls: %w", err)
		}
	}
	if user := s.url.User; user != nil {
		pass, _ := user.Password()
		if err := c.Auth(smtp.PlainAuth("", user.Username(), pass, host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := c.Rcpt(m.To.Address); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := w.Write(Render(s.from, m, time.Now())); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mail: send: %w", err)
	}
	return c.Quit()
}

// Render builds the RFC 5322 message: UTF-8 plain text, quoted-printable.
func Render(from *mail.Address, m Message, now time.Time) []byte {
	var b bytes.Buffer
	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := from.Address[strings.LastIndexByte(from.Address, '@')+1:]

	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", m.To.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(strings.ReplaceAll(m.Body, "\n", "\r\n")))
	_ = qp.Close()
	return b.Bytes()
}

// Memory records messages instead of sending them, for tests.
type Memory struct {
	mu   sync.Mutex
	sent []Message
}

// Send records m.
func (s *Memory) Send(_ context.Context, m Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, m)
	return nil
}

// Sent returns the messages sent so far.
func (s *Memory) Sent() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.sent...)
}
