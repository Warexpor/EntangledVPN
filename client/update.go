package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"entangled-client/vpncore"
)

const (
	updateRepoAPI   = "https://api.github.com/repos/Warexpor/EntangledVPN/releases/latest"
	updateAssetName = "Entangled.exe"
	updateUserAgent = "EntangledVPN/" + vpncore.AppVersion
)

// UpdateInfo is returned by CheckForUpdate.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	AssetURL  string `json:"assetURL"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func (a *App) CheckForUpdate() (UpdateInfo, error) {
	info, err := fetchLatestUpdate()
	if err != nil {
		return UpdateInfo{Current: vpncore.AppVersion}, err
	}
	return info, nil
}

func (a *App) ApplyUpdate() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("in-app update is only supported on Windows")
	}
	info, err := fetchLatestUpdate()
	if err != nil {
		return err
	}
	if !info.Available || info.AssetURL == "" {
		return fmt.Errorf("no update available")
	}
	if err := applyUpdateFromURL(info.AssetURL); err != nil {
		return err
	}
	// Quit so the swap script can replace the locked exe.
	a.Quit()
	return nil
}

func fetchLatestUpdate() (UpdateInfo, error) {
	current := vpncore.AppVersion
	out := UpdateInfo{Current: current}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, updateRepoAPI, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", updateUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("github releases: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return out, err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return out, err
	}
	return parseRelease(current, rel)
}

func parseRelease(current string, rel ghRelease) (UpdateInfo, error) {
	out := UpdateInfo{Current: current}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if latest == "" {
		return out, fmt.Errorf("release has no tag")
	}
	out.Latest = latest
	out.Notes = truncateNotes(rel.Body, 2000)

	assetURL := ""
	for _, a := range rel.Assets {
		if a.Name == updateAssetName {
			assetURL = a.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return out, fmt.Errorf("%s not found in latest release", updateAssetName)
	}
	if err := validateDownloadURL(assetURL); err != nil {
		return out, err
	}
	out.AssetURL = assetURL

	cmp, err := compareSemver(latest, current)
	if err != nil {
		return out, err
	}
	out.Available = cmp > 0
	return out, nil
}

func truncateNotes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func validateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "https" {
		return fmt.Errorf("update URL must be https")
	}
	host := strings.ToLower(u.Host)
	switch host {
	case "github.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com":
		return nil
	default:
		if strings.HasSuffix(host, ".githubusercontent.com") {
			return nil
		}
		return fmt.Errorf("untrusted update host: %s", host)
	}
}

// compareSemver returns 1 if a>b, -1 if a<b, 0 if equal. Accepts optional "v" prefix.
func compareSemver(a, b string) (int, error) {
	pa, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if pa[i] > pb[i] {
			return 1, nil
		}
		if pa[i] < pb[i] {
			return -1, nil
		}
	}
	return 0, nil
}

func parseSemver(v string) ([3]int, error) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return out, fmt.Errorf("empty version")
	}
	parts := strings.Split(v, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return out, fmt.Errorf("bad version: %s", v)
	}
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 {
			return out, fmt.Errorf("bad version: %s", v)
		}
		out[i] = n
	}
	return out, nil
}

func applyUpdateFromURL(assetURL string) error {
	if err := validateDownloadURL(assetURL); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	newPath := exe + ".new"

	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, assetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", updateUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	if err := validateDownloadURL(resp.Request.URL.String()); err != nil {
		return err
	}

	f, err := os.OpenFile(newPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, io.LimitReader(resp.Body, 200<<20)) // 200 MiB cap
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(newPath)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(newPath)
		return closeErr
	}

	script := filepath.Join(os.TempDir(), "entangled-update.cmd")
	content := "" +
		"@echo off\r\n" +
		"setlocal\r\n" +
		"set \"EXE=%~1\"\r\n" +
		"set \"NEW=%~2\"\r\n" +
		"set \"PID=%~3\"\r\n" +
		":wait\r\n" +
		"ping -n 2 127.0.0.1 >nul\r\n" +
		"tasklist /FI \"PID eq %PID%\" 2>nul | find \"%PID%\" >nul\r\n" +
		"if not errorlevel 1 goto wait\r\n" +
		"move /Y \"%NEW%\" \"%EXE%\"\r\n" +
		"if errorlevel 1 goto wait\r\n" +
		"start \"\" \"%EXE%\"\r\n" +
		"del \"%~f0\"\r\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		os.Remove(newPath)
		return err
	}

	cmd := vpncore.HiddenCommand("cmd", "/C", "call", script, exe, newPath, strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		os.Remove(newPath)
		os.Remove(script)
		return err
	}
	return nil
}
