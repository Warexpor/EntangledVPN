package vpncore

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var LogFile *os.File
var Logger *log.Logger
var logPathMu sync.Mutex
var currentLogPath string

func init() {
	Logger = log.New(io.Discard, "", log.LstdFlags)
}

func LogDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.TempDir()
	}
	return filepath.Join(appData, "EntangledVPN")
}

func CurrentLogPath() string {
	logPathMu.Lock()
	defer logPathMu.Unlock()
	return currentLogPath
}

func InitLogger() error {
	logDir := LogDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("entangled-%s.log", time.Now().Format("2006-01-02")))
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	LogFile = f
	logPathMu.Lock()
	currentLogPath = logPath
	logPathMu.Unlock()
	Logger = log.New(f, "", log.LstdFlags)

	Logger.Printf("=== Entangled VPN Client Started === log=%s", logPath)
	return nil
}

func CloseLogger() {
	if LogFile != nil {
		if Logger != nil {
			Logger.Println("=== Entangled VPN Client Stopped ===")
		}
		LogFile.Close()
		LogFile = nil
	}
}
