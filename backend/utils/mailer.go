package utils

import (
	"bytes"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"

	"sso-backend/internal/apptime"
)

const emailLogoContentID = "pelajarnu-magetan-id-sso-logo"

//go:embed assets/logo-sso-email.png
var emailLogoPNG []byte

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

// SendVerificationEmail menjalankan satu percobaan SMTP. Handler API menaruh
// pekerjaan pada outbox; worker memanggil fungsi ini dan mengatur retry.
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
	minutes := int(expiresIn.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	plain := fmt.Sprintf(
		"%s\r\n\r\nGunakan kode berikut untuk memverifikasi akun %s Anda:\r\n\r\n%s\r\n\r\nKode berlaku selama %d menit dan hanya dapat digunakan satu kali. Jangan berikan kode ini kepada siapa pun.\r\n\r\nJika Anda tidak membuat akun ini, abaikan email ini.",
		plainGreeting(recipientName), appName, code, minutes,
	)
	body := fmt.Sprintf(`
		%s
		<p style="margin:0 0 22px;color:#46534b;font-size:15px;line-height:24px;">Gunakan kode berikut untuk memverifikasi akun <strong style="color:#17221b;">%s</strong> Anda.</p>
		<div style="margin:0 0 22px;padding:18px 16px;border:1px solid #cfe3d5;border-radius:12px;background:#eef8f1;color:#005c2d;text-align:center;font-family:Arial,'Segoe UI',sans-serif;font-size:32px;font-weight:700;letter-spacing:8px;line-height:40px;">%s</div>
		<p style="margin:0 0 22px;color:#46534b;font-size:14px;line-height:22px;">Kode berlaku selama <strong style="color:#17221b;">%d menit</strong> dan hanya dapat digunakan satu kali.</p>
		%s`,
		htmlGreeting(recipientName), html.EscapeString(appName), html.EscapeString(code), minutes,
		securityNote("Jangan berikan kode ini kepada siapa pun. Jika Anda tidak membuat akun ini, abaikan email ini."),
	)
	htmlBody := brandedEmailHTML("Verifikasi email Anda", "Kode verifikasi akun PelajarNU Magetan ID", body)
	return buildBrandedMessage(from, to, "Kode verifikasi "+appName, plain, htmlBody)
}

// SendPasswordResetEmail mengirim tautan reset satu kali melalui antrean
// persisten. Fungsi ini hanya melakukan satu percobaan SMTP.
func SendPasswordResetEmail(recipient, recipientName, resetURL string, expiresIn time.Duration) error {
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || to.Address == "" {
		return errors.New("recipient email is invalid")
	}
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	message := buildPasswordResetMessage(config.from, *to, recipientName, resetURL, expiresIn)
	return sendSMTP(config, to.Address, message)
}

func buildPasswordResetMessage(from, to mail.Address, recipientName, resetURL string, expiresIn time.Duration) []byte {
	appName := applicationName()
	minutes := int(expiresIn.Round(time.Minute) / time.Minute)
	if minutes < 1 {
		minutes = 1
	}
	plain := fmt.Sprintf(
		"%s\r\n\r\nKami menerima permintaan untuk mengatur ulang kata sandi akun %s Anda.\r\n\r\nBuka tautan berikut:\r\n%s\r\n\r\nTautan berlaku selama %d menit dan hanya dapat digunakan satu kali.\r\n\r\nJika Anda tidak meminta perubahan ini, abaikan email ini. Kata sandi Anda tetap aman dan tidak berubah.",
		plainGreeting(recipientName), appName, resetURL, minutes,
	)
	safeURL := html.EscapeString(resetURL)
	body := fmt.Sprintf(`
		%s
		<p style="margin:0 0 24px;color:#46534b;font-size:15px;line-height:24px;">Kami menerima permintaan untuk mengatur ulang kata sandi akun <strong style="color:#17221b;">%s</strong> Anda.</p>
		<table role="presentation" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 24px;"><tr><td style="border-radius:10px;background:#007a3d;"><a href="%s" style="display:inline-block;padding:13px 20px;color:#ffffff;font-family:Arial,'Segoe UI',sans-serif;font-size:15px;font-weight:700;line-height:20px;text-decoration:none;">Atur ulang kata sandi</a></td></tr></table>
		<p style="margin:0 0 8px;color:#66736b;font-size:12px;line-height:19px;">Jika tombol tidak dapat dibuka, salin tautan berikut:</p>
		<p style="margin:0 0 22px;word-break:break-all;color:#007a3d;font-size:12px;line-height:19px;"><a href="%s" style="color:#007a3d;text-decoration:underline;">%s</a></p>
		<p style="margin:0 0 22px;color:#46534b;font-size:14px;line-height:22px;">Tautan berlaku selama <strong style="color:#17221b;">%d menit</strong> dan hanya dapat digunakan satu kali.</p>
		%s`,
		htmlGreeting(recipientName), html.EscapeString(appName), safeURL, safeURL, safeURL, minutes,
		securityNote("Jika Anda tidak meminta perubahan ini, abaikan email ini. Kata sandi Anda tetap aman dan tidak berubah."),
	)
	htmlBody := brandedEmailHTML("Atur ulang kata sandi", "Tautan reset kata sandi PelajarNU Magetan ID", body)
	return buildBrandedMessage(from, to, "Atur ulang kata sandi "+appName, plain, htmlBody)
}

