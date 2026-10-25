package services

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"vectile/backend/ocr"
)

// OCRState is the OCR plugin as the UI needs to see it. Disk-derived fields
// (installed, platform, paths) are filled in on every call so a frontend that
// reloads mid-install rebuilds the right thing, and installing from another
// window shows up here.
type OCRState struct {
	Supported   bool     `json:"supported"`
	Platform    string   `json:"platform"`
	Version     string   `json:"version"`
	Installed   bool     `json:"installed"`
	Enabled     bool     `json:"enabled"`
	SizeBytes   int64    `json:"sizeBytes"`
	Dir         string   `json:"dir"`
	DownloadURL string   `json:"downloadUrl"`
	ReleaseURL  string   `json:"releaseUrl"`
	Languages   []string `json:"languages"`

	Installing bool    `json:"installing"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Speed      float64 `json:"speed"`
	Error      string  `json:"error"`
}

// OCRInstallProgress is emitted while the bundle downloads.
type OCRInstallProgress struct {
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Speed      float64 `json:"speed"`
}

// OCRInstallError is emitted when an install fails.
type OCRInstallError struct {
	Message string `json:"message"`
}

// OCRLanguageProgress is emitted while one language's data downloads.
type OCRLanguageProgress struct {
	Code       string  `json:"code"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percent    float64 `json:"percent"`
	Speed      float64 `json:"speed"`
}

// OCRLanguageEvent names the language a terminal event is about, so a UI with
// several rows can tell which one settled.
type OCRLanguageEvent struct {
	Code string `json:"code"`
}

// OCRLanguageError is emitted when a language download fails.
type OCRLanguageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OCRService installs and reports the Tesseract plugin. The work lives in
// backend/ocr; this layer adds the single-flight guard, the progress events,
// and the state the UI renders.
type OCRService struct{ core *Core }

// NewOCRService creates an OCRService bound to the shared core.
func NewOCRService(core *Core) *OCRService { return &OCRService{core: core} }

// installMu guards the one in-flight job plus its progress, so two downloads
// cannot overlap, Cancel targets the right one, and GetOCRState reads a stable
// snapshot.
var installMu sync.Mutex

// job is the download in flight: the bundle install (code "") or one language's
// data. The two share one slot because both write into the same plugin tree:
// installing the bundle replaces the whole version directory, which would
// delete a language download running underneath it.
type job struct {
	active    bool
	code      string
	cancel    context.CancelFunc
	lastAt    time.Time
	lastBytes int64

	// progress, replayed by GetOCRState while the bundle downloads
	downloaded int64
	total      int64
	percent    float64
	speed      float64
	err        string
}

var current job

