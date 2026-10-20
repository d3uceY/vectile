package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vectile/backend/appdata"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// ModelDownloadProgress is emitted during a model download.
type ModelDownloadProgress struct {
	Key        string  `json:"key"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Speed      float64 `json:"speed"`
}

// ModelDownloadState is a snapshot of the active download, returned by
// GetDownloadState so a reloading frontend can rebuild its progress UI.
type ModelDownloadState struct {
	Active     bool    `json:"active"`
	Key        string  `json:"key"`
	Status     string  `json:"status"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Speed      float64 `json:"speed"`
	Error      string  `json:"error"`
}

// ModelDownloadError is emitted when a model download fails.
type ModelDownloadError struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

// downloadStateMu guards the single in-flight download (catalog+state+cancel),
// so two downloads can't overlap and CancelModelDownload targets the right one.
var downloadStateMu sync.Mutex

type downloadState struct {
	active bool
	cancel context.CancelFunc
	key    string
	state  ModelDownloadState
}

var currentDownload downloadState

// modelDownloadEmitThrottle is how often progress events are sent (~10/s).
const modelDownloadEmitThrottle = 100 * time.Millisecond

// ListRecommendedModels returns the curated catalog of models the app can
// download directly.
func (s *ModelService) ListRecommendedModels() ([]CatalogModel, error) {
	return ModelCatalog(), nil
}

// DownloadModel starts a background download of a catalog model: HTTP fetch,
// SHA-256 verify, GGUF embedding check, then register + activate. Returns
// whether the download started (false when one is already running, an index
// run is in flight, or the model is already installed). Progress arrives as
// model:download-progress events; completion/failure as model:download-complete
// / model:download-failed.
func (s *ModelService) DownloadModel(key string) (bool, error) {
	if s.isIndexing() {
		return false, fmt.Errorf("cannot download a model while an index run is in progress")
	}
	cat, ok := CatalogByKey(key)
	if !ok {
		return false, fmt.Errorf("unknown model: %q", key)
	}
	if s.modelInstalled(cat.File) {
		return false, fmt.Errorf("%s is already installed", cat.Name)
	}
	downloadStateMu.Lock()
	if currentDownload.active {
		downloadStateMu.Unlock()
		return false, fmt.Errorf("a download is already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentDownload = downloadState{active: true, cancel: cancel, key: key,
		state: ModelDownloadState{Active: true, Key: key, Status: "downloading"}}
	downloadStateMu.Unlock()
	go s.runDownload(ctx, cat)
	return true, nil
}

// CancelModelDownload cancels the in-flight model download, if any.
func (s *ModelService) CancelModelDownload() bool {
	downloadStateMu.Lock()
	defer downloadStateMu.Unlock()
	if currentDownload.cancel == nil {
		return false
	}
	currentDownload.cancel()
	return true
}

// GetDownloadState returns a snapshot of the active download so a reloading
// frontend can rebuild its progress UI; live updates still arrive as events.
func (s *ModelService) GetDownloadState() ModelDownloadState {
	downloadStateMu.Lock()
	defer downloadStateMu.Unlock()
	return currentDownload.state
}

func (s *ModelService) modelInstalled(base string) bool {
	_, err := os.Stat(filepath.Join(appdata.ModelsDir(), base))
	return err == nil
}

func (s *ModelService) setDownloadState(st ModelDownloadState) {
	downloadStateMu.Lock()
	currentDownload.state = st
	downloadStateMu.Unlock()
}

// endDownload clears the in-flight download (releasing its cancel func so the
// next download can start) and emits the terminal event when ev != "".
func (s *ModelService) endDownload(st ModelDownloadState, ev string, payload any) {
	downloadStateMu.Lock()
	currentDownload = downloadState{state: st}
	downloadStateMu.Unlock()
	if s.core.App != nil && ev != "" {
		s.core.App.Event.Emit(ev, payload)
	}
}

// runDownload is the full download pipeline, run on its own goroutine. It
// downloads to a .part file, checks it is a text-embedding model, then
// registers and (when safe) activates it.
func (s *ModelService) runDownload(ctx context.Context, cat CatalogModel) {
	dir := appdata.ModelsDir()
	dest := filepath.Join(dir, cat.File)
	tmp := dest + ".part"
	_ = os.Remove(tmp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cat.URL, nil)
	if err != nil {
		s.abortDownload(cat, "", err.Error(), false)
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.abortDownload(cat, tmp, err.Error(), ctx.Err() != nil)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.abortDownload(cat, tmp, "server returned "+resp.Status, false)
		return
	}

	f, err := os.Create(tmp)
	if err != nil {
		s.abortDownload(cat, tmp, err.Error(), false)
		return
	}
	total := resp.ContentLength
	var downloaded int64
	lastBytes, lastTime, lastEmit := int64(0), time.Now(), time.Now()
	buf := make([]byte, 64*1024)

	// Hash what is written, so the checksum costs no second pass over the file
	// and no extra memory. The body is bounded at the pinned size: a server
	// that keeps sending (or a response with no length) must not be able to
	// fill the disk before the checks below run.
	h := sha256.New()
	body := io.Reader(resp.Body)
	if cat.SizeBytes > 0 {
		body = io.LimitReader(resp.Body, cat.SizeBytes+1) // the extra byte detects oversize
	}
	w := io.MultiWriter(f, h)

	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				f.Close()
				s.abortDownload(cat, tmp, werr.Error(), false)
				return
			}

			downloaded += int64(n)
			if time.Since(lastEmit) >= modelDownloadEmitThrottle {
				percent := float64(0)
				if total > 0 {
					percent = float64(downloaded) / float64(total) * 100
				}
				speed := float64(downloaded-lastBytes) / time.Since(lastTime).Seconds()
				lastBytes, lastTime = downloaded, time.Now()
				s.setDownloadState(ModelDownloadState{Active: true, Key: cat.Key, Status: "downloading",
					Downloaded: downloaded, Total: total, Percent: percent, Speed: speed})
				if s.core.App != nil {
					s.core.App.Event.Emit("model:download-progress", ModelDownloadProgress{
						Key: cat.Key, Downloaded: downloaded, Total: total, Percent: percent, Speed: speed,
					})
				}
				lastEmit = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			s.abortDownload(cat, tmp, rerr.Error(), ctx.Err() != nil)
			return
		}
	}
	if err := f.Close(); err != nil {
		s.abortDownload(cat, tmp, err.Error(), false)
		return
	}
	if ctx.Err() != nil {
		s.abortDownload(cat, tmp, "", true)
		return
	}

	// Nothing is parsed, renamed into models/, or activated until the bytes
	// match what the catalog pins. The URL is a moving ref (resolve/main), so a
	// mismatch means upstream replaced the file, or the transfer was cut short.
	if cat.SHA256 == "" {
		s.abortDownload(cat, tmp, "model has no pinned checksum", false)
		return
	}
	if cat.SizeBytes > 0 && downloaded != cat.SizeBytes {
		s.abortDownload(cat, tmp, fmt.Sprintf("download is %d bytes, expected %d", downloaded, cat.SizeBytes), false)
		return
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, cat.SHA256) {
		s.abortDownload(cat, tmp, fmt.Sprintf("checksum mismatch: got %s, expected %s", got, cat.SHA256), false)
		return
	}

	dims, context, err := embeddings.ReadMetadata(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		s.abortDownload(cat, "", "invalid GGUF: "+err.Error(), false)
		return
	}
	if dims <= 0 {
		_ = os.Remove(tmp)
		s.abortDownload(cat, "", "not a text-embedding model", false)
		return
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		s.abortDownload(cat, "", err.Error(), false)
		return
	}
	if _, err := db.UpsertModel(db.DB, db.Model{
		Name: strings.TrimSuffix(cat.File, ".gguf"), Path: dest,
		Dimensions: dims, ContextWindow: context, BatchSize: defaultBatchSize,
	}); err != nil {
		s.abortDownload(cat, "", err.Error(), false)
		return
	}
	m, ok, err := db.GetModelByPath(db.DB, dest)
	if err != nil || !ok {
		s.abortDownload(cat, "", "reload downloaded model", false)
		return
	}
	s.activateIfSafe(m)
	slog.Info("downloaded model", "name", m.Name, "dims", m.Dimensions)
	s.endDownload(ModelDownloadState{Active: false, Key: cat.Key, Status: "done"}, "model:download-complete", m)
	s.core.sendNotification("model-"+cat.Key, "Model ready", m.Name+" is ready")
}

