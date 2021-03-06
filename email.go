package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
)

type EmailManager struct {
	mail_queue    chan *EmailMessage
	server_config *Config
}

type EmailMessage struct {
	From    string
	To      string
	Subject string
	Body    string
}

func NewEmailManager(server_config *Config) *EmailManager {
	t := new(EmailManager)
	t.server_config = server_config

	// Create a mail queue, max length is set by config file.
	t.mail_queue = make(chan *EmailMessage, server_config.Mail.QueueSize)

	go t.process_mail_queue()

	return t
}

// Create a message struct, populate it with the message details and queue it to be sent.
func (t *EmailManager) QueueEmail(to, from, subject, body string) {
	message := new(EmailMessage)
	message.To = to
	message.From = from
	message.Subject = subject
	message.Body = body
	t.mail_queue <- message
}

// Listen on the channel of email messages, and send them using the server
// set in the application settings.
func (t *EmailManager) process_mail_queue() {
	for {
		// Block until we get an email to send
		message := <-t.mail_queue

		host := fmt.Sprintf("%s:%d", t.server_config.Mail.Host, t.server_config.Mail.Port)

		log.Println("Sending message from", message.From, "to", message.To, "body", message.Body)

		// Connect to the remote SMTP server.
		var conn net.Conn
		var err error
		if t.server_config.Mail.TLS {
			conf := tls.Config{InsecureSkipVerify: true}
			conn, err = tls.Dial("tcp", host, &conf)
		} else {
			conn, err = net.Dial("tcp", host)
		}
		if err != nil {
			log.Println("Mail: Dial:", err)
			continue
		}

		client, err := smtp.NewClient(conn, host)
		if err != nil {
			log.Println("Mail: NewClient:", err)
			continue
		}

		// If authentication was enabled in the config file, load it.
		if t.server_config.Mail.Auth {
			auth := smtp.PlainAuth(
				"",
				t.server_config.Mail.User,
				t.server_config.Mail.Password,
				t.server_config.Mail.Host,
			)
			if err := client.Auth(auth); err != nil {
				log.Println("Mail: Auth:", err)
				continue
			}
		}

		// Set the sender and recipient first
		if err := client.Mail(message.From); err != nil {
			log.Println("Mail: Mail:", err)
			continue
		}
		if err := client.Rcpt(message.To); err != nil {
			log.Println("Mail: Rcpt:", err)
			continue
		}

		// Send the email body.
		wc, err := client.Data()
		if err != nil {
			log.Println("Mail: Data:", err)
			continue
		}

		fmt.Fprintf(wc, "To: %s\n", message.To)
		fmt.Fprintf(wc, "From: %s\n", message.From)
		fmt.Fprintf(wc, "Subject: %s\n", message.Subject)
		fmt.Fprintf(wc, "\n%s\n", message.Body)

		err = wc.Close()
		if err != nil {
			log.Println("Mail: Close:", err)
			continue
		}

		// Send the QUIT command and close the connection.
		err = client.Quit()
		if err != nil {
			log.Println("Mail: Quit:", err)
			continue
		}
	}
}
