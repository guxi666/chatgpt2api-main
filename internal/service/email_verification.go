package service

import (
	"crypto/rand"
	"crypto/tls"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"chatgpt2api/internal/config"
	"chatgpt2api/internal/util"
)

const (
	registerCodeTTL         = 10 * time.Minute
	registerCodeCooldown    = 60 * time.Second
	registerCodeDailyLimit  = 20
	registerCodeMaxAttempts = 8
)

type emailVerificationRecord struct {
	CodeHash     string
	ExpiresAt    time.Time
	LastSentAt   time.Time
	WindowStart  time.Time
	RequestCount int
	Attempts     int
}

type EmailVerificationService struct {
	mu      sync.Mutex
	smtpCfg config.EmailSMTPConfig
	records map[string]emailVerificationRecord
}

func NewEmailVerificationService(cfg config.EmailSMTPConfig) *EmailVerificationService {
	return &EmailVerificationService{
		smtpCfg: cfg,
		records: map[string]emailVerificationRecord{},
	}
}

func (s *EmailVerificationService) Enabled() bool {
	return s.smtpCfg.Enabled &&
		strings.TrimSpace(s.smtpCfg.Host) != "" &&
		s.smtpCfg.Port > 0 &&
		strings.TrimSpace(s.smtpCfg.FromEmail) != ""
}

func (s *EmailVerificationService) SendCode(email string) error {
	if !s.Enabled() {
		return errors.New("email verification is not configured")
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("email is required")
	}
	now := time.Now()

	s.mu.Lock()
	s.cleanupLocked(now)
	record := s.records[email]
	if !record.LastSentAt.IsZero() {
		elapsed := now.Sub(record.LastSentAt)
		if elapsed < registerCodeCooldown {
			wait := int((registerCodeCooldown - elapsed + time.Second - 1) / time.Second)
			s.mu.Unlock()
			return fmt.Errorf("please retry after %d seconds", wait)
		}
	}
	if record.WindowStart.IsZero() || now.Sub(record.WindowStart) >= 24*time.Hour {
		record.WindowStart = now
		record.RequestCount = 0
	}
	if record.RequestCount >= registerCodeDailyLimit {
		s.mu.Unlock()
		return errors.New("too many verification requests today")
	}
	s.mu.Unlock()

	code, err := randomNumericCode(6)
	if err != nil {
		return err
	}
	if err := s.sendCodeMail(email, code); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record = s.records[email]
	record.CodeHash = util.SHA256Hex(code)
	record.ExpiresAt = now.Add(registerCodeTTL)
	record.LastSentAt = now
	if record.WindowStart.IsZero() || now.Sub(record.WindowStart) >= 24*time.Hour {
		record.WindowStart = now
		record.RequestCount = 0
	}
	record.RequestCount++
	record.Attempts = 0
	s.records[email] = record
	return nil
}

func (s *EmailVerificationService) VerifyCode(email, code string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return errors.New("email and verification code are required")
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	record, ok := s.records[email]
	if !ok || record.CodeHash == "" {
		return errors.New("verification code is not sent")
	}
	if now.After(record.ExpiresAt) {
		delete(s.records, email)
		return errors.New("verification code is expired")
	}
	if record.Attempts >= registerCodeMaxAttempts {
		delete(s.records, email)
		return errors.New("verification code has too many failed attempts")
	}
	if util.SHA256Hex(code) != record.CodeHash {
		record.Attempts++
		s.records[email] = record
		return errors.New("verification code is invalid")
	}
	delete(s.records, email)
	return nil
}

func (s *EmailVerificationService) cleanupLocked(now time.Time) {
	for email, record := range s.records {
		if record.CodeHash == "" || now.After(record.ExpiresAt.Add(30*time.Minute)) {
			delete(s.records, email)
		}
	}
}

func randomNumericCode(length int) (string, error) {
	if length < 4 {
		length = 4
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, length)
	for i := range buf {
		out[i] = byte('0' + (buf[i] % 10))
	}
	return string(out), nil
}

func (s *EmailVerificationService) sendCodeMail(toEmail, code string) error {
	cfg := s.smtpCfg
	host := strings.TrimSpace(cfg.Host)
	port := cfg.Port
	fromEmail := strings.TrimSpace(cfg.FromEmail)
	fromName := strings.TrimSpace(cfg.FromName)
	if fromName == "" {
		fromName = "chatgpt2api"
	}
	subject := "邮箱验证码"
	body := fmt.Sprintf("您的验证码是：%s，10分钟内有效。", code)
	message := buildSMTPMessage(fromName, fromEmail, toEmail, subject, body)
	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.Auth(nil)
	if strings.TrimSpace(cfg.Username) != "" && strings.TrimSpace(cfg.Password) != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, host)
	}
	if cfg.UseSSL {
		tlsConn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
		if err != nil {
			return err
		}
		defer tlsConn.Close()
		client, err := smtp.NewClient(tlsConn, host)
		if err != nil {
			return err
		}
		defer client.Close()
		if auth != nil {
			if ok, _ := client.Extension("AUTH"); ok {
				if err := client.Auth(auth); err != nil {
					return err
				}
			}
		}
		if err := client.Mail(fromEmail); err != nil {
			return err
		}
		if err := client.Rcpt(toEmail); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(message)); err != nil {
			_ = w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return client.Quit()
	}
	return smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(message))
}

func buildSMTPMessage(fromName, fromEmail, toEmail, subject, body string) string {
	return strings.Join([]string{
		fmt.Sprintf("From: %s <%s>", fromName, fromEmail),
		fmt.Sprintf("To: %s", toEmail),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
}