// activateIfSafe makes a fresh download the active model unless that would
// strand existing embeddings: stored vectors belong to the model that produced
// them, and a different model cannot reuse them even when the dimensions match,
// so with embeddings present the download stays inactive and the user switches
// in Settings (where the reindex is confirmed).
func (s *ModelService) activateIfSafe(m db.Model) {
	vectorDim, err := db.GetVectorDim(db.DB)
	if err != nil {
		return
	}
	count, err := db.CountEmbeddings(db.DB)
	if err != nil {
		return
	}
	if count > 0 {
		differs, err := s.activeModelDiffers(m)
		if err != nil {
			return
		}
		if differs {
			slog.Info("downloaded model left inactive (stored vectors belong to another model)", "name", m.Name)
			return
		}
	}
	dimChange := m.Dimensions > 0 && vectorDim > 0 && m.Dimensions != vectorDim
	if err := s.applyActive(m, dimChange); err != nil {
		slog.Warn("activate downloaded model", "err", err)
	}
}

// abortDownload removes the partial file and reports a cancelled/failed
// download. cancel is true when the download was aborted by the user.
func (s *ModelService) abortDownload(cat CatalogModel, tmp, msg string, cancel bool) {
	if tmp != "" {
		_ = os.Remove(tmp)
	}
	st := ModelDownloadState{Active: false, Key: cat.Key}
	if cancel {
		st.Status = "cancelled"
		s.endDownload(st, "model:download-cancelled", cat.Key)
		return
	}
	st.Status = "failed"
	st.Error = msg
	s.endDownload(st, "model:download-failed", ModelDownloadError{Key: cat.Key, Message: msg})
}
