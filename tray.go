package main

// System tray integration. Follows the same pattern local-rag uses for its
// tray: a status line, an "Index" submenu (all collections + each configured
// collection), a cancel action while a run is in flight, and Quit. The menu
// is rebuilt from live app state through a single coalescing goroutine so
// SetMenu is never called concurrently (native systrays don't like that).

import (
	"fmt"
	"sort"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"vectile/backend/db"
	"vectile/backend/embeddings"
	"vectile/backend/services"
)

// tray holds the system-tray references and the rebuild plumbing.
type tray struct {
	app    *application.App
	core   *services.Core
	window *application.WebviewWindow

	systray *application.SystemTray

	// rebuildCh coalesces rebuild requests from any goroutine (Wails event
	// listeners fire on background goroutines) into a single rebuild goroutine.
	rebuildCh chan struct{}
	done      chan struct{}

	// last* caches what the menu last looked like so rebuilds that change
	// nothing (e.g. a progress tick that doesn't alter the status text) are
	// cheap no-ops instead of swapping the native menu.
	lastStatus     string
	lastIndexing   bool
	lastCollection string
}

func newTray(app *application.App, core *services.Core, window *application.WebviewWindow) *tray {
	return &tray{
		app:       app,
		core:      core,
		window:    window,
		rebuildCh: make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
}

// start creates the systray (must happen before app.Run), wires the rebuild
// loop and event listeners, and builds the initial menu.
func (t *tray) start() {
	st := t.app.SystemTray.New()
	st.SetIcon(trayIconBytes())
	st.SetTooltip("vectile — local, private search")
	st.OnClick(t.showWindow)
	t.systray = st

	// Coalescing rebuild loop (local-rag pattern): all rebuild requests land
	// on one goroutine so SetMenu is never called concurrently.
	go t.rebuildLoop()

	// Rebuild on the app events that change what the menu shows. The guard in
	// rebuildMenu makes per-file progress events no-ops while a collection is
	// already being shown.
	for _, name := range []string{
		"indexing:file",
		"indexing:complete",
		"indexing:all-done",
		"indexing:cancelled",
		"model:changed",
	} {
		t.app.Event.On(name, func(*application.CustomEvent) { t.requestRebuild() })
	}

	// Periodic refresh so the status line stays honest even without events
	// (e.g. the model lazily loading on first embed).
	go t.statusTimer()

	// macOS: clicking the dock icon when the window is hidden reopens it.
	t.app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) {
		t.showWindow()
	})

	t.rebuildMenu()
}

// stop destroys the tray and stops the rebuild/status goroutines.
func (t *tray) stop() {
	close(t.done)
	if t.systray != nil {
		t.systray.Destroy()
	}
}

// requestRebuild asks the rebuild goroutine to refresh the tray menu.
// Multiple rapid calls are coalesced into one rebuild.
func (t *tray) requestRebuild() {
	select {
	case t.rebuildCh <- struct{}{}:
	default:
		// A rebuild is already pending — skip.
	}
}

// rebuildLoop drains rebuildCh and calls rebuildMenu serially, with a short
// debounce so bursts of requests collapse into one.
func (t *tray) rebuildLoop() {
	for {
		select {
		case <-t.rebuildCh:
			// Debounce: wait briefly for more requests to coalesce.
			time.Sleep(150 * time.Millisecond)
			for {
				select {
				case <-t.rebuildCh:
				default:
					goto drained
				}
			}
		drained:
			t.rebuildMenu()

		case <-t.done:
			return
		}
	}
}

// statusTimer periodically refreshes the menu (the guard makes no-op ticks
// free).
func (t *tray) statusTimer() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			t.requestRebuild()
		case <-t.done:
			return
		}
	}
}

