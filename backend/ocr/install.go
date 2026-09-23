package ocr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vectile/backend/appdata"
)

// Install downloads the archive for platform, verifies it against the
// release's SHA256SUMS, extracts it beside the target, proves the binary runs,
// and only then swaps it into place. An interrupted or failed install leaves
// nothing behind, and an installed tree is always one that has run.
//
// onProgress is called with (downloaded, total) at most every 100ms. total is
// 0 when the server sends no length. It may be nil.
func Install(ctx context.Context, platform string, onProgress func(downloaded, total int64)) error {
	a, ok := platforms[platform]
	if !ok {
		return fmt.Errorf("no OCR bundle for %s", platform)
	}

	root := filepath.Join(appdata.PluginsDir(), "tesseract")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	// The expected hash is fetched first so a tampered archive is never written.
	want, err := expectedSum(ctx, a.name)
	if err != nil {
		return err
	}

	// Downloads and the extraction scratch dir live beside the target, not in
	// the OS temp dir: same volume makes the final swap a plain rename.
	archive := filepath.Join(root, a.name+".part")
	defer os.Remove(archive)
	if err := fetch(ctx, AssetURL(platform), archive, want, onProgress); err != nil {
		return err
	}

	staging := Dir() + ".tmp"
	_ = os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := extract(archive, staging, a.isZip); err != nil {
		return err
	}

	bin := filepath.Join(staging, "bin", filepath.Base(BinPath()))
	if err := validateBin(bin, filepath.Join(staging, "tessdata")); err != nil {
		return err
	}

	// Windows rename refuses to overwrite, so clear the old tree first. Nothing
	// reads it between these two lines: the parser stats BinPath per page.
	_ = os.RemoveAll(Dir())
	return os.Rename(staging, Dir())
}

// validateBin proves a freshly extracted binary runs before it is kept. A var
// so tests can substitute a stub: a synthetic archive cannot contain a working
// Tesseract.
var validateBin = validate

// Uninstall removes every installed version.
func Uninstall() error {
	return os.RemoveAll(filepath.Join(appdata.PluginsDir(), "tesseract"))
}

// Sweep clears leftovers from an install interrupted by a crash. Call it once
// at startup. A missing plugins dir is not an error.
func Sweep() {
	root := filepath.Join(appdata.PluginsDir(), "tesseract")
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") || strings.HasSuffix(e.Name(), ".tmp") {
			_ = os.RemoveAll(filepath.Join(root, e.Name()))
		}
	}
}

// expectedSum fetches the release's SHA256SUMS and returns the hash listed for
// name. The file is ~500 bytes of `sha256sum` output.
func expectedSum(ctx context.Context, name string) (string, error) {
	data, err := get(ctx, ChecksumsURL())
	if err != nil {
		return "", fmt.Errorf("fetch SHA256SUMS: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && strings.TrimPrefix(f[1], "*") == name {
			return f[0], nil
		}
	}
	return "", fmt.Errorf("SHA256SUMS has no entry for %s", name)
}

func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// fetch streams url to dest and fails unless the bytes hash to want, so a
// truncated or swapped download never reaches the extractor. Streaming keeps
// memory flat regardless of archive size.
func fetch(ctx context.Context, url, dest, want string, onProgress func(int64, int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	sum := sha256.New()
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, emit: onProgress}
	if _, err := io.Copy(io.MultiWriter(f, sum), pr); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if got := hex.EncodeToString(sum.Sum(nil)); !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", want, got)
	}
	return nil
}

// progressReader reports bytes read at most every 100ms, which is roughly how
// fast the frontend can redraw a progress bar.
type progressReader struct {
	r     io.Reader
	total int64
	done  int64
	last  time.Time
	emit  func(downloaded, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.emit != nil && time.Since(p.last) >= 100*time.Millisecond {
		p.last = time.Now()
		p.emit(p.done, p.total)
	}
	return n, err
}

func extract(archive, dest string, isZip bool) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if isZip {
		return extractZip(archive, dest)
	}
	return extractTarGz(archive, dest)
}

// extractZip unpacks a Windows bundle. Compress-Archive stores no Unix mode,
// so the executable bit is set afterwards by chmodBin.
func extractZip(archive, dest string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := writeEntry(target, 0o644, func() (io.ReadCloser, error) { return f.Open() }); err != nil {
			return err
		}
	}
	return chmodBin(dest)
}

// extractTarGz unpacks a Unix bundle, keeping the modes tar recorded.
func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		target, err := safeJoin(dest, h.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeEntry(target, os.FileMode(h.Mode)&0o777, func() (io.ReadCloser, error) {
				return io.NopCloser(tr), nil
			}); err != nil {
				return err
			}
		}
	}
	return chmodBin(dest)
}

// writeEntry creates one file and streams an archive entry into it.
func writeEntry(target string, mode os.FileMode, open func() (io.ReadCloser, error)) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}

// chmodBin marks the extracted binary executable. Windows has no execute bit,
// so this is a no-op there and the error is ignored.
func chmodBin(dest string) error {
	if err := os.Chmod(filepath.Join(dest, "bin", filepath.Base(BinPath())), 0o755); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// safeJoin resolves an archive entry inside dest. Both archive formats nest
// everything under a single "tesseract/" root, which is stripped so the result
// looks like bin/tesseract and tessdata/eng.traineddata. An entry that would
// land outside dest is rejected, so a crafted archive cannot write elsewhere on
// disk (zip-slip). The archive root itself returns "".
func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "tesseract" {
		clean = "."
	}
	clean = strings.TrimPrefix(clean, "tesseract"+string(filepath.Separator))

	root := filepath.Clean(dest)
	if clean == "." {
		return "", nil
	}
	target := filepath.Join(root, clean)
	if !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe path in archive: %q", name)
	}
	return target, nil
}
