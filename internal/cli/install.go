package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wilberucx/dots/internal/config"
	"github.com/Wilberucx/dots/internal/plugins"
	"github.com/Wilberucx/dots/internal/system"
	"github.com/Wilberucx/dots/internal/template"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/yaml"
	"github.com/spf13/cobra"
)

func init() {
	installCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd)
	}
}

func runInstall(cmd *cobra.Command) error {
	ui.PrintHeader("Installing Dependencies")

	dryRun := boolFlag(cmd, "dry-run")
	modules := stringSliceFlag(cmd, "module")
	types := stringSliceFlag(cmd, "type")

	// Detect package manager
	manager := plugins.GetPackageManager()
	if manager == nil {
		ui.PrintError("No supported package manager found (pacman, apt, brew).")
		return fmt.Errorf("no supported package manager found")
	}

	ui.PrintInfo(fmt.Sprintf("Detected Package Manager: %s", ui.BoldStyle.Render(manager.Name())))
	ui.PrintInfo(fmt.Sprintf("Architecture: %s", ui.BoldStyle.Render(template.GetSystemArch())))

	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load dependencies from modules
	allDeps := loadDependencies(cfg, modules, types)

	if len(allDeps) == 0 {
		ui.PrintInfo("No dependencies found.")
		return nil
	}

	// Install each dependency
	for _, dep := range allDeps {
		installDep(dep, manager, dryRun)
	}

	ui.PrintSuccess("\nDependency installation process finished.")
	return nil
}

// loadDependencies scans modules and collects unique dependencies.
func loadDependencies(cfg *config.DotsConfig, modules, types []string) []yaml.Dependency {
	modDirs, err := cfg.GetModuleDirs(modules, types)
	if err != nil {
		return nil
	}

	var allDeps []yaml.Dependency
	seen := make(map[string]bool)

	for _, mod := range modDirs {
		yamlPath := filepath.Join(mod.Path, "path.yaml")
		deps, err := yaml.ParseDependencies(yamlPath)
		if err != nil || deps == nil {
			continue
		}
		for _, d := range deps {
			if !seen[d.Name] {
				allDeps = append(allDeps, d)
				seen[d.Name] = true
			}
		}
	}

	return allDeps
}

// installDep dispatches to the correct installer based on dep.Type.
func installDep(dep yaml.Dependency, manager plugins.PackageManager, dryRun bool) {
	switch dep.Type {
	case "git":
		installGitDep(dep, dryRun)
	case "binary":
		installBinaryDep(dep, dryRun)
	case "package":
		installPackageDep(dep, manager, dryRun)
	default:
		ui.PrintWarning(fmt.Sprintf("  [skip] %s: unknown type '%s'", dep.Name, dep.Type))
	}

	runPostInstall(dep, dryRun)
}

// ─── Git dependencies ────────────────────────────────────────────────────────

func installGitDep(dep yaml.Dependency, dryRun bool) {
	if dep.URL == "" || dep.Dest == "" {
		ui.PrintWarning(fmt.Sprintf("Skipping git dependency '%s': missing source or target.", dep.Name))
		return
	}

	expandedDest := system.ExpandPath(dep.Dest)

	if _, err := os.Stat(expandedDest); err == nil {
		ui.PrintInfo(fmt.Sprintf("  [skip] %s already exists at %s", dep.Name, shortDisplayPath(expandedDest, system.HomeDir())))
		return
	}

	ui.PrintInfo(fmt.Sprintf("  [git] Cloning %s to %s...", dep.Name, shortDisplayPath(expandedDest, system.HomeDir())))

	if dryRun {
		return
	}

	cmd := exec.Command("git", "clone", dep.URL, expandedDest)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		ui.PrintError(fmt.Sprintf("  Failed to install %s: %v", dep.Name, err))
		return
	}

	if dep.Ref != "" {
		ui.PrintInfo(fmt.Sprintf("  [git] Checking out ref: %s", dep.Ref))
		checkout := exec.Command("git", "-C", expandedDest, "checkout", dep.Ref)
		checkout.Stdout = nil
		checkout.Stderr = nil
		if err := checkout.Run(); err != nil {
			ui.PrintError(fmt.Sprintf("  Failed to checkout ref %s for %s: %v", dep.Ref, dep.Name, err))
			return
		}
	}

	ui.PrintSuccess(fmt.Sprintf("  Installed %s", dep.Name))
}

// ─── Package dependencies ────────────────────────────────────────────────────

func installPackageDep(dep yaml.Dependency, manager plugins.PackageManager, dryRun bool) {
	var pkgName string

	if dep.Managers != nil {
		name, ok := dep.Managers[manager.Name()]
		if !ok {
			// Manager not listed — try fallback
			if dep.Fallback != nil {
				ui.PrintInfo(fmt.Sprintf("  [%s] %s not available — using fallback (%s)", manager.Name(), dep.Name, dep.Fallback.Type))
				installDep(*dep.Fallback, manager, dryRun)
				return
			}
			ui.PrintWarning(fmt.Sprintf("  [skip] %s: not available for %s", dep.Name, manager.Name()))
			return
		}
		pkgName = name
	} else {
		pkgName = dep.Name
	}

	binaryName := dep.Bin
	if binaryName == "" {
		binaryName = dep.Name
	}

	if _, err := exec.LookPath(binaryName); err == nil {
		ui.PrintInfo(fmt.Sprintf("  [skip] %s already installed", dep.Name))
		return
	}

	cmd := manager.InstallCommand([]string{pkgName})
	if manager.NeedsSudo() {
		cmd = append([]string{"sudo"}, cmd...)
	}

	ui.PrintInfo(fmt.Sprintf("  [pkg] Installing %s as '%s' via %s...", dep.Name, pkgName, manager.Name()))

	if dryRun {
		ui.PrintInfo(fmt.Sprintf("  [DRY] Would run: %s", strings.Join(cmd, " ")))
		return
	}

	installCmd := exec.Command(cmd[0], cmd[1:]...)
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		ui.PrintError(fmt.Sprintf("  Failed to install %s: %v", dep.Name, err))
		return
	}

	ui.PrintSuccess(fmt.Sprintf("  Installed %s", dep.Name))
}

