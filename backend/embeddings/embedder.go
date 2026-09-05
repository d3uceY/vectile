// Package embeddings wraps the in-process llama.go model (bge-m3) for
// sentence embeddings. The model loads lazily on first use and stays
// resident; inference is serialized because llama.go's Context is not safe
// for concurrent use.
package embeddings

import (
	"fmt"
	"os"
	"runtime"
	"sync"

	llama "github.com/tcpipuk/llama-go"
)

// State describes the embedder's lifecycle for the UI status pill.
type State string

const (
	StateIdle   State = "idle"
	StateLoaded State = "loaded"
	StateFailed State = "failed"
)

// Embedder lazily loads the llama.go model + context on first embed.
type Embedder struct {
	mu         sync.Mutex
	modelPath  string
	model      *llama.Model
	ctx        *llama.Context
	loaded     bool
	loadFailed bool
	loadErr    error

	// ctxSize is the model context window in tokens (0 = the model's native
	// maximum, resolved by llama.cpp after load); threads is the CPU thread
	// count (0 = runtime.NumCPU()). Both are set per model by SetModel /
	// NewEmbedder.
	ctxSize int
	threads int

	// inferMu serializes inference: the indexer embeds chunks in the
	// background while a search embeds its query on demand.
	inferMu sync.Mutex
}

// NewEmbedder returns an Embedder that loads modelPath on first embed. ctxSize
// is the context window in tokens (0 = the model's native maximum, resolved by
// llama.cpp after load); threads is the CPU thread count (0 = runtime.NumCPU()).
func NewEmbedder(modelPath string, ctxSize, threads int) *Embedder {
	return &Embedder{modelPath: modelPath, ctxSize: ctxSize, threads: threads}
}

// State reports the embedder lifecycle state without loading the model.
func (e *Embedder) State() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loadFailed {
		return StateFailed
	}
	if e.loaded {
		return StateLoaded
	}
	return StateIdle
}

// LoadError returns the error that caused StateFailed, if any.
func (e *Embedder) LoadError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.loadErr
}

// ModelPath returns the configured model path.
func (e *Embedder) ModelPath() string { return e.modelPath }

// SetModel re-points the embedder at a different model, closing the current
// one. The new model loads lazily on the next embed with the given settings.
// Callers must ensure no index run is in flight, so a mid-run model change
// can't mix embedding dimensions.
func (e *Embedder) SetModel(path string, ctxSize, threads int) error {
	e.inferMu.Lock()
	defer e.inferMu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx != nil { 
		_ = e.ctx.Close()
		e.ctx = nil
	}
	if e.model != nil {
		_ = e.model.Close()
		e.model = nil
	}
	e.modelPath = path
	e.ctxSize = ctxSize
	e.threads = threads
	e.loaded = false
	e.loadFailed = false
	e.loadErr = nil
	return nil
}

// ensureLoaded loads the model once. Failures are cached so a missing or
// corrupt model isn't re-attempted on every job.
func (e *Embedder) ensureLoaded() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return nil
	}
	if e.loadFailed {
		return e.loadErr
	}

	if _, err := os.Stat(e.modelPath); err != nil {
		e.loadFailed = true
		e.loadErr = fmt.Errorf("embedding model not found at %q", e.modelPath)
		return e.loadErr
	}

	// CPU-only build (the vendored archives are CPU builds); WithMMap keeps
	// the weights mapped from disk instead of copying them into RAM.
	model, err := llama.LoadModel(e.modelPath, llama.WithGPULayers(0), llama.WithMMap(true))
	if err != nil {
		e.loadFailed = true
		e.loadErr = fmt.Errorf("load embedding model: %w", err)
		return e.loadErr
	}

	threads := e.threads
	if threads <= 0 {
		threads = runtime.NumCPU()
	}
	// ctxSize 0 is a sentinel llama.go resolves to the model's native maximum
	// context (llama_model_n_ctx_train) after load, so every model runs at its
	// real window instead of a hardcoded cap.
	ctxSize := e.ctxSize
	if ctxSize < 0 {
		ctxSize = 0
	}
	ctx, err := model.NewContext(
		llama.WithContext(ctxSize),
		llama.WithThreads(threads),
		llama.WithEmbeddings(), // required to get vectors back
	)
	if err != nil {
		_ = model.Close()
		e.loadFailed = true
		e.loadErr = fmt.Errorf("new embedding context: %w", err)
		return e.loadErr
	}

	e.model, e.ctx = model, ctx
	e.loaded = true
	return nil
}

// Embed returns the embedding vector for a single text.
func (e *Embedder) Embed(text string) ([]float32, error) {
	e.inferMu.Lock()
	defer e.inferMu.Unlock()
	if err := e.ensureLoaded(); err != nil {
		return nil, err
	}
	return e.ctx.GetEmbeddings(text)
}

// EmbedBatch returns one vector per input text in one model call.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	e.inferMu.Lock()
	defer e.inferMu.Unlock()
	if err := e.ensureLoaded(); err != nil {
		return nil, err
	}
	return e.ctx.GetEmbeddingsBatch(texts)
}

// Close releases the model and context.
func (e *Embedder) Close() {
	e.inferMu.Lock()
	defer e.inferMu.Unlock()
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ctx != nil {
		_ = e.ctx.Close()
		e.ctx = nil
	}
	if e.model != nil {
		_ = e.model.Close()
		e.model = nil
	}
	e.loaded = false
}
