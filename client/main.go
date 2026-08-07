package main

import (
	"embed"
	"log"

	"entangled-client/vpncore"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := vpncore.InitLogger(); err != nil {
		log.Fatal(err)
	}
	defer vpncore.CloseLogger()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "Entangled VPN",
		Width:            1100,
		Height:           720,
		MinWidth:         720,
		MinHeight:        480,
		DisableResize:    false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 8, G: 8, B: 8, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		vpncore.Logger.Printf("App error: %v", err)
		log.Fatal(err)
	}
}
