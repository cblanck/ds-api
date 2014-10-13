package main

import (
	"fmt"
	"log"
	"net/smtp"
)

type EmailManager struct {
	mail_queue    chan *EmailMessage
	server_config *Config
}

type EmailMessage struct {
	From string
	To   string
	Body string
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
func (t *EmailManager) QueueEmail(to, from, body string) {
	message := new(EmailMessage)
	message.To = to
	message.From = from
	message.Body = body
	t.mail_queue <- message
}

// Listen on the channel of email messages, and send them using the server
// set in the application settings.
func (t *EmailManager) process_mail_queue() {
	for {
		// Block until we get an email to send
		message := <-t.mail_queue

		// Connect to the remote SMTP server.
		conn, err := smtp.Dial(fmt.Sprintf("%s:%d", t.server_config.Mail.Host, t.server_config.Mail.Port))
		if err != nil {
			log.Println(err)
			continue
		}

		// Set the sender and recipient first
		if err := conn.Mail(message.From); err != nil {
			log.Println(err)
			continue
		}
		if err := conn.Rcpt(message.To); err != nil {
			log.Println(err)
			continue
		}

		// Send the email body.
		wc, err := conn.Data()
		if err != nil {
			log.Println(err)
			continue
		}
		_, err = fmt.Fprintf(wc, message.Body)
		if err != nil {
			log.Println(err)
			continue
		}

		err = wc.Close()
		if err != nil {
			log.Println(err)
			continue
		}

		// Send the QUIT command and close the connection.
		err = conn.Quit()
		if err != nil {
			log.Println(err)
			continue
		}
	}
}