// ─── Binary dependencies ─────────────────────────────────────────────────────

func installBinaryDep(dep yaml.Dependency, dryRun bool) {
	if dep.URL == "" || dep.Dest == "" {
		ui.PrintWarning(fmt.Sprintf("Skipping binary dependency '%s': missing source or target.", dep.Name))
		return
	}

	expandedDest := system.ExpandPath(dep.Dest)

	if _, err := os.Stat(expandedDest); err == nil {
		ui.PrintInfo(fmt.Sprintf("  [skip] %s already exists at %s", dep.Name, shortDisplayPath(expandedDest, system.HomeDir())))
		return
	}

	// Build template context
	ctx := template.BuildContext(dep.Version, dep.Arch)
	url := template.Render(dep.URL, ctx)
	extract := ""
	if dep.Extract != "" {
		extract = template.Render(dep.Extract, ctx)
	}

	ui.PrintInfo(fmt.Sprintf("  [bin] Downloading %s from %s...", dep.Name, url))

	if dryRun {
		return
	}

	if err := downloadAndExtract(url, expandedDest, extract); err != nil {
		ui.PrintError(fmt.Sprintf("  Failed to install %s: %v", dep.Name, err))
		return
	}

	ui.PrintSuccess(fmt.Sprintf("  Installed %s", dep.Name))
}

// downloadAndExtract downloads a URL and either extracts (tar.gz/tgz) or saves directly.
func downloadAndExtract(url, dest, extract string) error {
	// Create parent dir
	parentDir := filepath.Dir(dest)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("creating parent dir: %w", err)
	}

	// Download to temp file using net/http with 30s timeout
	tmpFile := dest + ".download.tmp"
	defer os.Remove(tmpFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("writing download: %w", err)
	}

	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") || extract != "" {
		if extract != "" {
			// Extract only the specified member
			// Use tar to list members and extract the specific one
			listCmd := exec.Command("tar", "-tzf", tmpFile)
			listOut, listErr := listCmd.Output()
			if listErr != nil {
				return fmt.Errorf("listing archive contents: %w", listErr)
			}

			found := false
			for _, member := range strings.Split(string(listOut), "\n") {
				member = strings.TrimSpace(member)
				if member == extract || strings.HasSuffix(member, "/"+extract) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("extract '%s' not found in archive. Available: %s", extract, strings.ReplaceAll(string(listOut), "\n", ", "))
			}

			// Extract the specific member
			extractCmd := exec.Command("tar", "-xzf", tmpFile, "-C", parentDir, extract)
			extractCmd.Stderr = nil
			if err := extractCmd.Run(); err != nil {
				return fmt.Errorf("extracting '%s': %w", extract, err)
			}

			// Move to final dest if needed
			extractedPath := filepath.Join(parentDir, extract)
			if extractedPath != dest {
				if err := os.Rename(extractedPath, dest); err != nil {
					return fmt.Errorf("moving extracted file: %w", err)
				}
			}
		} else {
			// Extract all to parent dir
			extractCmd := exec.Command("tar", "-xzf", tmpFile, "-C", parentDir)
			extractCmd.Stderr = nil
			if err := extractCmd.Run(); err != nil {
				return fmt.Errorf("extracting archive: %w", err)
			}
		}
	} else if strings.HasSuffix(url, ".zip") {
		extractCmd := exec.Command("unzip", "-o", tmpFile, "-d", parentDir)
		extractCmd.Stderr = nil
		if err := extractCmd.Run(); err != nil {
			return fmt.Errorf("extracting zip: %w", err)
		}
	} else {
		// Raw binary — move to destination
		if err := os.Rename(tmpFile, dest); err != nil {
			return fmt.Errorf("moving binary: %w", err)
		}
		if err := os.Chmod(dest, 0755); err != nil {
			return fmt.Errorf("chmod binary: %w", err)
		}
	}

	return nil
}

// ─── Post-install ────────────────────────────────────────────────────────────

func runPostInstall(dep yaml.Dependency, dryRun bool) {
	if dep.PostInstall == "" {
		return
	}

	ui.PrintInfo(fmt.Sprintf("  [exec] Running post-install for %s...", dep.Name))
	if dryRun {
		ui.PrintInfo(fmt.Sprintf("  [DRY] Would run: %s", dep.PostInstall))
		return
	}

	cmd := exec.Command("sh", "-c", dep.PostInstall)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		ui.PrintWarning(fmt.Sprintf("  [exec] post-install for %s exited with: %v", dep.Name, err))
	}
}
