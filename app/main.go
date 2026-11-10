// Command app is the Prayer Deck client: the web frontend wrapped by Wails so
// the same interface ships to browsers (server build) and, later, iOS.
package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

// defaultAPIURL is the API the client talks to when PRAYER_DECK_API_URL is
// unset. Builds without a process environment (iOS) override it with
// -ldflags "-X main.defaultAPIURL=...".
var defaultAPIURL = "http://localhost:8080"

func main() {
	apiURL := os.Getenv("PRAYER_DECK_API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	app := application.New(application.Options{
		Name:        "Prayer Deck",
		Description: "Draw cards, remember people, and pray for them.",
		Services: []application.Service{
			application.NewService(&ConfigService{config: ClientConfig{APIURL: apiURL}}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Prayer Deck",
		Width:            1100,
		Height:           800,
		BackgroundColour: application.NewRGB(244, 237, 225),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
