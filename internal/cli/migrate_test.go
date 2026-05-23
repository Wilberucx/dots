package cli

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// ─── Dependency migration ───────────────────────────────────────────────────

func TestMigrateSourceToURL(t *testing.T) {
	dep := map[string]interface{}{
		"name":   "bat",
		"source": "https://example.com/bat.tar.gz",
	}
	result := migrateDependency(dep)
	if result["url"] != dep["source"] {
		t.Errorf("expected url=%q, got %q", dep["source"], result["url"])
	}
	if _, ok := result["source"]; ok {
		t.Error("expected 'source' field to be removed")
	}
}

func TestMigrateTargetToDest(t *testing.T) {
	dep := map[string]interface{}{
		"name":   "bat",
		"target": "/usr/local/bin/bat",
	}
	result := migrateDependency(dep)
	if result["dest"] != "/usr/local/bin/bat" {
		t.Errorf("expected dest=/usr/local/bin/bat, got %q", result["dest"])
	}
	if _, ok := result["target"]; ok {
		t.Error("expected 'target' field to be removed")
	}
}

func TestMigrateExtractPathToExtract(t *testing.T) {
	dep := map[string]interface{}{
		"name":         "bat",
		"extract-path": "bat-v0.24.0/bat",
	}
	result := migrateDependency(dep)
	if result["extract"] != "bat-v0.24.0/bat" {
		t.Errorf("expected extract=%q, got %q", "bat-v0.24.0/bat", result["extract"])
	}
	if _, ok := result["extract-path"]; ok {
		t.Error("expected 'extract-path' field to be removed")
	}
}

func TestMigrateArchMapToArch(t *testing.T) {
	dep := map[string]interface{}{
		"name": "fd",
		"arch_map": map[string]interface{}{
			"x86_64":  "https://example.com/fd-x64.tar.gz",
			"aarch64": "https://example.com/fd-arm64.tar.gz",
		},
	}
	result := migrateDependency(dep)
	arch, ok := result["arch"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'arch' to be a map")
	}
	if arch["x86_64"] != "https://example.com/fd-x64.tar.gz" {
		t.Error("arch.x86_64 not preserved")
	}
	if _, ok := result["arch_map"]; ok {
		t.Error("expected 'arch_map' field to be removed")
	}
}

func TestMigratePackageManagersToManagers(t *testing.T) {
	dep := map[string]interface{}{
		"name": "bat",
		"package-managers": map[string]interface{}{
			"pacman": "bat",
			"apt":    "bat",
		},
	}
	result := migrateDependency(dep)
	mgr, ok := result["managers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'managers' to be a map")
	}
	if mgr["pacman"] != "bat" {
		t.Error("managers.pacman not preserved")
	}
	if _, ok := result["package-managers"]; ok {
		t.Error("expected 'package-managers' field to be removed")
	}
}

func TestMigrateTypeSystemToPackage(t *testing.T) {
	dep := map[string]interface{}{
		"name": "git",
		"type": "system",
	}
	result := migrateDependency(dep)
	if result["type"] != "package" {
		t.Errorf("expected type=package, got %q", result["type"])
	}
}

func TestMigratePreservesExistingV3Fields(t *testing.T) {
	dep := map[string]interface{}{
		"name": "bat",
		"url":  "https://example.com/bat.tar.gz",
		"dest": "/usr/bin/bat",
	}
	result := migrateDependency(dep)
	if result["url"] != "https://example.com/bat.tar.gz" {
		t.Error("url should be preserved")
	}
	if result["dest"] != "/usr/bin/bat" {
		t.Error("dest should be preserved")
	}
}

func TestMigrateFullDependencyV2ToV3(t *testing.T) {
	dep := map[string]interface{}{
		"name":             "bat",
		"type":             "system",
		"source":           "https://example.com/bat.tar.gz",
		"target":           "/usr/bin/bat",
		"extract-path":     "bat/bat",
		"arch_map":         map[string]interface{}{"x86_64": "https://x64.com"},
		"package-managers": map[string]interface{}{"pacman": "bat"},
	}
	result := migrateDependency(dep)
	if result["type"] != "package" {
		t.Error("type should be 'package'")
	}
	if result["url"] != "https://example.com/bat.tar.gz" {
		t.Error("url not mapped")
	}
	if result["dest"] != "/usr/bin/bat" {
		t.Error("dest not mapped")
	}
	if result["extract"] != "bat/bat" {
		t.Error("extract not mapped")
	}
}