// beginJob claims the single in-flight slot. code is "" for the bundle.
func beginJob(code string, total int64) (context.Context, error) {
	installMu.Lock()
	defer installMu.Unlock()
	if current.active {
		if current.code != "" {
			return nil, fmt.Errorf("a language download is already in progress")
		}
		return nil, fmt.Errorf("an install is already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	current = job{active: true, code: code, cancel: cancel, total: total, lastAt: time.Now()}
	return ctx, nil
}

// GetOCRState reports the plugin as it is right now.
func (s *OCRService) GetOCRState() OCRState {
	installMu.Lock()
	defer installMu.Unlock()
	st := OCRState{Error: current.err}
	if current.active && current.code == "" {
		st.Installing = true
		st.Downloaded, st.Total = current.downloaded, current.total
		st.Percent, st.Speed = current.percent, current.speed
	}
	return s.stamp(st)
}

// stamp fills in everything derived from disk and config, leaving the install
// progress fields from st alone.
func (s *OCRService) stamp(st OCRState) OCRState {
	st.Platform = ocr.Platform()
	st.Supported = st.Platform != ""
	st.Version = ocr.Version
	st.Installed = ocr.Installed()
	st.Dir = ocr.Dir()
	st.ReleaseURL = ocr.ReleaseURL()
	st.Languages = ocr.Languages()
	if st.Supported {
		st.SizeBytes = ocr.AssetSize(st.Platform)
		st.DownloadURL = ocr.AssetURL(st.Platform)
	}
	if cfg := s.core.Config(); cfg != nil {
		st.Enabled = cfg.OCR.Enabled
	}
	return st
}

// InstallOCR starts a background download of the bundle for this platform.
// Returns whether it started: false when one is already running, when the
// platform has no bundle, when it is already installed, or when an index run
// is in flight. Progress arrives as ocr:install-progress, then
// ocr:install-complete / ocr:install-failed / ocr:install-cancelled.
func (s *OCRService) InstallOCR() (bool, error) {
	platform := ocr.Platform()
	if platform == "" {
		return false, fmt.Errorf("no OCR download for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if ocr.Installed() {
		return false, fmt.Errorf("OCR is already installed")
	}
	if s.isIndexing() {
		return false, fmt.Errorf("cannot install OCR while an index run is in progress")
	}

	ctx, err := beginJob("", ocr.AssetSize(platform))
	if err != nil {
		return false, err
	}

	go s.runInstall(ctx, platform)
	return true, nil
}

// CancelOCRInstall cancels whichever download is in flight: the bundle, or one
// language's data.
func (s *OCRService) CancelOCRInstall() bool {
	installMu.Lock()
	defer installMu.Unlock()
	if current.cancel == nil {
		return false
	}
	current.cancel()
	return true
}

// ListOCRLanguages returns every language the UI can offer, English first. The
// data for most of them is downloaded on demand; nothing here touches disk.
func (s *OCRService) ListOCRLanguages() []ocr.Language {
	return ocr.Catalog()
}

// InstallOCRLanguage downloads one language's data into the installed bundle.
// Progress arrives as ocr:language-progress, then ocr:language-complete,
// ocr:language-failed or ocr:language-cancelled. Returns whether it started.
func (s *OCRService) InstallOCRLanguage(code string) (bool, error) {
	if !ocr.Installed() {
		return false, fmt.Errorf("install the OCR plugin first")
	}
	if s.isIndexing() {
		return false, fmt.Errorf("cannot download language data while an index run is in progress")
	}

	ctx, err := beginJob(code, 0)
	if err != nil {
		return false, err
	}

	go s.runLangInstall(ctx, code)
	return true, nil
}

// RemoveOCRLanguage deletes one language's data. English is refused: it is what
// a run falls back to when the configured list names nothing the bundle holds.
func (s *OCRService) RemoveOCRLanguage(code string) error {
	if s.isIndexing() {
		return fmt.Errorf("cannot remove language data while an index run is in progress")
	}
	installMu.Lock()
	busy := current.active && current.code == code
	installMu.Unlock()
	if busy {
		return fmt.Errorf("%s is still downloading", code)
	}
	return ocr.RemoveLanguage(code)
}

// RemoveOCR deletes the installed bundle. Scans stop being readable until it is
// installed again; the Settings card offers exactly that.
func (s *OCRService) RemoveOCR() error {
	if s.isIndexing() {
		return fmt.Errorf("cannot remove OCR while an index run is in progress")
	}
	return ocr.Uninstall()
}

// runInstall performs the install and emits exactly one terminal event. The
// install itself is atomic, so a failure or a cancel leaves nothing behind.
func (s *OCRService) runInstall(ctx context.Context, platform string) {
	err := ocr.Install(ctx, platform, s.reportProgress)
	switch {
	case err == nil:
		s.end("", "ocr:install-complete", nil)
	case ctx.Err() != nil:
		s.end("", "ocr:install-cancelled", nil)
	default:
		s.end(err.Error(), "ocr:install-failed", OCRInstallError{Message: err.Error()})
	}
}

// runLangInstall performs one language download and emits exactly one terminal
// event. The download itself writes to a .part file, so a failure or a cancel
// leaves the previous data alone.
func (s *OCRService) runLangInstall(ctx context.Context, code string) {
	err := ocr.InstallLanguage(ctx, code, func(downloaded, total int64) {
		s.reportLangProgress(code, downloaded, total)
	})
	switch {
	case err == nil:
		s.end("", "ocr:language-complete", OCRLanguageEvent{Code: code})
	case ctx.Err() != nil:
		s.end("", "ocr:language-cancelled", OCRLanguageEvent{Code: code})
	default:
		s.end("", "ocr:language-failed", OCRLanguageError{Code: code, Message: err.Error()})
	}
}

// reportProgress emits a throttled bundle-install progress event. Throttling
// happens in the downloader; this converts bytes into percent and speed.
func (s *OCRService) reportProgress(downloaded, total int64) {
	pct, speed := trackProgress(downloaded, total)
	if s.core.App != nil {
		s.core.App.Event.Emit("ocr:install-progress", OCRInstallProgress{
			Downloaded: downloaded, Total: total, Percent: pct, Speed: speed,
		})
	}
}

// reportLangProgress emits the same for one language's data.
func (s *OCRService) reportLangProgress(code string, downloaded, total int64) {
	pct, speed := trackProgress(downloaded, total)
	if s.core.App != nil {
		s.core.App.Event.Emit("ocr:language-progress", OCRLanguageProgress{
			Code: code, Downloaded: downloaded, Total: total, Percent: pct, Speed: speed,
		})
	}
}

// trackProgress records the running job's byte count and returns the percent and
// speed the UI shows.
func trackProgress(downloaded, total int64) (pct, speed float64) {
	installMu.Lock()
	defer installMu.Unlock()
	if total > 0 {
		pct = float64(downloaded) / float64(total) * 100
	}
	if elapsed := time.Since(current.lastAt).Seconds(); elapsed > 0 {
		speed = float64(downloaded-current.lastBytes) / elapsed
	}
	current.lastAt = time.Now()
	current.lastBytes = downloaded
	current.downloaded, current.total = downloaded, total
	current.percent, current.speed = pct, speed
	return pct, speed
}

// end clears the running job and emits its terminal event. errMsg is replayed
// by GetOCRState on the bundle card; a language failure travels only in its own
// event, so one missing language cannot mark the plugin as broken.
func (s *OCRService) end(errMsg, ev string, payload any) {
	installMu.Lock()
	current = job{err: errMsg}
	installMu.Unlock()

	if s.core.App == nil {
		return
	}
	s.core.App.Event.Emit(ev, payload)
	if ev == "ocr:install-complete" {
		s.core.sendNotification("ocr-installed", "OCR installed",
			"Scanned PDFs are read on the next re-index")
	}
}

func (s *OCRService) isIndexing() bool {
	s.core.indexMu.Lock()
	defer s.core.indexMu.Unlock()
	return s.core.indexing
}
