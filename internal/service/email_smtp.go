package service

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

type EmailSMTPConfig struct {
	Enabled   bool
	Host      string
	Port      int
	UseSSL    bool
	Username  string
	AuthCode  string
	FromEmail string
	FromName  string
}

func (c EmailSMTPConfig) Ready() bool {
	if !c.Enabled {
		return false
	}
	return strings.TrimSpace(c.Host) != "" &&
		c.Port > 0 &&
		c.Port <= 65535 &&
		strings.TrimSpace(c.Username) != "" &&
		strings.TrimSpace(c.AuthCode) != "" &&
		strings.TrimSpace(c.FromEmail) != ""
}

func SendSMTPMail(cfg EmailSMTPConfig, to, subject, content string) error {
	if !cfg.Ready() {
		return fmt.Errorf("email smtp is not configured")
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("email recipient is required")
	}

	host := strings.TrimSpace(cfg.Host)
	port := cfg.Port
	addr := fmt.Sprintf("%s:%d", host, port)
	from := strings.TrimSpace(cfg.FromEmail)
	username := strings.TrimSpace(cfg.Username)
	password := strings.TrimSpace(cfg.AuthCode)

	fromName := strings.TrimSpace(cfg.FromName)
	if fromName == "" {
		fromName = "chatgpt2api"
	}
	mimeMessage := strings.Join([]string{
		fmt.Sprintf("From: %s <%s>", fromName, from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		content,
	}, "\r\n")

	auth := smtp.PlainAuth("", username, password, host)
	if cfg.UseSSL {
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		})
		if err != nil {
			return err
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer client.Close()

		if err := client.Auth(auth); err != nil {
			return err
		}
		if err := client.Mail(from); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		wc, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := wc.Write([]byte(mimeMessage)); err != nil {
			_ = wc.Close()
			return err
		}
		if err := wc.Close(); err != nil {
			return err
		}
		return client.Quit()
	}

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(mimeMessage))
}