// ─── File entry migration ───────────────────────────────────────────────────

func TestMigrateDestinationLinuxToPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source":            "config/linux.conf",
		"destination-linux": "/home/user/.config/app.conf",
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["linux"] != "/home/user/.config/app.conf" {
		t.Error("per-os.linux not set correctly")
	}
	if _, ok := result["destination-linux"]; ok {
		t.Error("expected 'destination-linux' to be removed")
	}
}

func TestMigrateDestinationMacToPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source":         "config/mac.conf",
		"destination-mac": "/Users/user/.config/app.conf",
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["mac"] != "/Users/user/.config/app.conf" {
		t.Error("per-os.mac not set correctly")
	}
}

func TestMigrateDestinationOverrideStringToPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source":               "config/app.conf",
		"destination-override": "/custom/path/app.conf",
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["linux"] != "/custom/path/app.conf" {
		t.Error("per-os.linux should be set from override string")
	}
	if perOS["mac"] != "/custom/path/app.conf" {
		t.Error("per-os.mac should be set from override string")
	}
}

func TestMigrateDestinationOverrideDictToPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source": "config/app.conf",
		"destination-override": map[string]interface{}{
			"linux": "/linux/path/app.conf",
			"mac":   "/mac/path/app.conf",
		},
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["linux"] != "/linux/path/app.conf" {
		t.Error("per-os.linux not set from dict")
	}
	if perOS["mac"] != "/mac/path/app.conf" {
		t.Error("per-os.mac not set from dict")
	}
}

func TestMigrateMergeMultipleDestinationsToPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source":            "config/app.conf",
		"destination-linux": "/linux/path/app.conf",
		"destination-mac":   "/mac/path/app.conf",
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["linux"] != "/linux/path/app.conf" {
		t.Error("per-os.linux not merged correctly")
	}
	if perOS["mac"] != "/mac/path/app.conf" {
		t.Error("per-os.mac not merged correctly")
	}
	if _, ok := result["destination-linux"]; ok {
		t.Error("destination-linux should be removed")
	}
	if _, ok := result["destination-mac"]; ok {
		t.Error("destination-mac should be removed")
	}
}

func TestMigratePreservesExistingPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source": "config.conf",
		"per-os": map[string]interface{}{
			"mac": "/existing/mac/path",
		},
		"destination-linux": "/new/linux/path",
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["mac"] != "/existing/mac/path" {
		t.Error("existing per-os.mac should be preserved")
	}
	if perOS["linux"] != "/new/linux/path" {
		t.Error("new per-os.linux should be added")
	}
}

func TestMigrateOverrideDictMergesWithExistingPerOS(t *testing.T) {
	entry := map[string]interface{}{
		"source": "config/app.conf",
		"per-os": map[string]interface{}{
			"windows": "/windows/path/app.conf",
		},
		"destination-override": map[string]interface{}{
			"linux": "/linux/path/app.conf",
			"mac":   "/mac/path/app.conf",
		},
	}
	result := migrateFileEntry(entry)
	perOS, ok := result["per-os"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'per-os' to exist")
	}
	if perOS["windows"] != "/windows/path/app.conf" {
		t.Error("existing per-os.windows should be preserved")
	}
	if perOS["linux"] != "/linux/path/app.conf" {
		t.Error("per-os.linux should be set from override dict")
	}
	if perOS["mac"] != "/mac/path/app.conf" {
		t.Error("per-os.mac should be set from override dict")
	}
	if len(perOS) != 3 {
		t.Errorf("expected 3 per-os entries, got %d", len(perOS))
	}
	if _, ok := result["destination-override"]; ok {
		t.Error("destination-override should be removed")
	}
}

// ─── File migration integration ─────────────────────────────────────────────

