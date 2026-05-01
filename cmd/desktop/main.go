package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:     "Meeting Notes",
		Width:     1280,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.OnStartup,
		OnShutdown:    app.OnShutdown,
		OnBeforeClose: app.OnBeforeClose,
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 26, G: 26, B: 26, A: 255},
	})
	if err != nil {
		log.Fatal(err)
	}
}
