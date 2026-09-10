package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"entangled-client/vpncore"
)

const (
	updateRepoAPI   = "https://api.github.com/repos/Warexpor/EntangledVPN/releases/latest"
	updateUserAgent = "EntangledVPN-Electron/" + vpncore.AppVersion
)

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
	return fmt.Errorf("in-app update is not supported in the Electron client yet; update via your package manager or GitHub releases")
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("github api: %s (%s)", resp.Status, strings.TrimSpace(string(body)))
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return out, err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	out.Latest = latest
	out.Notes = rel.Body
	out.Available = versionGreater(latest, current)
	want := "Entangled"
	if runtime.GOOS == "windows" {
		want = "Entangled.exe"
	}
	for _, asset := range rel.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, "electron") || strings.EqualFold(asset.Name, want) {
			out.AssetURL = asset.BrowserDownloadURL
			break
		}
	}
	return out, nil
}

func versionGreater(a, b string) bool {
	ap := splitVersion(a)
	bp := splitVersion(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return true
		}
		if av < bv {
			return false
		}
	}
	return false
}

func splitVersion(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				break
			}
			n = n*10 + int(c-'0')
		}
		out = append(out, n)
	}
	return out
}
