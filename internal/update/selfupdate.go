package update

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// AssetName is the release archive for this platform, e.g.
// spawnpoint_darwin_arm64.tar.gz (see .goreleaser.yaml).
func AssetName() string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("spawnpoint_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
}

var downloadClient = &http.Client{Timeout: 2 * time.Minute}

func download(url string) ([]byte, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// SelfUpdate downloads a release, verifies its checksum and atomically
// replaces exe.
func SelfUpdate(version, exe string) error {
	base := fmt.Sprintf("%s/download/v%s", releasesURL(), version)
	asset := AssetName()

	sums, err := download(base + "/checksums.txt")
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}
	want := checksumFor(sums, asset)
	if want == "" {
		return fmt.Errorf("no release build for %s/%s (%s)", runtime.GOOS, runtime.GOARCH, asset)
	}

	archive, err := download(base + "/" + asset)
	if err != nil {
		return err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("checksum mismatch for %s", asset)
	}

	bin, err := extractBinary(archive, asset)
	if err != nil {
		return err
	}
	return replace(exe, bin)
}

func checksumFor(sums []byte, asset string) string {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0]
		}
	}
	return ""
}

func extractBinary(archive []byte, asset string) ([]byte, error) {
	name := "spawnpoint"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if strings.HasSuffix(asset, ".zip") {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != name {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
		return nil, fmt.Errorf("%s not found in %s", name, asset)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in %s", name, asset)
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == name {
			return io.ReadAll(tr)
		}
	}
}

// replace swaps exe for bin via a temp file and a rename in the same
// directory, so a failed update never leaves a half-written binary.
func replace(exe string, bin []byte) error {
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".spawnpoint-update-*")
	if err != nil {
		return permissionHint(err, dir)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		// A running .exe can't be overwritten, but it can be renamed.
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return permissionHint(err, dir)
		}
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return permissionHint(err, dir)
	}
	return nil
}

func permissionHint(err error, dir string) error {
	if os.IsPermission(err) {
		return fmt.Errorf("%w\n%s isn't writable: rerun with sudo, or reinstall with the install script", err, dir)
	}
	return err
}