func TestMigrateFullPathYAMLv2(t *testing.T) {
	dir := t.TempDir()
	pathYAML := filepath.Join(dir, "path.yaml")

	v2Data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name":   "bat",
				"type":   "system",
				"source": "https://example.com/bat.tar.gz",
				"target": "/usr/bin/bat",
			},
			map[string]interface{}{
				"name": "git",
				"package-managers": map[string]interface{}{
					"pacman": "git",
					"apt":    "git",
				},
			},
		},
		"files": []interface{}{
			map[string]interface{}{
				"source":            "linux/config.conf",
				"destination-linux": "/home/user/.config/app.conf",
			},
			map[string]interface{}{
				"source":         "mac/config.conf",
				"destination-mac": "/Users/user/.config/app.conf",
			},
		},
	}

	out, _ := yaml.Marshal(v2Data)
	os.WriteFile(pathYAML, out, 0644)

	modified, err := migrateFile(pathYAML, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified {
		t.Fatal("expected file to be modified")
	}

	data, _ := os.ReadFile(pathYAML)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	deps := result["dependencies"].([]interface{})
	dep0 := deps[0].(map[string]interface{})
	if dep0["type"] != "package" {
		t.Error("dependency type should be 'package'")
	}
	if dep0["url"] != "https://example.com/bat.tar.gz" {
		t.Error("dependency url not mapped")
	}
	if dep0["dest"] != "/usr/bin/bat" {
		t.Error("dependency dest not mapped")
	}

	dep1 := deps[1].(map[string]interface{})
	if dep1["managers"] == nil {
		t.Error("dependency managers should exist")
	}

	files := result["files"].([]interface{})
	file0 := files[0].(map[string]interface{})
	perOS0 := file0["per-os"].(map[string]interface{})
	if perOS0["linux"] != "/home/user/.config/app.conf" {
		t.Error("file0 per-os.linux not set")
	}

	file1 := files[1].(map[string]interface{})
	perOS1 := file1["per-os"].(map[string]interface{})
	if perOS1["mac"] != "/Users/user/.config/app.conf" {
		t.Error("file1 per-os.mac not set")
	}
}

func TestMigrateAlreadyV3NoChange(t *testing.T) {
	dir := t.TempDir()
	pathYAML := filepath.Join(dir, "path.yaml")

	v3Data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name": "bat",
				"type": "package",
				"url":  "https://example.com/bat.tar.gz",
			},
		},
		"files": []interface{}{
			map[string]interface{}{
				"source":      "config.conf",
				"destination": "/home/user/.config/app.conf",
			},
		},
	}

	out, _ := yaml.Marshal(v3Data)
	os.WriteFile(pathYAML, out, 0644)

	modified, err := migrateFile(pathYAML, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified {
		t.Error("expected no modification for already v3 file")
	}

	// Verify content hasn't changed
	data, _ := os.ReadFile(pathYAML)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	if result["dependencies"].([]interface{})[0].(map[string]interface{})["type"] != "package" {
		t.Error("data should be unchanged")
	}
}

// ─── Dry-run ────────────────────────────────────────────────────────────────

func TestDryRunDoesNotModifyFile(t *testing.T) {
	dir := t.TempDir()
	pathYAML := filepath.Join(dir, "path.yaml")

	v2Data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name":   "bat",
				"type":   "system",
				"source": "https://example.com/bat.tar.gz",
			},
		},
		"files": []interface{}{
			map[string]interface{}{
				"source":            "config.conf",
				"destination-linux": "/home/user/.config/app.conf",
			},
		},
	}

	out, _ := yaml.Marshal(v2Data)
	os.WriteFile(pathYAML, out, 0644)

	modified, err := migrateFile(pathYAML, true) // dry-run
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified {
		t.Error("dry-run should report that file needs migration")
	}

	// File should be unchanged
	data, _ := os.ReadFile(pathYAML)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	deps := result["dependencies"].([]interface{})
	dep0 := deps[0].(map[string]interface{})
	if dep0["type"] != "system" {
		t.Error("dry-run should not modify type")
	}
	if _, ok := dep0["source"]; !ok {
		t.Error("dry-run should keep v2 fields")
	}
}

