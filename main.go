// Talent Hound is a local-first AI desktop app built with Wails v3 (Go backend)
// and SolidJS (TypeScript frontend).
package main

import (
	"camstuart/talent-hound/internal/setup"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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

// dataPath is where this launch opens the database.
//
// The recruiter chooses a data folder in the first-run wizard, and that choice
// was recorded and never read: the database opened where it always had, so the
// folder they picked held nothing and the folder holding everything was one
// they were never shown. Recovery copies "the one folder", diagnostics report
// it, and the encryption gate decides whether real data may be kept in it — all
// of a folder that was not the one in use.
//
// The choice takes effect at the next launch rather than immediately: moving an
// open database out from under a running application is a different feature,
// and a worse one to get wrong.
func dataPath() (string, error) {
	settings, err := setup.Load(configDir())
	if err != nil || strings.TrimSpace(settings.DataFolder) == "" {
		// No preference, or none readable: where it has always been. A settings
		// file that cannot be read is not a reason to refuse to start.
		return db.DefaultPath()
	}
	// The environment still wins, because that is what the tests and the
	// server build point at their own folder with.
	if p := os.Getenv("TALENT_HOUND_DB_PATH"); p != "" {
		return db.DefaultPath()
	}
	dir := settings.DataFolder
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating the chosen data folder: %w", err)
	}
	return filepath.Join(dir, "talent-hound.db"), nil
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and runs the application, logging any error that might occur.
func main() {

	dbPath, err := dataPath()
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
			application.NewService(NewDiscoveryService(gdb, nil, profiles, criteria, records, artifacts, credentials)),
			application.NewService(shortlist),
			application.NewService(NewAssessService(gdb, jobs, registry, ollama, embed, criteria, profiles, roleProfiles, shortlist)),
			application.NewService(NewQAService(gdb, registry, ollama, search, embed, profiles)),
			application.NewService(NewDraftService(gdb, registry, ollama, profiles, roleProfiles)),
			application.NewService(credentials),
			application.NewService(NewCloudService(gdb, records, profiles, credentials)),
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
			Backdrop: application.MacBackdropTranslucent,
			// Hidden-inset keeps the traffic lights on top of the webview; the
			// title bar the frontend draws leaves room for them and marks itself
			// draggable. No InvisibleTitleBarHeight: that swallows every click in
			// the top 50px, which is exactly where the menu buttons now live.
			TitleBar: application.MacTitleBarHiddenInset,
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
