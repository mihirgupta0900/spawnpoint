// Package update checks for new releases and upgrades spawnpoint in place,
// using whichever tool installed it (Homebrew, pipx, uv, pip, go install) or
// replacing the binary directly for curl/manual installs.
package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mihirgupta0900/spawnpoint/internal/config"
	"github.com/mihirgupta0900/spawnpoint/internal/ui"
)

const (
	Repo     = "mihirgupta0900/spawnpoint"
	cacheTTL = 24 * time.Hour
)

// releasesURL is the base for release lookups and downloads; overridable for
// tests via SPAWNPOINT_RELEASES_URL.
func releasesURL() string {
	if u := os.Getenv("SPAWNPOINT_RELEASES_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://github.com/" + Repo + "/releases"
}

var client = &http.Client{Timeout: 3 * time.Second}

// Latest returns the newest release version (without the leading "v").
// It follows the /releases/latest redirect instead of calling the GitHub API,
// so it isn't subject to API rate limits.
func Latest() (string, error) {
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := noRedirect.Head(releasesURL() + "/latest")
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	i := strings.LastIndex(loc, "/tag/")
	if i < 0 {
		return "", fmt.Errorf("unexpected response from %s (status %d)", releasesURL(), resp.StatusCode)
	}
	return strings.TrimPrefix(loc[i+len("/tag/"):], "v"), nil
}

// The cache format matches the Python release's .version_cache.json.
type cache struct {
	Latest    string  `json:"latest_version"`
	CheckedAt float64 `json:"checked_at"`
}

func cachePath() string { return filepath.Join(config.Dir(), ".version_cache.json") }

func readCache() string {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return ""
	}
	var c cache
	if json.Unmarshal(data, &c) != nil {
		return ""
	}
	if time.Since(time.Unix(int64(c.CheckedAt), 0)) > cacheTTL {
		return ""
	}
	return c.Latest
}

func writeCache(latest string) {
	data, _ := json.Marshal(cache{latest, float64(time.Now().Unix())})
	_ = os.MkdirAll(config.Dir(), 0o755)
	_ = os.WriteFile(cachePath(), data, 0o644)
}

// Checker looks up the latest version in the background so it never slows
// a command down.
type Checker struct {
	current string
	done    chan struct{}
	latest  string
}

func NewChecker(current string) *Checker { return &Checker{current: current} }

func (c *Checker) Start() {
	c.done = make(chan struct{})
	go func() {
		defer close(c.done)
		if v := readCache(); v != "" {
			c.latest = v
			return
		}
		if v, err := Latest(); err == nil && v != "" {
			c.latest = v
			writeCache(v)
		}
	}()
}

// Notice returns an "update available" line, or "" (also if the check hasn't
// finished within a moment).
func (c *Checker) Notice() string {
	if c.done == nil {
		return ""
	}
	select {
	case <-c.done:
	case <-time.After(300 * time.Millisecond):
		return ""
	}
	if !Newer(c.latest, c.current) {
		return ""
	}
	return ui.Dim(fmt.Sprintf("Update available: %s → %s  (run ", c.current, c.latest)) + ui.Bold("sp update") + ui.Dim(" to upgrade)")
}

// Newer reports whether version a is strictly newer than b. Unparseable
// versions (dev builds) never count as newer or older.
func Newer(a, b string) bool {
	pa, okA := parse(a)
	pb, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := range pa {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parse(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return out, false
	}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
