package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"vectile/backend/appdata"
	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
	"vectile/backend/mcp"
	"vectile/backend/parser"
	"vectile/backend/services"
	"vectile/backend/startup"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version is stamped at release time via `-ldflags "-X main.Version=vX.Y.Z"`.
// Keep it a var (not a const) so the linker can override it.
var Version = "v0.4.1"

func main() {
	// Expose the build-time version to the frontend (AppService.GetVersion).
	services.Version = Version

	// Resolve the app-data directory first so config, db/, and models/ have a
	// home. The embedding model is expected to already be in models/.
	if _, err := appdata.Init(); err != nil {
		log.Fatalf("appdata init: %v", err)
	}

	cfg, err := config.Load(appdata.ConfigPath())
	if err != nil {
		log.Fatalf("config load: %v", err)
	}

	modelPath := cfg.ActiveModel
	if modelPath == "" {
		modelPath = appdata.ModelPath()
	}

	core := &services.Core{
		Cfg:      cfg,
		CfgPath:  appdata.ConfigPath(),
		Embedder: embeddings.NewEmbedder(modelPath, 0, 0),
	}

	// The in-app MCP server: exposed to the frontend and auto-started below
	// when the user has enabled it in Settings. Binds to 127.0.0.1 only.
	mcpSvc := mcp.NewMCPService(core)

	// Native desktop notifications (model download + indexing finished).
	notifier := notifications.New()
	core.Notifications = notifier

	app := application.New(application.Options{
		Name:        "vectile",
		Description: "A local, private search across everything you've written, read, and kept.",
		Services: []application.Service{
			application.NewService(services.NewAppService(core)),
			application.NewService(services.NewSearchService(core)),
			application.NewService(services.NewIndexService(core)),
			application.NewService(services.NewModelService(core)),
			application.NewService(mcpSvc),
			application.NewService(notifier),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// The app lives in the system tray; closing the window hides it
			// (see the WindowClosing hook below) instead of terminating.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})
	core.App = app

	// Ask for notification permission once (no-op on Windows/Linux; on macOS
	// this prompts the user). Best-effort: a rejection never blocks launch.
	_, _ = notifier.RequestNotificationAuthorization()

	if cfg.GUI.StartOnLogin {
		if enabled, _ := startup.IsEnabled(); !enabled {
			_ = startup.Enable()
		}
	}

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "vectile",
		Width:     1180,
		Height:    760,
		MinWidth:  680,
		MinHeight: 600,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(246, 244, 239),
		URL:              "/",
	})

	// Closing the window hides it instead of quitting — the app keeps running
	// in the system tray and is re-opened from the tray menu or tray click.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		win.Hide()
	})

	// Open the database and apply the schema once the app is wired up.
	if err := db.Open(appdata.DBPath()); err != nil {
		log.Fatalf("db open: %v", err)
	}

	// Reconcile the models folder and load the active model (or the default)
	// with its per-model settings, rebuilding the vector tables if the
	// active model's embedding dimension changed.
	if err := services.NewModelService(core).ApplyActiveModel(); err != nil {
		log.Fatalf("apply active model: %v", err)
	}

	// Auto-start the MCP server when the user enabled it in Settings. A busy
	// port logs a warning but never blocks launch.
	if cfg.MCP.Enabled {
		if _, err := mcpSvc.StartServer(cfg.MCP.Port); err != nil {
			log.Printf("mcp auto-start: %v", err)
		}
	}

	// Auto-reindex: poll the config each minute; fire an index-all when the
	// interval has elapsed. Shares the same mutex as manual index runs. The
	// loop reads core.Cfg live so settings changes apply without a restart.
	go autoReindexLoop(core)

	// System tray: status line, Index submenu, cancel, and Quit. Must be
	// created before app.Run() — Wails defers the actual tray init until the
	// event loop starts, but New() has to happen first.
	tray := newTray(app, core, win)
	tray.start()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}

	tray.stop()

	parser.ClosePDFPool()
	core.Embedder.Close()
	_ = db.Close()
}

// autoReindexLoop reads core.Cfg live so settings changes (auto-reindex toggle
// and interval) take effect on the next tick without restarting the loop.
func autoReindexLoop(core *services.Core) {
	svc := services.NewIndexService(core)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	var last time.Time
	for range ticker.C {
		c := core.Cfg
		if !c.GUI.AutoReindex {
			continue
		}
		interval := time.Duration(c.GUI.AutoReindexIntervalMinutes) * time.Minute
		if interval < time.Minute {
			interval = time.Minute
		}
		if !last.IsZero() && time.Since(last) < interval {
			continue
		}
		last = time.Now()
		svc.IndexAll(false)
	}
}
