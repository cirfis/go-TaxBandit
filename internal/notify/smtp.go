package notify

import (
	"fmt"
	"log"
	"net/smtp"
)

type Notifier struct {
	Host     string
	Port     string
	User     string
	Pass     string
	Receiver string
}

func NewNotifier(host, port, user, pass, receiver string) *Notifier {
	return &Notifier{
		Host:     host,
		Port:     port,
		User:     user,
		Pass:     pass,
		Receiver: receiver,
	}
}

func (n *Notifier) Emit(urgency, subject, body string) {
	prefix := "[NORMAL]"
	if urgency == "CRITICAL" {
		prefix = "[CRITICAL ERROR]"
	}
	log.Printf("%s %s\n%s\n", prefix, subject, body)

	if n.Host == "" || n.User == "" || n.Pass == "" || n.Receiver == "" {
		log.Println("[WARN] SMTP disabled. Output restricted to stdout.")
		return
	}

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: [WH-COMPLIANCE] %s\r\nMIME-version: 1.0;\r\nContent-Type: text/plain; charset=\"UTF-8\";\r\n\r\n%s\r\n",
		n.User, n.Receiver, subject, body))

	auth := smtp.PlainAuth("", n.User, n.Pass, n.Host)
	err := smtp.SendMail(n.Host+":"+n.Port, auth, n.User, []string{n.Receiver}, msg)
	if err != nil {
		log.Printf("[ERROR] SMTP Transit Failure: %v\n", err)
	}
}
