package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/scch94/Gconfiguration"
	"github.com/scch94/ins_log"
)

var Config SenderConfig

func Upconfig(ctx context.Context) error {

	//traemos el contexto y le setiamos el contexto actual
	// Agregamos el valor "packageName" al contexto
	ctx = ins_log.SetPackageNameInContext(ctx, "config")

	ins_log.Info(ctx, "starting to get the config struct ")
	err := Gconfiguration.GetConfig(&Config, "../config", "senderConfig.json")

	if err != nil {
		ins_log.Fatalf(ctx, "error in Gconfiguration.GetConfig() ", err)
		return err
	}
	return nil
}
func (s SenderConfig) ConfigurationString() string {
	configJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf("Error al convertir la configuración a JSON: %v", err)
	}
	return string(configJSON)
}

type SenderConfig struct {
	CompanyName       string     `json:"company_name"`
	LogLevel          string     `json:"log_Level"`
	LogName           string     `json:"log_Name"`
	SmtpConfig        SmtpConfig `json:"smtp_Config"`
	MailInfo          MailInfo   `json:"mail_Info"`
	UbicationFiles    string     `json:"ubication_files"`
	ExecutionTime     time.Time  `json:"execution_time"`
	PdgGenerationTime string     `json:"pdg_generation_time"`
}

type SmtpConfig struct {
	SmtpHost string `json:"smtp_Host"`
	SmtpPort string `json:"smtp_Port"`
}

type MailInfo struct {
	MailSender       string         `json:"mail_Sender"`
	MailReceivers    []MailReceiver `json:"mail_Receibers"`
	UbicationMessage string         `json:"ubication_message"`
	Subject          string         `json:"subject"`
}

type MailReceiver struct {
	Email string `json:"email"`
}
