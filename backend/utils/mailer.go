package utils

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

type smtpConfig struct {
	host       string
	port       string
	username   string
	password   string
	encryption string
	from       mail.Address
}

func loadSMTPConfig() (smtpConfig, error) {
	if mailer := strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_MAILER"))); mailer != "smtp" {
		return smtpConfig{}, errors.New("MAIL_MAILER must be smtp")
	}
	host := strings.TrimSpace(os.Getenv("MAIL_HOST"))
	port := strings.TrimSpace(os.Getenv("MAIL_PORT"))
	username := strings.TrimSpace(os.Getenv("MAIL_USERNAME"))
	password := strings.TrimSpace(os.Getenv("MAIL_PASSWORD"))
	fromAddress := strings.TrimSpace(os.Getenv("MAIL_FROM_ADDRESS"))
	parsedFrom, err := mail.ParseAddress(fromAddress)
	if err != nil || parsedFrom.Address == "" {
		return smtpConfig{}, errors.New("MAIL_FROM_ADDRESS is invalid")
	}
	if host == "" || username == "" || password == "" {
		return smtpConfig{}, errors.New("MAIL_HOST, MAIL_USERNAME, and MAIL_PASSWORD are required")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return smtpConfig{}, errors.New("MAIL_PORT must be numeric")
	}
	encryption := strings.ToLower(strings.TrimSpace(os.Getenv("MAIL_ENCRYPTION")))
	switch encryption {
	case "tls", "starttls", "ssl", "smtps", "none":
	default:
		return smtpConfig{}, errors.New("MAIL_ENCRYPTION must be tls, starttls, ssl, smtps, or none")
	}
	if strings.EqualFold(os.Getenv("APP_ENV"), "production") && encryption == "none" {
		return smtpConfig{}, errors.New("unencrypted SMTP is not allowed in production")
	}
	fromName := strings.TrimSpace(os.Getenv("MAIL_FROM_NAME"))
	if fromName == "" {
		return smtpConfig{}, errors.New("MAIL_FROM_NAME is required")
	}
	return smtpConfig{
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		encryption: encryption,
		from:       mail.Address{Name: fromName, Address: parsedFrom.Address},
	}, nil
}

func applicationName() string {
	return strings.TrimSpace(os.Getenv("APP_NAME"))
}

// SendVerificationEmail mengirim OTP secara sinkron. Untuk trafik produksi
// berskala besar, fungsi ini dapat diganti queue worker tanpa mengubah handler.
func SendVerificationEmail(recipient, recipientName, code string, expiresIn time.Duration) error {
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || to.Address == "" {
		return errors.New("recipient email is invalid")
	}
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	message := buildVerificationMessage(config.from, *to, recipientName, code, expiresIn)
	return sendSMTP(config, to.Address, message)
}

func buildVerificationMessage(from, to mail.Address, recipientName, code string, expiresIn time.Duration) []byte {
	appName := applicationName()
	safeName := html.EscapeString(strings.TrimSpace(recipientName))
	if safeName == "" {
		safeName = "Anggota"
	}
	minutes := int(expiresIn.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	boundary := "ipnu-ippnu-id-verification"
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", "Kode verifikasi "+appName))
	message.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	fmt.Fprintf(&message, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n", boundary)
	fmt.Fprintf(&message, "Halo %s,\r\n\r\nKode verifikasi %s Anda: %s\r\nKode berlaku selama %d menit. Jangan berikan kode ini kepada siapa pun.\r\n", strings.TrimSpace(recipientName), appName, code, minutes)
	fmt.Fprintf(&message, "\r\n--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", boundary)
	fmt.Fprintf(&message, "<!doctype html><html><body style=\"font-family:Arial,sans-serif;color:#17221b\"><p>Halo %s,</p><p>Gunakan kode berikut untuk memverifikasi akun <strong>%s</strong>:</p><p style=\"font-size:32px;font-weight:700;letter-spacing:8px\">%s</p><p>Kode berlaku selama %d menit. Jangan berikan kode ini kepada siapa pun.</p></body></html>\r\n", safeName, html.EscapeString(appName), html.EscapeString(code), minutes)
	fmt.Fprintf(&message, "--%s--\r\n", boundary)
	return message.Bytes()
}

func sendSMTP(config smtpConfig, recipient string, message []byte) error {
	address := net.JoinHostPort(config.host, config.port)
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.host}
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var client *smtp.Client
	if config.encryption == "ssl" || config.encryption == "smtps" {
		connection, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		if err != nil {
			return fmt.Errorf("connect SMTP TLS: %w", err)
		}
		client, err = smtp.NewClient(connection, config.host)
		if err != nil {
			_ = connection.Close()
			return fmt.Errorf("initialize SMTP: %w", err)
		}
	} else {
		connection, err := dialer.Dial("tcp", address)
		if err != nil {
			return fmt.Errorf("connect SMTP: %w", err)
		}
		client, err = smtp.NewClient(connection, config.host)
		if err != nil {
			_ = connection.Close()
			return fmt.Errorf("initialize SMTP: %w", err)
		}
		if config.encryption == "tls" || config.encryption == "starttls" {
			if err := client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				return fmt.Errorf("start SMTP TLS: %w", err)
			}
		}
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", config.username, config.password, config.host)); err != nil {
		return fmt.Errorf("authenticate SMTP: %w", err)
	}
	if err := client.Mail(config.from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	// Setelah DATA diterima dan writer ditutup tanpa error, server sudah
	// menerima pesan. Kegagalan QUIT tidak boleh membuat OTP yang terkirim
	// dianggap gagal lalu dibatalkan di database.
	_ = client.Quit()
	return nil
}
