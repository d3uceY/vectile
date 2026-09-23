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
	Supported   bool   `json:"supported"`
	Platform    string `json:"platform"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Enabled     bool   `json:"enabled"`
	SizeBytes   int64  `json:"sizeBytes"`
	Dir         string `json:"dir"`
	DownloadURL string `json:"downloadUrl"`
	ReleaseURL  string `json:"releaseUrl"`

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

// OCRService installs and reports the Tesseract plugin. The work lives in
// backend/ocr; this layer adds the single-flight guard, the progress events,
// and the state the UI renders.
type OCRService struct{ core *Core }

// NewOCRService creates an OCRService bound to the shared core.
func NewOCRService(core *Core) *OCRService { return &OCRService{core: core} }

// installMu guards the one in-flight install plus the snapshot of it, so two
// installs cannot overlap and Cancel targets the right one.
var installMu sync.Mutex

type ocrInstall struct {
	active    bool
	cancel    context.CancelFunc
	state     OCRState
	lastAt    time.Time
	lastBytes int64
}

var currentInstall ocrInstall

// GetOCRState reports the plugin as it is right now.
func (s *OCRService) GetOCRState() OCRState {
	installMu.Lock()
	defer installMu.Unlock()
	return s.stamp(currentInstall.state)
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
	if st.Supported {
		st.SizeBytes = ocr.AssetSize(st.Platform)
		st.DownloadURL = ocr.AssetURL(st.Platform)
	}
	st.Enabled = s.core.Cfg != nil && s.core.Cfg.OCR.Enabled
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

	installMu.Lock()
	if currentInstall.active {
		installMu.Unlock()
		return false, fmt.Errorf("an install is already in progress")
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentInstall = ocrInstall{
		active: true,
		cancel: cancel,
		state:  OCRState{Installing: true, Total: ocr.AssetSize(platform)},
		lastAt: time.Now(),
	}
	installMu.Unlock()

	go s.runInstall(ctx, platform)
	return true, nil
}

// CancelOCRInstall cancels an in-flight install, if any.
func (s *OCRService) CancelOCRInstall() bool {
	installMu.Lock()
	defer installMu.Unlock()
	if currentInstall.cancel == nil {
		return false
	}
	currentInstall.cancel()
	return true
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
		s.endInstall(OCRState{}, "ocr:install-complete", nil)
	case ctx.Err() != nil:
		s.endInstall(OCRState{}, "ocr:install-cancelled", nil)
	default:
		s.endInstall(OCRState{Error: err.Error()}, "ocr:install-failed",
			OCRInstallError{Message: err.Error()})
	}
}

// reportProgress emits a throttled progress event. Throttling happens in the
// downloader; this converts bytes into the percent and speed the UI shows.
func (s *OCRService) reportProgress(downloaded, total int64) {
	installMu.Lock()
	prev := currentInstall
	st := OCRState{Installing: true, Downloaded: downloaded, Total: total}
	if total > 0 {
		st.Percent = float64(downloaded) / float64(total) * 100
	}
	if elapsed := time.Since(prev.lastAt).Seconds(); elapsed > 0 {
		st.Speed = float64(downloaded-prev.lastBytes) / elapsed
	}
	currentInstall.state = st
	currentInstall.lastAt = time.Now()
	currentInstall.lastBytes = downloaded
	installMu.Unlock()

	if s.core.App != nil {
		s.core.App.Event.Emit("ocr:install-progress", OCRInstallProgress{
			Downloaded: downloaded, Total: total, Percent: st.Percent, Speed: st.Speed,
		})
	}
}

// endInstall clears the in-flight install (releasing its cancel func) and
// emits the terminal event when ev != "".
func (s *OCRService) endInstall(st OCRState, ev string, payload any) {
	installMu.Lock()
	currentInstall = ocrInstall{state: st}
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
