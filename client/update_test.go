package main

import (
	"testing"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.1.0", "1.1.0", 0},
		{"v1.2.0", "1.1.0", 1},
		{"1.0.9", "1.1.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.1", "1.1.0", 0},
		{"1.1.1", "1.1", 1},
	}
	for _, tc := range tests {
		got, err := compareSemver(tc.a, tc.b)
		if err != nil {
			t.Fatalf("compareSemver(%q,%q): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Fatalf("compareSemver(%q,%q)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseRelease(t *testing.T) {
	rel := ghRelease{
		TagName: "v1.2.0",
		Body:    "fixes",
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "entangled-server-linux-amd64", BrowserDownloadURL: "https://github.com/Warexpor/EntangledVPN/releases/download/v1.2.0/server"},
			{Name: "Entangled.exe", BrowserDownloadURL: "https://github.com/Warexpor/EntangledVPN/releases/download/v1.2.0/Entangled.exe"},
		},
	}
	info, err := parseRelease("1.1.0", rel)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "1.2.0" || info.AssetURL == "" {
		t.Fatalf("unexpected: %+v", info)
	}

	info, err = parseRelease("1.2.0", rel)
	if err != nil {
		t.Fatal(err)
	}
	if info.Available {
		t.Fatal("same version should not be available")
	}
}

func TestParseReleaseMissingAsset(t *testing.T) {
	rel := ghRelease{TagName: "v9.0.0"}
	_, err := parseRelease("1.0.0", rel)
	if err == nil {
		t.Fatal("expected missing asset error")
	}
}

func TestValidateDownloadURL(t *testing.T) {
	ok := []string{
		"https://github.com/Warexpor/EntangledVPN/releases/download/v1.1.0/Entangled.exe",
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/x",
	}
	for _, u := range ok {
		if err := validateDownloadURL(u); err != nil {
			t.Fatalf("%s: %v", u, err)
		}
	}
	if err := validateDownloadURL("http://evil.example/x"); err == nil {
		t.Fatal("expected reject http")
	}
	if err := validateDownloadURL("https://evil.example/Entangled.exe"); err == nil {
		t.Fatal("expected reject host")
	}
}
