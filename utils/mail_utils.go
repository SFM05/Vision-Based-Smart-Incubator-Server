package utils

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"gopkg.in/gomail.v2"
)

func SendAlert(uuid string, subject string, body string) {
	const smtpHost = "smtp.qq.com"
	const smtpPort = 465
	from := os.Getenv("SRC_EMAIL")
	to := os.Getenv("DEST_EMAIL")
	authcode := os.Getenv("AUTHCODE")

	loc, _ := time.LoadLocation("Asia/Shanghai")
	timestamp := time.Now().In(loc)
	msg := fmt.Sprintf("您的培养皿（UUID：%s）于 %s %s", uuid, timestamp.Format("2006年01月02日 15:04:05"), body)

	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	// 直接设置中文主题，gomail 会自动进行 RFC 2047 编码
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", msg)

	d := gomail.NewDialer(smtpHost, smtpPort, from, authcode)
	err := d.DialAndSend(m)
	if err != nil {
		slog.Error(fmt.Sprintf("Send email error: %v.", err))
		return
	}
	slog.Info(fmt.Sprintf("Send email success."))

	// // 1. 构造邮件头（必须包含From/To/Subject）
	// header := make(map[string]string)
	// header["From"] = from
	// header["To"] = to
	// header["Subject"] = fmt.Sprintf("=?UTF-8?B?%s?=", subject) // 解决中文乱码

	// loc, _ := time.LoadLocation("Asia/Shanghai")
	// timestamp := time.Now().In(loc)
	// body = fmt.Sprintf("您的培养皿（UUID：%s）于 %s %s", uuid, timestamp.Format("2006年01月02日 15:04:05"), body)

	// // 2. 拼接邮件正文
	// msg := ""
	// for k, v := range header {
	// 	msg += fmt.Sprintf("%s: %s\r\n", k, v)
	// }
	// msg += "\r\n" + body

	// // 3. 建立SSL连接（端口465必须用TLS）
	// conn, err := tls.Dial("tcp", smtpHost+":"+smtpPort, &tls.Config{
	// 	ServerName: smtpHost,
	// })
	// if err != nil {
	// 	slog.Error(fmt.Sprintf("连接失败: %v", err))
	// 	return
	// }
	// defer conn.Close()

	// client, err := smtp.NewClient(conn, smtpHost)
	// if err != nil {
	// 	slog.Error(fmt.Sprintf("创建客户端失败: %v", err))
	// 	return
	// }
	// defer client.Quit()

	// // 4. 登录认证
	// auth := smtp.PlainAuth("", from, authcode, smtpHost)
	// err = client.Auth(auth)
	// if err != nil {
	// 	slog.Error(fmt.Sprintf("Auth failed: %v", err))
	// }

	// // 5. 设置发件人和收件人
	// if err = client.Mail(from); err != nil {
	// 	return
	// }
	// if err = client.Rcpt(to); err != nil {
	// 	return
	// }

	// // 6. 写入邮件数据
	// w, err := client.Data()
	// if err != nil {
	// 	return
	// }
	// _, err = w.Write([]byte(msg))
	// if err != nil {
	// 	return
	// }
	// err = w.Close()
}