// SendPasswordChangedEmail memberi tahu pemilik akun setelah kredensial
// berubah. Email ini tidak membawa token atau tindakan masuk otomatis.
func SendPasswordChangedEmail(recipient, recipientName string, changedAt time.Time, ipAddress string, currentSessionKept bool) error {
	to, err := mail.ParseAddress(strings.TrimSpace(recipient))
	if err != nil || to.Address == "" {
		return errors.New("recipient email is invalid")
	}
	config, err := loadSMTPConfig()
	if err != nil {
		return err
	}
	message := buildPasswordChangedMessage(config.from, *to, recipientName, changedAt, ipAddress, currentSessionKept)
	return sendSMTP(config, to.Address, message)
}

func buildPasswordChangedMessage(from, to mail.Address, recipientName string, changedAt time.Time, ipAddress string, currentSessionKept bool) []byte {
	appName := applicationName()
	when := formatSecurityEmailTime(changedAt)
	ipText := strings.TrimSpace(ipAddress)
	if ipText == "" {
		ipText = "Tidak tersedia"
	}
	sessionMessage := "Seluruh sesi dan akses aplikasi lama telah dicabut. Silakan masuk kembali menggunakan kata sandi baru."
	if currentSessionKept {
		sessionMessage = "Sesi lain dan akses aplikasi lama telah dicabut. Sesi yang digunakan untuk mengubah kata sandi tetap aktif."
	}
	plain := fmt.Sprintf(
		"%s\r\n\r\nKata sandi akun %s Anda telah berhasil diubah.\r\n\r\nWaktu: %s\r\nAlamat IP: %s\r\n\r\n%s\r\n\r\nJika bukan Anda yang melakukan perubahan ini, segera hubungi administrator %s.",
		plainGreeting(recipientName), appName, when, ipText, sessionMessage, appName,
	)
	body := fmt.Sprintf(`
		%s
		<p style="margin:0 0 22px;color:#46534b;font-size:15px;line-height:24px;">Kata sandi akun <strong style="color:#17221b;">%s</strong> Anda telah berhasil diubah.</p>
		<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="margin:0 0 22px;border:1px solid #dce6df;border-radius:12px;background:#ffffff;"><tr><td style="padding:16px 18px;">
			<p style="margin:0 0 7px;color:#66736b;font-size:12px;line-height:18px;">Waktu perubahan</p><p style="margin:0 0 14px;color:#17221b;font-size:14px;font-weight:700;line-height:20px;">%s</p>
			<p style="margin:0 0 7px;color:#66736b;font-size:12px;line-height:18px;">Alamat IP</p><p style="margin:0;color:#17221b;font-family:'SFMono-Regular',Consolas,monospace;font-size:13px;line-height:20px;">%s</p>
		</td></tr></table>
		<p style="margin:0 0 22px;color:#46534b;font-size:14px;line-height:22px;">%s</p>
		%s`,
		htmlGreeting(recipientName), html.EscapeString(appName), html.EscapeString(when), html.EscapeString(ipText), html.EscapeString(sessionMessage),
		securityNote("Jika bukan Anda yang melakukan perubahan ini, segera hubungi administrator PelajarNU Magetan ID."),
	)
	htmlBody := brandedEmailHTML("Kata sandi telah diubah", "Pemberitahuan keamanan akun PelajarNU Magetan ID", body)
	return buildBrandedMessage(from, to, "Kata sandi "+appName+" telah diubah", plain, htmlBody)
}

func plainGreeting(recipientName string) string {
	name := strings.TrimSpace(recipientName)
	if name == "" {
		return "Halo,"
	}
	return "Halo " + name + ","
}

func htmlGreeting(recipientName string) string {
	name := html.EscapeString(strings.TrimSpace(recipientName))
	if name == "" {
		return `<p style="margin:0 0 12px;color:#17221b;font-size:15px;line-height:24px;">Halo,</p>`
	}
	return fmt.Sprintf(`<p style="margin:0 0 12px;color:#17221b;font-size:15px;line-height:24px;">Halo %s,</p>`, name)
}

func securityNote(text string) string {
	return fmt.Sprintf(`<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="border-radius:10px;background:#eef8f1;"><tr><td style="padding:14px 16px;color:#35533f;font-size:12px;line-height:19px;">%s</td></tr></table>`, html.EscapeString(text))
}