// rebuildMenu builds the tray menu from the current app state and applies it.
// It must only run on the rebuild goroutine (or before concurrency starts).
func (t *tray) rebuildMenu() {
	if t.systray == nil {
		return
	}

	idx := services.NewIndexService(t.core)
	st := idx.GetIndexingState()
	status := t.trayStatus(st)
	collection := currentCollection(st.Collections)

	if status == t.lastStatus && st.Active == t.lastIndexing &&
		collection == t.lastCollection {
		return
	}
	t.lastStatus, t.lastIndexing, t.lastCollection = status, st.Active, collection

	menu := t.app.NewMenu()

	menu.Add("Show Vectile").OnClick(func(*application.Context) { t.showWindow() })
	menu.AddSeparator()

	// Status line (disabled) — mirror of local-rag's status item.
	menu.Add(status).SetEnabled(false)

	// Index submenu (local-rag style): All Collections + each configured one.
	indexSub := menu.AddSubmenu("Index")
	indexSub.Add("All Collections").OnClick(func(*application.Context) {
		t.triggerIndex("")
	})
	indexSub.AddSeparator()
	for _, name := range t.collections() {
		n := name
		indexSub.Add(n).OnClick(func(*application.Context) {
			t.triggerIndex(n)
		})
	}
	// Disable the whole submenu while a run is in flight.
	if st.Active {
		if item := menu.FindByLabel("Index"); item != nil {
			item.SetEnabled(false)
		}
	}

	if st.Active {
		menu.Add("Cancel Indexing").OnClick(func(*application.Context) {
			idx.CancelIndexing()
		})
	}

	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { t.app.Quit() })

	t.systray.SetMenu(menu)
}

// triggerIndex starts an index run from the tray and refreshes the menu so it
// flips to "Indexing…" immediately, before the first indexing:file event.
func (t *tray) triggerIndex(name string) {
	idx := services.NewIndexService(t.core)
	var ok bool
	var err error
	if name == "" {
		ok, err = idx.IndexAll(false)
	} else {
		ok, err = idx.IndexCollection(name, false)
	}
	if err != nil || !ok {
		return
	}
	t.requestRebuild()
}

// showWindow brings the main window forward from the tray.
func (t *tray) showWindow() {
	showMainWindow(t.window)
}

// showMainWindow brings the main window forward. Focus also restores the window
// if it is minimised; the brief always-on-top toggle helps on window managers
// that won't raise a hidden window otherwise, without leaving the window
// permanently on top. Called from the tray and from the single-instance
// callback in main.go, both of which run on background goroutines.
func showMainWindow(win *application.WebviewWindow) {
	win.Show()
	win.Focus()
	win.SetAlwaysOnTop(true)
	win.SetAlwaysOnTop(false)
}

// trayStatus returns the status line shown in the menu.
func (t *tray) trayStatus(st services.IndexState) string {
	if st.Active {
		if name := currentCollection(st.Collections); name != "" {
			return "Indexing: " + name + "…"
		}
		return "Indexing…"
	}
	return t.modelStatus()
}

// currentCollection returns the collection with the most advanced progress
// (the one currently being indexed), or "" when none.
func currentCollection(cols map[string]services.IndexFileProgress) string {
	var name string
	var best int
	for n, p := range cols {
		if p.Indexed >= best {
			best = p.Indexed
			name = n
		}
	}
	return name
}

// modelStatus describes the embedding model state.
func (t *tray) modelStatus() string {
	model := t.modelName()
	switch t.core.Embedder.State() {
	case embeddings.StateLoaded:
		return "Model: " + model + " · loaded"
	case embeddings.StateFailed:
		return "Model: failed"
	default:
		return "Model: " + model + " · idle"
	}
}

// modelName returns the active model's display name (falling back to the
// configured default).
func (t *tray) modelName() string {
	if active, ok, err := db.GetActiveModel(db.DB); err == nil && ok {
		if active.Dimensions > 0 {
			return fmt.Sprintf("%s · %dd", active.Name, active.Dimensions)
		}
		return active.Name
	}
	if t.core.Cfg != nil && t.core.Cfg.EmbeddingModel != "" {
		return t.core.Cfg.EmbeddingModel
	}
	return "bge-m3"
}

// collections returns the enabled, configured collections in a deterministic
// order: system (obsidian, calibre), then repos, then projects.
func (t *tray) collections() []string {
	if t.core.Cfg == nil {
		return nil
	}
	cfg := t.core.Cfg
	var names []string
	if cfg.IsCollectionEnabled("obsidian") && len(cfg.ObsidianVaults) > 0 {
		names = append(names, "obsidian")
	}
	if cfg.IsCollectionEnabled("calibre") && len(cfg.CalibreLibraries) > 0 {
		names = append(names, "calibre")
	}
	for _, n := range sortedKeys(cfg.Repositories) {
		if cfg.IsCollectionEnabled(n) {
			names = append(names, n)
		}
	}
	for _, n := range sortedKeys(cfg.Projects) {
		if cfg.IsCollectionEnabled(n) {
			names = append(names, n)
		}
	}
	return names
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
