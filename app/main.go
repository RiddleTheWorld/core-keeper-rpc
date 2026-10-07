package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()
	startHidden := len(os.Args) > 1 && os.Args[1] == "--hidden"

	err := wails.Run(&options.App{
		Title:             "Core Keeper Presence",
		Width:             1120,
		Height:            720,
		MinWidth:          560,
		MinHeight:         560,
		StartHidden:       startHidden,
		WindowStartState:  options.Normal,
		HideWindowOnClose: true, // keeps updating Discord in the background
		BackgroundColour:  &options.RGBA{R: 21, G: 17, B: 13, A: 1},
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup:         app.startup,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ckpresence-7d1f2e",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if app.ctx != nil {
					runtime.WindowShow(app.ctx)
					runtime.WindowUnminimise(app.ctx)
				}
			},
		},
		Bind:    []interface{}{app},
		Windows: &windows.Options{Theme: windows.Dark},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
