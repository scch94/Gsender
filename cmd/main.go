package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scch94/Gsender/config"
	"github.com/scch94/ins_log"
)

func main() {

	ctx := context.Background()

	// Load configuration
	if err := config.Upconfig(ctx); err != nil {
		ins_log.Errorf(ctx, "error loading configuration: %v %s ", err)
		return
	}

	//iniciamos el logger
	go initializeAndWatchLogger(ctx)

	ins_log.SetService("EpinMailSender")
	ins_log.SetLevel(config.Config.LogLevel)
	ctx = ins_log.SetPackageNameInContext(ctx, "main")
	defer end(ctx)

	ins_log.Infof(ctx, "starting email sender gateway version: %+v", getVersion())
	// Ejecutamos la función para enviar correos electrónicos
	sendEmails(ctx)

	// Esperamos para que el programa no termine inmediatamente
	select {}

}

// funcion que ira cambiando de log cada hora
func initializeAndWatchLogger(ctx context.Context) {
	var file *os.File
	var logFileName string
	var err error
	for {
		select {
		case <-ctx.Done():
			return
		default:
			logDir := "../log"

			// Create the log directory if it doesn't exist
			if err = os.MkdirAll(logDir, 0755); err != nil {
				ins_log.Errorf(ctx, "error creating log directory: %v", err)
				return
			}

			// Define the log file name
			today := time.Now().Format("2006-01-02 15")
			replacer := strings.NewReplacer(" ", "_")
			today = replacer.Replace(today)
			logFileName = filepath.Join(logDir, config.Config.LogName+today+".log")

			// Open the log file
			file, err = os.OpenFile(logFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
			if err != nil {
				ins_log.Errorf(ctx, "error opening log file: %v", err)
				return
			}

			// Create a writer that writes to both file and console
			multiWriter := io.MultiWriter(os.Stdout, file)
			ins_log.StartLoggerWithWriter(multiWriter)

			// Esperar hasta el inicio de la próxima hora
			nextHour := time.Now().Truncate(time.Hour).Add(time.Hour)
			time.Sleep(time.Until(nextHour))

			// Close the previous log file
			file.Close()
		}
	}
}
func getVersion() string {
	return "1.0.0"
}

// LoginAuth implements the smtp.Auth interface for LOGIN authentication.
type LoginAuth struct {
	username, password string
}

// Start implements the Start method of smtp.Auth interface.
func (a *LoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", []byte(a.username), nil
}

// Next implements the Next method of smtp.Auth interface.
func (a *LoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		switch string(fromServer) {
		case "Username:":
			return []byte(a.username), nil
		case "Password:":
			return []byte(a.password), nil
		default:
			return nil, fmt.Errorf("unexpected server challenge: %s", fromServer)
		}
	}
	return nil, nil
}
func sendEmails(ctx context.Context) {
	for {
		utfi := ins_log.GenerateUTFI()
		ctx = ins_log.SetUTFIInContext(ctx, utfi)
		// Calcular la hora específica del día para repetir las tareas
		dailyTime := config.Config.ExecutionTime

		// Obtener el momento actual
		now := time.Now()

		// Calcular la duración hasta la próxima ejecución
		nextExecution := time.Date(now.Year(), now.Month(), now.Day(), dailyTime.Hour(), dailyTime.Minute(), dailyTime.Second(), 0, now.Location())
		if nextExecution.Before(now) {
			// Si ya ha pasado la hora para la ejecución de hoy, pasar a mañana
			nextExecution = nextExecution.AddDate(0, 0, 1)
		}
		durationUntilNextExecution := nextExecution.Sub(now)

		// Esperar hasta la próxima ejecución
		ins_log.Tracef(ctx, "durayion Until Next Execution: %s", durationUntilNextExecution)
		time.Sleep(durationUntilNextExecution)

		smtpHost := config.Config.SmtpConfig.SmtpHost
		smtpPort := config.Config.SmtpConfig.SmtpPort
		senderEmail := config.Config.MailInfo.MailSender
		subject := config.Config.MailInfo.Subject

		ins_log.Infof(ctx, "these are the emails to send")
		// Obtener todos los correos electrónicos de los destinatarios
		var recipients []string
		for i, receiver := range config.Config.MailInfo.MailReceivers {
			ins_log.Infof(ctx, "%d: %s", i+1, receiver.Email)
			recipients = append(recipients, receiver.Email)
		}

		recipientEmails := strings.Join(recipients, ", ")
		// Adjuntar el archivo PDF

		// Obtener la fecha de ayer
		today := time.Now()
		todayFormatted := today.Format("20060102")

		// Concatenar el nombre del archivo
		pdfFileName := fmt.Sprintf("report_Dominica_%s%s.pdf", todayFormatted, config.Config.PdfGenerationTime)

		ins_log.Tracef(ctx, "this is the name of the pdf file %s", pdfFileName)

		pdfFile, err := os.ReadFile(config.Config.UbicationFiles + pdfFileName)
		if err != nil {
			ins_log.Errorf(ctx, "Error when try to get the pdffile: %s ", err)
			return
		}

		encodedPDF := base64.StdEncoding.EncodeToString(pdfFile)

		ins_log.Tracef(ctx, "pdf_filed updated ")

		// Establecer conexión con el servidor SMTP
		client, err := smtp.Dial(smtpHost + ":" + smtpPort)
		if err != nil {
			ins_log.Errorf(ctx, "Error when we try to connect with the SMTP server: %s ", err)
			return
		}
		defer client.Close()

		ins_log.Debug(ctx, "conectted to de smtp server")

		// Ejecutar el saludo (HELO)
		if err := client.Hello(config.Config.SmtpConfig.HostName); err != nil {
			ins_log.Errorf(ctx, "Error when we try to specify the host name: %s ", err)
			return
		}

		ins_log.Debugf(ctx, "hostname %s", config.Config.SmtpConfig.HostName)

		// Ejecutar el envío del remitente (MAIL FROM)
		if err := client.Mail(senderEmail); err != nil {
			ins_log.Errorf(ctx, "Failed to send MAIL FROM command: %s", err)
			return
		}
		ins_log.Debugf(ctx, "sender updated successfully")

		// Execute the RCPT TO command for all recipients
		for _, recipient := range recipients {
			if err := client.Rcpt(recipient); err != nil {
				ins_log.Errorf(ctx, "Failed to deliver to recipient %s: %s", recipient, err)
				return
			}
		}
		ins_log.Debugf(ctx, "recipients updated successfully")

		// leer el html
		htmlBodyPath := config.Config.MailInfo.UbicationMessage
		htmlBody, err := os.ReadFile(htmlBodyPath)
		if err != nil {
			ins_log.Errorf(ctx, "Error reading HTML body from %s: %s", htmlBodyPath, err)
			return
		}

		// remplazamos los valores del html
		companyName := config.Config.CompanyName
		dateYesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

		body := strings.ReplaceAll(string(htmlBody), "[COMPANY_NAME]", companyName)
		body = strings.ReplaceAll(body, "[DATE_YESTERDAY]", dateYesterday)

		ins_log.Tracef(ctx, "HTML body has been created and placeholders have been replaced successfully")

		message := []byte(
			"To: " + recipientEmails + "\r\n" +
				"Subject: " + subject + "\r\n" +
				"MIME-Version: 1.0\r\n" +
				"Content-Type: multipart/mixed; boundary=boundary\r\n" +
				"\r\n" +
				"--boundary\r\n" +
				"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
				"Content-Transfer-Encoding: 7bit\r\n" +
				"\r\n" +
				body + "\r\n" +
				"\r\n" +
				"--boundary\r\n" +
				"Content-Type: application/pdf; name=\"" + pdfFileName + "\"\r\n" +
				"Content-Transfer-Encoding: base64\r\n" +
				"Content-Disposition: attachment; filename=\"" + pdfFileName + "\"\r\n" +
				"\r\n" +
				encodedPDF + "\r\n" +
				"--boundary--\r\n",
		)

		// Close the multipart section
		message = append(message, "\r\n--boundary--\r\n"...)
		message = append(message, "\r\n."...)

		// Start writing the message (DATA)
		w, err := client.Data()
		if err != nil {
			ins_log.Errorf(ctx, "Error starting the message write: %s", err)
			return
		}
		defer w.Close()
		ins_log.Tracef(ctx, "Message write started successfully")

		// Write the message body
		if _, err := w.Write(message); err != nil {
			ins_log.Errorf(ctx, "Error writing the message body: %s", err)
			return
		}

		ins_log.Tracef(ctx, "Message body written successfully")

		// Finish writing the message
		if err := w.Close(); err != nil {
			ins_log.Errorf(ctx, "Error finishing the message write: %s", err)
			return
		}
		ins_log.Infof(ctx, "Message write finished successfully")

		// QUIT to end the SMTP session
		if err := client.Quit(); err != nil {
			ins_log.Errorf(ctx, "Error ending the SMTP session: %s", err)
			return
		}
		ins_log.Tracef(ctx, "SMTP session ended successfully")
		ins_log.Infof(ctx, "Email sent successfully!")
	}
}
func end(ctx context.Context) {
	ins_log.Infof(ctx, "clossing...")
}