// ─── Idempotency ────────────────────────────────────────────────────────────

func TestMigrateTwiceIdempotent(t *testing.T) {
	dir := t.TempDir()
	pathYAML := filepath.Join(dir, "path.yaml")

	v2Data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name":   "bat",
				"type":   "system",
				"source": "https://example.com/bat.tar.gz",
			},
		},
		"files": []interface{}{
			map[string]interface{}{
				"source":            "config.conf",
				"destination-linux": "/home/user/.config/app.conf",
			},
		},
	}

	out, _ := yaml.Marshal(v2Data)
	os.WriteFile(pathYAML, out, 0644)

	// First migration - should modify
	modified1, err := migrateFile(pathYAML, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !modified1 {
		t.Error("first migration should modify")
	}

	// Second migration - should NOT modify
	modified2, err := migrateFile(pathYAML, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if modified2 {
		t.Error("second migration should be idempotent (no change)")
	}

	// Verify v3 fields are present
	data, _ := os.ReadFile(pathYAML)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	dep0 := result["dependencies"].([]interface{})[0].(map[string]interface{})
	if dep0["type"] != "package" {
		t.Error("type should be migrated")
	}
	if dep0["url"] != "https://example.com/bat.tar.gz" {
		t.Error("url should be migrated")
	}
}

func TestMigrateThreeTimesIdempotent(t *testing.T) {
	dir := t.TempDir()
	pathYAML := filepath.Join(dir, "path.yaml")

	v2Data := map[string]interface{}{
		"dependencies": []interface{}{
			map[string]interface{}{
				"name":   "bat",
				"source": "https://example.com/bat.tar.gz",
			},
		},
		"files": []interface{}{
			map[string]interface{}{
				"source":            "x",
				"destination-linux": "/linux/path",
			},
		},
	}

	out, _ := yaml.Marshal(v2Data)
	os.WriteFile(pathYAML, out, 0644)

	for i := 0; i < 3; i++ {
		migrateFile(pathYAML, false)
	}

	data, _ := os.ReadFile(pathYAML)
	var result map[string]interface{}
	yaml.Unmarshal(data, &result)

	dep0 := result["dependencies"].([]interface{})[0].(map[string]interface{})
	if dep0["url"] != "https://example.com/bat.tar.gz" {
		t.Error("url should be present after 3 migrations")
	}

	file0 := result["files"].([]interface{})[0].(map[string]interface{})
	perOS := file0["per-os"].(map[string]interface{})
	if perOS["linux"] != "/linux/path" {
		t.Error("per-os.linux should be present after 3 migrations")
	}
}

// ─── File discovery ─────────────────────────────────────────────────────────

func TestFindPathYAMLInSubdirectories(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "module1"), 0755)
	os.WriteFile(filepath.Join(dir, "module1", "path.yaml"), []byte("files: []"), 0644)

	os.MkdirAll(filepath.Join(dir, "module2", "nested"), 0755)
	os.WriteFile(filepath.Join(dir, "module2", "nested", "path.yaml"), []byte("files: []"), 0644)

	os.MkdirAll(filepath.Join(dir, "module3"), 0755)
	os.WriteFile(filepath.Join(dir, "module3", "path.yaml"), []byte("files: []"), 0644)

	results := findPathYAMLFiles(dir)
	if len(results) != 3 {
		t.Errorf("expected 3 path.yaml files, got %d", len(results))
	}
}

func TestFindPathYAMLNoneFound(t *testing.T) {
	dir := t.TempDir()
	results := findPathYAMLFiles(dir)
	if len(results) != 0 {
		t.Errorf("expected 0 path.yaml files, got %d", len(results))
	}
}

func TestFindPathYAMLSkipsDotGit(t *testing.T) {
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, ".git", "path.yaml"), []byte("files: []"), 0644)

	os.MkdirAll(filepath.Join(dir, "module"), 0755)
	os.WriteFile(filepath.Join(dir, "module", "path.yaml"), []byte("files: []"), 0644)

	results := findPathYAMLFiles(dir)
	if len(results) != 1 {
		t.Errorf("expected 1 path.yaml file (skip .git), got %d", len(results))
	}
}
