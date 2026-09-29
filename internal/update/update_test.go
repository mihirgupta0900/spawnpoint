package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "0.11.0", true},
		{"0.11.1", "0.11.0", true},
		{"0.11.0", "0.11.0", false},
		{"0.10.9", "0.11.0", false},
		{"v1.2.0", "1.1.9", true},
		{"1.0.0", "dev", false},
		{"1.0.0", "0.11.1.dev0+g4860bff", true},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestDetect(t *testing.T) {
	cases := []struct {
		exe, channel string
		fromGo       bool
		want         string
	}{
		{"/opt/homebrew/Cellar/spawnpoint/1.0.0/bin/spawnpoint", "", false, "homebrew"},
		{"/Users/x/.local/pipx/venvs/spawnpoint/bin/spawnpoint", "pypi", false, "pipx"},
		{"/Users/x/.local/share/uv/tools/spawnpoint/bin/spawnpoint", "pypi", false, "uv"},
		{"/Users/x/proj/.venv/bin/spawnpoint", "pypi", false, "pip"},
		{"/Users/x/go/bin/spawnpoint", "", true, "go"},
		{"/Users/x/.local/bin/spawnpoint", "", false, "binary"},
	}
	for _, c := range cases {
		if got := detect(c.exe, c.channel, c.fromGo).Name; got != c.want {
			t.Errorf("detect(%s) = %s, want %s", c.exe, got, c.want)
		}
	}
}

// TestSelfUpdate serves a fake release and checks the binary is swapped in
// place (and that a bad checksum leaves it untouched).
func TestSelfUpdate(t *testing.T) {
	newBin := []byte("#!/bin/sh\necho new\n")
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "spawnpoint", Mode: 0o755, Size: int64(len(newBin)), Typeflag: tar.TypeReg})
	tw.Write(newBin)
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(archive.Bytes())
	checksum := hex.EncodeToString(sum[:])

	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tag/v9.9.9", http.StatusFound)
	})
	mux.HandleFunc("/download/v9.9.9/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", checksum, AssetName())
	})
	mux.HandleFunc("/download/v9.9.9/"+AssetName(), func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive.Bytes())
	})
	mux.HandleFunc("/download/v6.6.6/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", "deadbeef", AssetName())
	})
	mux.HandleFunc("/download/v6.6.6/"+AssetName(), func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive.Bytes())
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("SPAWNPOINT_RELEASES_URL", srv.URL)

	if v, err := Latest(); err != nil || v != "9.9.9" {
		t.Fatalf("Latest() = %q, %v", v, err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("archive is tar.gz")
	}

	exe := filepath.Join(t.TempDir(), "spawnpoint")
	os.WriteFile(exe, []byte("old"), 0o755)

	if err := SelfUpdate("6.6.6", exe); err == nil {
		t.Fatal("bad checksum accepted")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatal("binary changed despite bad checksum")
	}

	if err := SelfUpdate("9.9.9", exe); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(exe)
	info, _ := os.Stat(exe)
	if !bytes.Equal(got, newBin) || info.Mode().Perm() != 0o755 {
		t.Fatalf("binary = %q mode %v", got, info.Mode())
	}
}
