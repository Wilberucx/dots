package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	githubTagsURL = "https://api.github.com/repos/Wilberucx/dots/tags"
	cacheFileName = "dots-update.json"
	cacheTTL      = 24 * time.Hour
)

type updateCache struct {
	LatestVersion string `json:"latest_version"`
	LastCheck     int64  `json:"last_check"`
}

// checkForUpdates fetches the latest version from GitHub in a background goroutine.
func checkForUpdates() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	cachePath := filepath.Join(cacheDir, cacheFileName)

	// Check if cache is still fresh
	if fi, err := os.Stat(cachePath); err == nil {
		if time.Since(fi.ModTime()) < cacheTTL {
			return
		}
	}

	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(githubTagsURL)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return
		}

		var tags []struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			return
		}

		if len(tags) == 0 {
			return
		}

		latest := strings.TrimLeft(tags[0].Name, "v")
		if latest == "" {
			return
		}

		cache := updateCache{
			LatestVersion: latest,
			LastCheck:     time.Now().Unix(),
		}

		data, _ := json.Marshal(cache)
		os.MkdirAll(filepath.Dir(cachePath), 0755)
		os.WriteFile(cachePath, data, 0644)
	}()
}

// notifyIfNeeded prints a notification if a newer version is available.
func notifyIfNeeded() {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return
	}
	cachePath := filepath.Join(cacheDir, cacheFileName)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return
	}

	var cache updateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return
	}

	if cache.LatestVersion == "" {
		return
	}

	if isNewer(cache.LatestVersion, Version) {
		fmt.Println()
		fmt.Printf("✨ A new version of dots is available: v%s (current: %s)\n", cache.LatestVersion, Version)
		fmt.Printf("   Update with: curl -fsSL https://raw.githubusercontent.com/Wilberucx/dots/main/install.sh | bash\n")
	}
}

// isNewer compares two semantic version strings.
// Returns true if latest > current.
func isNewer(latest, current string) bool {
	lv := parseVersion(latest)
	cv := parseVersion(current)

	for i := 0; i < len(lv) && i < len(cv); i++ {
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return len(lv) > len(cv)
}

// parseVersion converts a version string like "0.8.1-dev" to ints.
func parseVersion(v string) []int {
	v = strings.TrimLeft(v, "v")
	parts := strings.Split(v, ".")
	var nums []int
	for _, p := range parts {
		// Handle suffixed versions like "0.8.1-dev"
		clean := p
		for i, c := range p {
			if c < '0' || c > '9' {
				clean = p[:i]
				break
			}
		}
		n := 0
		fmt.Sscanf(clean, "%d", &n)
		nums = append(nums, n)
	}
	return nums
}
