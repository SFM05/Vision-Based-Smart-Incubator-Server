package utils

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"gopkg.in/gomail.v2"
)

func SendAlert(uuid string, subject string, body string) {
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "smtp.qq.com"
	}
	smtpPort := 465
	if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			smtpPort = p
		}
	}
	from := os.Getenv("SRC_EMAIL")
	to := os.Getenv("DEST_EMAIL")
	authcode := os.Getenv("AUTHCODE")

	if from == "" || to == "" || authcode == "" {
		slog.Error("Email alert skipped: SRC_EMAIL, DEST_EMAIL, or AUTHCODE not set")
		return
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to load Asia/Shanghai timezone, falling back to UTC: %v", err))
		loc = time.UTC
	}
	timestamp := time.Now().In(loc)
	msg := fmt.Sprintf("您的培养皿（UUID：%s）于 %s %s", uuid, timestamp.Format("2006年01月02日 15:04:05"), body)

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", msg)

	d := gomail.NewDialer(smtpHost, smtpPort, from, authcode)
	err = d.DialAndSend(m)
	if err != nil {
		slog.Error(fmt.Sprintf("Send email error: %v", err))
		return
	}
	slog.Info("Send email success.")
}
