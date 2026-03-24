package firefox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestManifestDeclaresExpectedFirefoxExtensionStructure(t *testing.T) {
	manifestPath := filepath.Join(".", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		ManifestVersion int      `json:"manifest_version"`
		Name            string   `json:"name"`
		Version         string   `json:"version"`
		Permissions     []string `json:"permissions"`
		Background      struct {
			Scripts []string `json:"scripts"`
		} `json:"background"`
		OptionsUI struct {
			Page string `json:"page"`
		} `json:"options_ui"`
		ContentScripts []struct {
			Matches []string `json:"matches"`
			JS      []string `json:"js"`
		} `json:"content_scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	if manifest.ManifestVersion != 2 {
		t.Fatalf("expected manifest version 2, got %d", manifest.ManifestVersion)
	}
	if manifest.Name == "" {
		t.Fatal("expected extension name")
	}
	if manifest.Version == "" {
		t.Fatal("expected extension version")
	}
	if !slices.Contains(manifest.Permissions, "storage") {
		t.Fatalf("expected storage permission, got %v", manifest.Permissions)
	}
	if !slices.Contains(manifest.Permissions, "tabs") {
		t.Fatalf("expected tabs permission, got %v", manifest.Permissions)
	}
	if !slices.Contains(manifest.Permissions, "webNavigation") {
		t.Fatalf("expected webNavigation permission, got %v", manifest.Permissions)
	}
	if !slices.Contains(manifest.Permissions, "http://127.0.0.1/*") {
		t.Fatalf("expected 127.0.0.1 host permission, got %v", manifest.Permissions)
	}
	if !slices.Contains(manifest.Permissions, "http://localhost/*") {
		t.Fatalf("expected localhost host permission, got %v", manifest.Permissions)
	}
	if !slices.Contains(manifest.Background.Scripts, "background.js") {
		t.Fatalf("expected background.js in background scripts, got %v", manifest.Background.Scripts)
	}
	if manifest.OptionsUI.Page != "options.html" {
		t.Fatalf("expected options page options.html, got %q", manifest.OptionsUI.Page)
	}
	if len(manifest.ContentScripts) == 0 {
		t.Fatal("expected at least one content script")
	}
	if !slices.Contains(manifest.ContentScripts[0].Matches, "http://*/*") {
		t.Fatalf("expected http content script match, got %v", manifest.ContentScripts[0].Matches)
	}
	if !slices.Contains(manifest.ContentScripts[0].Matches, "https://*/*") {
		t.Fatalf("expected https content script match, got %v", manifest.ContentScripts[0].Matches)
	}
	if !slices.Contains(manifest.ContentScripts[0].JS, "content.js") {
		t.Fatalf("expected content.js in content scripts, got %v", manifest.ContentScripts[0].JS)
	}
}

func TestExtensionFilesExist(t *testing.T) {
	for _, name := range []string{"background.js", "content.js", "options.html", "options.js", "README.md"} {
		if _, err := os.Stat(filepath.Join(".", name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}

func TestExtensionScriptsExcludeLocalhostAndLoopbackPages(t *testing.T) {
	for _, name := range []string{"background.js", "content.js"} {
		data, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		source := string(data)
		for _, excludedHost := range []string{"localhost", "127.0.0.1", "::1"} {
			if !strings.Contains(source, excludedHost) {
				t.Fatalf("expected %s to exclude %q pages", name, excludedHost)
			}
		}
	}
}
