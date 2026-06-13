package ruleset

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	managerconfig "github.com/openwrt-singbox/singbox-manager/internal/config"
)

const (
	DefaultDirectory = "/etc/singbox-manager/rulesets"
	maxDownloadSize  = 64 << 20
)

type Result struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}

func Download(ctx context.Context, ruleset managerconfig.RuleSet) (Result, error) {
	if !ruleset.Enabled {
		return Result{}, fmt.Errorf("ruleset %q is disabled", ruleset.ID)
	}
	if ruleset.Type != "remote" {
		return Result{}, fmt.Errorf("ruleset %q is not remote", ruleset.ID)
	}
	if ruleset.URL == "" {
		return Result{}, fmt.Errorf("ruleset %q url is required", ruleset.ID)
	}
	parsed, err := url.Parse(ruleset.URL)
	if err != nil {
		return Result{}, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Result{}, fmt.Errorf("ruleset URL must use http or https")
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ruleset.URL, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("ruleset fetch failed: HTTP %d", resp.StatusCode)
	}

	path := Path(ruleset)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return Result{}, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".")
	if err != nil {
		return Result{}, err
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		_ = tmp.Close()
		return Result{}, err
	}
	if written > maxDownloadSize {
		_ = tmp.Close()
		return Result{}, fmt.Errorf("ruleset response exceeds %d bytes", maxDownloadSize)
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return Result{}, err
	}
	if err := tmp.Close(); err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return Result{}, err
	}
	return Result{ID: ruleset.ID, Path: path, Bytes: written}, nil
}

func Path(ruleset managerconfig.RuleSet) string {
	if ruleset.Path != "" {
		return ruleset.Path
	}
	name := strings.TrimSpace(ruleset.ID)
	if name == "" {
		name = "ruleset"
	}
	if filepath.Ext(name) == "" {
		name += ".srs"
	}
	return filepath.Join(DefaultDirectory, filepath.Base(name))
}

func Due(ruleset managerconfig.RuleSet, now time.Time) bool {
	if !ruleset.Enabled || ruleset.Type != "remote" || ruleset.URL == "" {
		return false
	}
	if ruleset.LastUpdate == "" {
		return true
	}
	updated, err := time.Parse(time.RFC3339, ruleset.LastUpdate)
	if err != nil {
		return true
	}
	interval, err := time.ParseDuration(ruleset.UpdateInterval)
	if err != nil || interval <= 0 {
		interval = 168 * time.Hour
	}
	if interval < time.Hour {
		interval = time.Hour
	}
	return !updated.Add(interval).After(now)
}
