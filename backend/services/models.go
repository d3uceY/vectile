// Package services exposes the vectile backend to the frontend as Wails v3
// services: status/library data, hybrid search, and config + indexing.
package services

import (
	"context"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"

	"vectile/backend/config"
	"vectile/backend/embeddings"
)

// Version is the app version shown in the UI; main() copies it from the
// build-time stamped main.Version (via -ldflags "-X main.Version=..."). Defaults to "dev".
var Version = "dev"

// Core holds the shared runtime state for all services.
type Core struct {
	Cfg           *config.Config
	CfgPath       string
	Embedder      *embeddings.Embedder
	App           *application.App
	Notifications *notifications.NotificationService

	indexMu  sync.Mutex // serializes index/prune runs
	indexing bool

	// progressMu guards the live snapshot of the active index run (latest
	// per-collection progress + whether it's an "Index all"), so a frontend
	// that reloads or reconnects mid-run can rebuild its indexing UI. Events
	// remain the live update channel; this only seeds the initial state.
	progressMu sync.Mutex
	progress   map[string]IndexFileProgress
	allRun     bool

	// cancelMu guards the active run's cancel func, so the user can abort an
	// index from the frontend between batches.
	cancelMu sync.Mutex
	cancel   context.CancelFunc
}

// newIndexContext creates a cancellable context for a new index run and
// registers its cancel func for CancelIndexing.
func (c *Core) newIndexContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelMu.Lock()
	c.cancel = cancel
	c.cancelMu.Unlock()
	return ctx
}

// clearIndexContext drops the active cancel func once a run finishes.
func (c *Core) clearIndexContext() {
	c.cancelMu.Lock()
	c.cancel = nil
	c.cancelMu.Unlock()
}

// cancelIndex aborts the active index run, if any, and reports whether one
// was running.
func (c *Core) cancelIndex() bool {
	c.cancelMu.Lock()
	defer c.cancelMu.Unlock()
	if c.cancel == nil {
		return false
	}
	c.cancel()
	return true
}

// sendNotification fires a native desktop notification through the Wails
// notifications service. No-op when the service isn't wired up (tests) or the
// platform rejects the call.
func (c *Core) sendNotification(id, title, body string) {
	if c.Notifications == nil {
		return
	}
	_ = c.Notifications.SendNotification(notifications.NotificationOptions{
		ID: id, Title: title, Body: body,
	})
}

// isAllRun reports whether the active index run is an "Index all" pass, so a
// single-collection run only notifies once.
func (c *Core) isAllRun() bool {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	return c.allRun
}

// Status is the app-wide status summary shown in the UI.
type Status struct {
	Collections int              `json:"collections"`
	Sources     int              `json:"sources"`
	Chunks      int              `json:"chunks"`
	DBSize      int64            `json:"dbSize"`
	ModelState  embeddings.State `json:"modelState"`
	ModelName   string           `json:"modelName"`
	ModelPath   string           `json:"modelPath"`
	ModelError  string           `json:"modelError"`
	LastIndexed string `json:"lastIndexed"`
}

// Collection is a library collection with counts and enabled state.
// NeedsReindex is true when the collection has indexed documents but no
// embeddings — typically right after switching to a model with a different
// embedding dimension, which empties the vector tables until re-indexing.
type Collection struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Sources      int    `json:"sources"`
	Chunks       int    `json:"chunks"`
	Created      string `json:"created"`
	Enabled      bool   `json:"enabled"`
	NeedsReindex bool   `json:"needsReindex"`
	LastIndexed string `json:"lastIndexed"`
}

// Source is one indexed source.
type Source struct {
	ID           int64  `json:"id"`
	CollectionID int64  `json:"collectionId"`
	SourceType   string `json:"sourceType"`
	Path         string `json:"path"`
	Chunks       int    `json:"chunks"`
	LastIndexed  string `json:"lastIndexed"`
}

// SourcePage is one keyset page of a collection's sources. Before and After are
// opaque cursors for the neighbouring pages; "" means that side ends there.
type SourcePage struct {
	Sources []Source `json:"sources"`
	Before  string   `json:"before"`
	After   string   `json:"after"`
}

// Document is one chunked document (a browse reading pane), with its full text.
type Document struct {
	ID           int64  `json:"id"`
	SourceID     int64  `json:"sourceId"`
	CollectionID int64  `json:"collectionId"`
	ChunkIndex   int    `json:"chunkIndex"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Metadata     any    `json:"metadata"`
}

// DocumentSummary is one row in the paged Browse chunk stream: the fields the
// list needs, without the text. The stream is paged indefinitely, so Content and
// Metadata come from GetDocument for the selected chunk only.
type DocumentSummary struct {
	ID           int64  `json:"id"`
	SourceID     int64  `json:"sourceId"`
	CollectionID int64  `json:"collectionId"`
	ChunkIndex   int    `json:"chunkIndex"`
	Title        string `json:"title"`
	SourcePath   string `json:"sourcePath"`
	SourceType   string `json:"sourceType"`
}

// DocumentPage is one keyset page of a collection's chunk stream, in stream
// order. Before and After are opaque cursors for the neighbouring pages; ""
// means that side ends there.
type DocumentPage struct {
	Documents []DocumentSummary `json:"documents"`
	Before    string            `json:"before"`
	After     string            `json:"after"`
}

// IndexProgress is emitted during an index run.
type IndexProgress struct {
	Collection string `json:"collection"`
	Current    int    `json:"current"`
	Total      int    `json:"total"`
	Item       string `json:"item"`
}

// IndexComplete is emitted when an index run finishes.
type IndexComplete struct {
	Collection string   `json:"collection"`
	Indexed    int      `json:"indexed"`
	Skipped    int      `json:"skipped"`
	Errors     int      `json:"errors"`
	Messages   []string `json:"messages"`
}

// IndexFileProgress is emitted per successfully indexed file during a run,
// scoped to the collection being indexed so the frontend can increment its
// per-collection file count.
type IndexFileProgress struct {
	Collection string `json:"collection"`
	File       string `json:"file"`
	Indexed    int    `json:"indexed"`
	Total      int    `json:"total"`
}

// IndexCancelled is emitted when an index run is cancelled by the user.
type IndexCancelled struct {
	Collection string `json:"collection"`
	Indexed    int    `json:"indexed"`
	Skipped    int    `json:"skipped"`
	Errors     int    `json:"errors"`
}

// IndexFailed is emitted when an index run panics. The panic is caught so the
// app does not crash; this tells the frontend to clear the run state and show
// the error instead of sitting on "indexing" forever.
type IndexFailed struct {
	Collection string `json:"collection"`
	Message    string `json:"message"`
}

// IndexState is a snapshot of the active index run, returned by
// GetIndexingState so a frontend that reloads or reconnects mid-run can
// rebuild its indexing UI instead of showing nothing. Live updates still
// arrive as events; this is only the initial state on (re)load.
type IndexState struct {
	Active      bool                         `json:"active"`
	All         bool                         `json:"all"`
	Collections map[string]IndexFileProgress `json:"collections"`
}
