// Talent Hound is a local-first AI desktop app built with Wails v3 (Go backend)
// and SolidJS (TypeScript frontend).
package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"camstuart/talent-hound/internal/db"
	"camstuart/talent-hound/internal/platform"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

// configDir is where the pointer to the data folder lives — outside the data
// folder itself. TALENT_HOUND_CONFIG_DIR overrides it, so an E2E run never
// reads or writes the recruiter's own setup.
func configDir() string {
	if dir := os.Getenv("TALENT_HOUND_CONFIG_DIR"); dir != "" {
		return dir
	}
	conf, err := os.UserConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(conf, "talent-hound")
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and runs the application, logging any error that might occur.
func main() {

	dbPath, err := db.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	gdb, err := db.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	jobs := NewJobService(gdb)
	// Extraction stages temporary plaintext beside the database, because that
	// is the folder the PRD says is encrypted.
	extraction := NewExtractService(gdb, jobs, filepath.Dir(dbPath))
	// One Ollama client for the registry and for embedding: they talk to the
	// same local endpoint, and the registry is what says which model answers.
	ollama := platform.NewOllama()
	registry := NewModelService(gdb, jobs, ollama)
	records := NewRecordService(gdb)
	classify := NewClassifyService(gdb, registry, ollama)
	profiles := NewCandidateProfileService(gdb, classify, records)
	artifacts := NewArtifactService(gdb)
	criteria := NewCriteriaService(gdb, registry, ollama, profiles)
	// The Exa key lives in the Windows credential store and is read at call
	// time; an empty key means searches refuse rather than being unavailable.
	exa := platform.NewExa("")
	credentials := NewCredentialService()
	search := NewSearchService(gdb)
	embed := NewEmbedService(gdb, jobs, registry, ollama)
	roleProfiles := NewRoleProfileService(gdb, classify)
	shortlist := NewShortlistService(gdb, search, embed, criteria, profiles, roleProfiles)
	// The data folder is the folder holding the database: the one folder the
	// recruiter copies for recovery.
	dataDir := filepath.Dir(dbPath)
	setupSv, err := NewSetupService(gdb, registry, configDir(), dataDir)
	if err != nil {
		log.Fatal(err)
	}
	// Personal-data entry is refused at the write, not in the interface.
	records.Guard = setupSv
	artifacts.Guard = setupSv

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "talent-hound",
		Description: "A local-first AI desktop app",
		Services: []application.Service{
			application.NewService(&GreetService{}),
			application.NewService(NewInitiativeService(gdb)),
			application.NewService(records),
			application.NewService(artifacts),
			application.NewService(jobs),
			application.NewService(extraction),
			application.NewService(NewChunkService(gdb, jobs)),
			application.NewService(search),
			application.NewService(registry),
			application.NewService(embed),
			application.NewService(classify),
			application.NewService(profiles),
			application.NewService(roleProfiles),
			application.NewService(criteria),
			application.NewService(NewDiscoveryService(gdb, exa, profiles, criteria, records, artifacts)),
			application.NewService(shortlist),
			application.NewService(NewAssessService(gdb, jobs, registry, ollama, embed, criteria, profiles, roleProfiles, shortlist)),
			application.NewService(NewQAService(gdb, registry, ollama, search, embed, profiles)),
			application.NewService(NewDraftService(gdb, registry, ollama, profiles, roleProfiles)),
			application.NewService(credentials),
			application.NewService(NewCloudService(gdb, ollama, records, profiles, credentials)),
			application.NewService(NewDeletionService(gdb)),
			application.NewService(setupSv),
			application.NewService(NewDiagnosticsService(gdb, setupSv, dataDir)),
			// Help is registered with the model it may use and works without
			// it: it is read when the rest of this list is the problem.
			application.NewService(NewHelpService(registry, ollama)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Talent Hound",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Run the application. This blocks until the application has been exited.
	err = app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
