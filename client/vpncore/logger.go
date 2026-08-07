package vpncore

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var LogFile *os.File
var Logger *log.Logger

func InitLogger() error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	logDir := filepath.Join(appData, "EntangledVPN")
	os.MkdirAll(logDir, 0755)

	logPath := filepath.Join(logDir, fmt.Sprintf("entangled-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	LogFile = f
	Logger = log.New(f, "", log.LstdFlags|log.Lshortfile)

	Logger.Println("=== Entangled VPN Client Started ===")
	return nil
}

func CloseLogger() {
	if LogFile != nil {
		Logger.Println("=== Entangled VPN Client Stopped ===")
		LogFile.Close()
	}
}