func brandedEmailHTML(title, preheader, content string) string {
	appName := html.EscapeString(applicationName())
	return fmt.Sprintf(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head>
<body style="margin:0;padding:0;background:#f4f7f5;color:#17221b;font-family:Arial,'Segoe UI',sans-serif;-webkit-text-size-adjust:100%%;">
<div style="display:none;max-height:0;overflow:hidden;opacity:0;color:transparent;">%s</div>
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="background:#f4f7f5;"><tr><td align="center" style="padding:28px 14px;">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" border="0" style="max-width:580px;border:1px solid #dce6df;border-radius:16px;background:#ffffff;overflow:hidden;">
<tr><td style="padding:22px 26px 18px;"><table role="presentation" cellspacing="0" cellpadding="0" border="0"><tr>
<td style="padding-right:12px;vertical-align:middle;"><img src="cid:%s" width="44" height="45" alt="Logo %s" style="display:block;width:44px;height:45px;object-fit:contain;border:0;"></td>
<td style="vertical-align:middle;"><p style="margin:0;color:#17221b;font-size:16px;font-weight:700;line-height:21px;">%s</p><p style="margin:3px 0 0;color:#66736b;font-size:12px;line-height:17px;">Single Sign-On</p></td>
</tr></table></td></tr>
<tr><td style="height:4px;background:#007a3d;font-size:0;line-height:0;">&nbsp;</td></tr>
<tr><td style="padding:28px 26px 30px;"><h1 style="margin:0 0 20px;color:#17221b;font-size:24px;font-weight:700;letter-spacing:-0.4px;line-height:31px;">%s</h1>%s</td></tr>
<tr><td style="border-top:1px solid #e6ede8;padding:18px 26px;color:#66736b;font-size:11px;line-height:18px;">Email keamanan otomatis dari %s.<br><a href="https://pelajarnumagetan.id" style="color:#007a3d;text-decoration:none;">pelajarnumagetan.id</a></td></tr>
</table></td></tr></table></body></html>`,
		html.EscapeString(title), html.EscapeString(preheader), emailLogoContentID, appName, appName,
		html.EscapeString(title), content, appName,
	)
}

func buildBrandedMessage(from, to mail.Address, subject, plainText, htmlBody string) []byte {
	var alternative bytes.Buffer
	alternativeWriter := multipart.NewWriter(&alternative)
	plainHeaders := textproto.MIMEHeader{
		"Content-Type":              {"text/plain; charset=UTF-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	plainPart, _ := alternativeWriter.CreatePart(plainHeaders)
	plainEncoder := quotedprintable.NewWriter(plainPart)
	_, _ = plainEncoder.Write([]byte(plainText))
	_ = plainEncoder.Close()
	htmlHeaders := textproto.MIMEHeader{
		"Content-Type":              {"text/html; charset=UTF-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}
	htmlPart, _ := alternativeWriter.CreatePart(htmlHeaders)
	htmlEncoder := quotedprintable.NewWriter(htmlPart)
	_, _ = htmlEncoder.Write([]byte(htmlBody))
	_ = htmlEncoder.Close()
	_ = alternativeWriter.Close()

	var body bytes.Buffer
	relatedWriter := multipart.NewWriter(&body)
	alternativeHeaders := textproto.MIMEHeader{
		"Content-Type": {fmt.Sprintf("multipart/alternative; boundary=%q", alternativeWriter.Boundary())},
	}
	alternativePart, _ := relatedWriter.CreatePart(alternativeHeaders)
	_, _ = alternativePart.Write(alternative.Bytes())
	logoHeaders := textproto.MIMEHeader{
		"Content-Type":              {"image/png; name=\"logo-sso.png\""},
		"Content-Transfer-Encoding": {"base64"},
		"Content-ID":                {"<" + emailLogoContentID + ">"},
		"Content-Disposition":       {"inline; filename=\"logo-sso.png\""},
	}
	logoPart, _ := relatedWriter.CreatePart(logoHeaders)
	writeMIMEBase64(logoPart, emailLogoPNG)
	_ = relatedWriter.Close()

	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s\r\n", from.String())
	fmt.Fprintf(&message, "To: %s\r\n", to.String())
	fmt.Fprintf(&message, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", subject))
	fmt.Fprintf(&message, "Date: %s\r\n", apptime.Now().Format(time.RFC1123Z))
	message.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/related; boundary=%q\r\n\r\n", relatedWriter.Boundary())
	_, _ = message.Write(body.Bytes())
	return message.Bytes()
}

func writeMIMEBase64(part interface{ Write([]byte) (int, error) }, value []byte) {
	encoded := make([]byte, base64.StdEncoding.EncodedLen(57))
	for len(value) > 0 {
		length := 57
		if len(value) < length {
			length = len(value)
		}
		chunk := value[:length]
		line := encoded[:base64.StdEncoding.EncodedLen(len(chunk))]
		base64.StdEncoding.Encode(line, chunk)
		_, _ = part.Write(line)
		_, _ = part.Write([]byte("\r\n"))
		value = value[length:]
	}
}

func formatSecurityEmailTime(value time.Time) string {
	if value.IsZero() {
		value = apptime.Now()
	}
	return value.In(apptime.Jakarta).Format("02-01-2006 15:04 WIB")
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
		if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			_ = connection.Close()
			return fmt.Errorf("set SMTP deadline: %w", err)
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
		if err := connection.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
			_ = connection.Close()
			return fmt.Errorf("set SMTP deadline: %w", err)
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
