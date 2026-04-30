package main

import (
	"context"
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
		OnStartup:  app.OnStartup,
		OnShutdown: app.OnShutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			return app.OnBeforeClose(ctx)
		},
		Bind:             []interface{}{app},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
	})
	if err != nil {
		log.Fatal(err)
	}
}
