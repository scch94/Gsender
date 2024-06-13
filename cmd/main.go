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
		time.Sleep(durationUntilNextExecution)

		smtpHost := config.Config.SmtpConfig.SmtpHost
		smtpPort := config.Config.SmtpConfig.SmtpPort
		senderEmail := config.Config.MailInfo.MailSender
		subject := config.Config.MailInfo.Subject

		// Obtener todos los correos electrónicos de los destinatarios
		var recipients []string
		for _, receiver := range config.Config.MailInfo.MailReceivers {
			recipients = append(recipients, receiver.Email)
		}

		recipientEmails := strings.Join(recipients, ", ")
		// Adjuntar el archivo PDF

		// Obtener la fecha de ayer
		yesterday := time.Now().AddDate(0, 0, -1)

		// Formatear la fecha como "YYYYMMDD"
		yesterdayFormatted := yesterday.Format("20060102")

		// Concatenar el nombre del archivo
		pdfFileName := fmt.Sprintf("report_Dominica_%s%s.pdf", yesterdayFormatted, config.Config.PdgGenerationTime)

		ins_log.Tracef(ctx, "this is the name of the pdf file %s", pdfFileName)

		pdfFile, err := os.ReadFile(config.Config.UbicationFiles + pdfFileName)
		if err != nil {
			ins_log.Errorf(ctx, "Error al leer el archivo PDF: %s ", err)
			return
		}

		encodedPDF := base64.StdEncoding.EncodeToString(pdfFile)

		ins_log.Infof(ctx, "pdf_filed updated")

		// Establecer conexión con el servidor SMTP
		client, err := smtp.Dial(smtpHost + ":" + smtpPort)
		if err != nil {
			ins_log.Errorf(ctx, "Error al conectar con el servidor SMTP: %s ", err)
			return
		}
		defer client.Close()

		ins_log.Infof(ctx, "conectted to de smtp server: ")

		// Ejecutar el saludo (HELO)
		if err := client.Hello("digicelgroup.com"); err != nil {
			ins_log.Errorf(ctx, "Error al saludar al servidor SMTP: %s ", err)
			return
		}

		ins_log.Infof(ctx, "connected to the group digicelgroup.com")

		// Ejecutar el envío del remitente (MAIL FROM)
		if err := client.Mail(senderEmail); err != nil {
			ins_log.Errorf(ctx, "Error al enviar el remitente: %s ", err)
			return
		}
		ins_log.Infof(ctx, "sender updated successfully")

		// Ejecutar la entrega a todos los destinatarios (RCPT TO)
		for _, recipient := range recipients {
			if err := client.Rcpt(recipient); err != nil {
				ins_log.Errorf(ctx, "Error al entregar al destinatario: %s ", err)
				return
			}
		}
		ins_log.Infof(ctx, "recipients updated successfully")

		//leemos el html
		htmlBodyPath := config.Config.MailInfo.UbicationMessage
		htmlBody, err := os.ReadFile(htmlBodyPath)
		if err != nil {
			ins_log.Errorf(ctx, "error reading html body")
			return
		}

		// Valores para reemplazar en el HTML
		companyName := config.Config.CompanyName
		dateYesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

		body := strings.ReplaceAll(string(htmlBody), "[COMPANY_NAME]", companyName)
		body = strings.ReplaceAll(body, "[DATE_YESTERDAY]", dateYesterday)

		ins_log.Info(ctx, "HTML FILE IS CREATED")

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

		// Cerrar la sección multipart
		message = append(message, "\r\n--boundary--\r\n"...)
		message = append(message, "\r\n."...)
		// Iniciar la escritura del mensaje (DATA)
		w, err := client.Data()
		if err != nil {
			ins_log.Errorf(ctx, "Error al iniciar la escritura del mensaje: %s ", err)
			return
		}
		defer w.Close()
		ins_log.Infof(ctx, "WRITE MESSAGE UPDATED SUCCESFULY")
		// Escribir el cuerpo del mensaje
		if _, err := w.Write(message); err != nil {
			ins_log.Errorf(ctx, "Error al escribir el cuerpo del mensaje: %s ", err)
			return
		}

		ins_log.Infof(ctx, "WRITE MESSAGE body UPDATED SUCCESFULY")

		// Finalizar la escritura del mensaje
		if err := w.Close(); err != nil {
			ins_log.Errorf(ctx, "Error al finalizar la escritura del mensaje: %s ", err)
			return
		}
		ins_log.Infof(ctx, "finish escritura del mensaje")
		// QUIT para terminar la sesión SMTP
		if err := client.Quit(); err != nil {
			ins_log.Errorf(ctx, "Error al finalizar la sesión SMTP: %s ", err)
			return
		}
		ins_log.Infof(ctx, "smtp session ended")
		ins_log.Infof(ctx, "Correo electrónico enviado correctamente!")
	}
}
