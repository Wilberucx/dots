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
	luacfg "github.com/Wilberucx/dots/internal/lua"
	"github.com/Wilberucx/dots/internal/plugins"
	"github.com/Wilberucx/dots/internal/system"
	"github.com/Wilberucx/dots/internal/template"
	"github.com/Wilberucx/dots/internal/ui"
	"github.com/Wilberucx/dots/internal/yaml"
	"github.com/spf13/cobra"
)

func init() {
	installCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runInstall(cmd, args)
	}
}

func runInstall(cmd *cobra.Command, args []string) error {
	ui.PrintHeader("Installing Dependencies")

	dryRun := boolFlag(cmd, "dry-run")
	yes := boolFlag(cmd, "yes")
	modules := mergeModuleArgs(stringSliceFlag(cmd, "module"), args)
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

	// Show summary of what will be installed
	fmt.Println()
	ui.PrintInfo(fmt.Sprintf("The following %d dependencies will be installed:", len(allDeps)))
	for _, dep := range allDeps {
		var depType string
		switch dep.Type {
		case "package":
			depType = "pkg"
			if dep.Managers != nil {
				depType = fmt.Sprintf("pkg/%s", manager.Name())
			}
		case "git":
			depType = "git"
		case "binary":
			depType = "curl"
		}
		fmt.Printf("  • %s (%s)\n", dep.Name, depType)
	}
	fmt.Println()

	// Show detailed commands for each dependency.
	// Post-install commands are shown inline when the effective dep is not skipped.
	for _, dep := range allDeps {
		displayDepCommands(dep, manager)
	}
	fmt.Println()

	// Show dry-run header — no commands are actually run
	if dryRun {
		ui.PrintWarning("Dry-run: no commands will be executed.")
		return nil
	}

	// Confirm before proceeding
	if !yes {
		if !ui.RunConfirm("Proceed with installation?", true) {
			ui.PrintInfo("Installation cancelled.")
			return nil
		}
	}

	// Install each dependency
	for _, dep := range allDeps {
		installDep(dep, manager, dryRun)
	}

	ui.PrintSuccess("\nDependency installation process finished.")
	return nil
}

// loadDependencies scans modules and collects unique dependencies.
// Supports both YAML (path.yaml) and Lua (dots.lua) modules.
func loadDependencies(cfg *config.DotsConfig, modules, types []string) []yaml.Dependency {
	modDirs, err := cfg.GetModuleDirs(modules, types)
	if err != nil {
		return nil
	}

	var allDeps []yaml.Dependency
	seen := make(map[string]bool)

	for _, mod := range modDirs {
		// Lua module: load dependencies from Lua config
		if mod.Type == int(luacfg.ModuleTypeLua) {
			deps, err := loadLuaDependencies(mod.Path)
			if err == nil && deps != nil {
				for _, d := range deps {
					if !seen[d.Name] {
						allDeps = append(allDeps, d)
						seen[d.Name] = true
					}
				}
			}
			continue
		}

		// YAML module: load dependencies from path.yaml
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

// loadLuaDependencies loads dependencies from a Lua module's dots.lua file.
func loadLuaDependencies(modPath string) ([]yaml.Dependency, error) {
	luaPath := filepath.Join(modPath, "dots.lua")
	if _, err := os.Stat(luaPath); os.IsNotExist(err) {
		return nil, nil
	}

	vm := luacfg.NewLuaVM()
	defer vm.Close()

	moduleCfg, err := vm.LoadModuleConfig(luaPath)
	if err != nil || moduleCfg == nil {
		return nil, err
	}

	var deps []yaml.Dependency
	for _, dep := range moduleCfg.Dependencies {
		yamlDep := yaml.Dependency{
			Name:        dep.Name,
			Type:        dep.Type,
			URL:         dep.URL,
			Dest:        dep.Destination,
			Version:     dep.Version,
			Ref:         dep.Ref,
			Extract:     dep.Extract,
			PostInstall: dep.PostInstall,
			Bin:         dep.Bin,
			Managers:    dep.Managers,
			Arch:        dep.Arch,
		}
		if dep.Fallback != nil {
			yamlDep.Fallback = &yaml.Dependency{
				Name:        dep.Fallback.Name,
				Type:        dep.Fallback.Type,
				URL:         dep.Fallback.URL,
				Dest:        dep.Fallback.Destination,
				Version:     dep.Fallback.Version,
				Ref:         dep.Fallback.Ref,
				Extract:     dep.Fallback.Extract,
				PostInstall: dep.Fallback.PostInstall,
				Bin:         dep.Fallback.Bin,
				Managers:    dep.Fallback.Managers,
				Arch:        dep.Fallback.Arch,
			}
		}
		deps = append(deps, yamlDep)
	}

	return deps, nil
}

// installDep dispatches to the correct installer based on dep.Type.
// It uses resolveInstallDecision to centralise skip/fallback logic so that
// the dry‑run preview (displayDepCommands) and actual execution always agree.
func installDep(dep yaml.Dependency, manager plugins.PackageManager, dryRun bool) {
	decision := resolveInstallDecision(dep, manager)

	if decision.SkipReason != "" {
		ui.PrintWarning(fmt.Sprintf("  [skip] %s: %s", dep.Name, decision.SkipReason))
		return
	}

	if decision.UsesFallback {
		ui.PrintInfo(fmt.Sprintf("  [%s] %s not available — using fallback (%s)", manager.Name(), dep.Name, decision.Dep.Type))
		installDep(decision.Dep, manager, dryRun)
		return
	}

	// Only run post-install for effective deps that are not skipped
	defer runPostInstall(decision.Dep, dryRun)

	switch decision.Dep.Type {
	case "git":
		installGitDep(decision.Dep, dryRun)
	case "binary":
		installBinaryDep(decision.Dep, dryRun)
	case "package":
		installPackageDep(decision.Dep, decision.PackageName, manager, dryRun)
	default:
		ui.PrintWarning(fmt.Sprintf("  [skip] %s: unknown type '%s'", dep.Name, decision.Dep.Type))
	}
}

// ─── Dependency decision ─────────────────────────────────────────────────────

// installDecision captures the resolved decision for a dependency.
// It is the single source of truth shared between preview and execution.
type installDecision struct {
	Dep          yaml.Dependency // the effective dep (original or fallback)
	PackageName  string          // resolved package name for "package" type
	UsesFallback bool            // true if a fallback is used instead of the original
	SkipReason   string          // non-empty means the dep should be skipped
}

// resolveInstallDecision centralises ALL skip / fallback / resolution logic
// for every dependency type. It is used by both the dry‑run preview
// (displayDepCommands) and the actual execution (installDep) so they
// always agree on what will happen.
//
// PackageName is only populated for package‑type deps that will actually be
// installed (not skipped, not using a fallback). For all other cases it is
// left empty, matching the old resolveInstallDep contract.
func resolveInstallDecision(dep yaml.Dependency, manager plugins.PackageManager) installDecision {
	decision := installDecision{
		Dep: dep,
	}

	switch dep.Type {
	case "git":
		if dep.URL == "" || dep.Dest == "" {
			decision.SkipReason = "missing source or target"
			return decision
		}
		expandedDest := system.ExpandPath(dep.Dest)
		if _, err := os.Stat(expandedDest); err == nil {
			decision.SkipReason = fmt.Sprintf("already exists at %s", shortDisplayPath(expandedDest, system.HomeDir()))
			return decision
		}
		return decision

	case "binary":
		if dep.URL == "" || dep.Dest == "" {
			decision.SkipReason = "missing source or target"
			return decision
		}
		expandedDest := system.ExpandPath(dep.Dest)
		if _, err := os.Stat(expandedDest); err == nil {
			decision.SkipReason = fmt.Sprintf("already exists at %s", shortDisplayPath(expandedDest, system.HomeDir()))
			return decision
		}
		return decision

	case "package":
		// Default: package name is the dep name (overridden if managers map is used)
		decision.PackageName = dep.Name

		// Resolve manager name
		if dep.Managers != nil {
			name, ok := dep.Managers[manager.Name()]
			if ok {
				decision.PackageName = name
			} else {
				// Manager not listed — try fallback
				if dep.Fallback != nil {
					decision.UsesFallback = true
					decision.Dep = *dep.Fallback
					decision.PackageName = "" // fallback is not a package dep
					return decision
				}
				decision.PackageName = "" // skip — no package to install
				decision.SkipReason = fmt.Sprintf("not available for %s", manager.Name())
				return decision
			}
		}

		// Check if binary already installed
		binaryName := dep.Bin
		if binaryName == "" {
			binaryName = dep.Name
		}
		if _, err := exec.LookPath(binaryName); err == nil {
			decision.SkipReason = "already installed"
			return decision
		}

		return decision

	default:
		decision.SkipReason = fmt.Sprintf("unknown type '%s'", dep.Type)
		return decision
	}
} // ─── Git dependencies ────────────────────────────────────────────────────────

// installGitDep clones a git repository. Caller must ensure dep.URL and dep.Dest
// are non‑empty and that Dest does not already exist (handled by resolveInstallDecision).
func installGitDep(dep yaml.Dependency, dryRun bool) {
	expandedDest := system.ExpandPath(dep.Dest)
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

// installPackageDep runs the package manager install command.
// Caller must ensure the dep is not skipped and managers are resolved
// (handled by resolveInstallDecision).
func installPackageDep(dep yaml.Dependency, pkgName string, manager plugins.PackageManager, dryRun bool) {
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

// installBinaryDep downloads a binary or archive. Caller must ensure dep.URL
// and dep.Dest are non‑empty and that Dest does not already exist
// (handled by resolveInstallDecision).
func installBinaryDep(dep yaml.Dependency, dryRun bool) {
	// Build template context
	ctx := template.BuildContext(dep.Version, dep.Arch)
	url := template.Render(dep.URL, ctx)
	extract := ""
	if dep.Extract != "" {
		extract = template.Render(dep.Extract, ctx)
	}
	expandedDest := system.ExpandPath(dep.Dest)

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

// displayDepCommands prints the exact external commands for a dependency.
// It uses resolveInstallDecision to show the real execution path
// (skip, fallback, or the actual commands), ensuring that the dry‑run
// preview always matches what installDep would do.
func displayDepCommands(dep yaml.Dependency, manager plugins.PackageManager) {
	decision := resolveInstallDecision(dep, manager)

	ui.PrintInfo(fmt.Sprintf("Dependency: %s (%s)", dep.Name, dep.Type))

	if decision.SkipReason != "" {
		// Show skip reason (same format as execution path)
		fmt.Printf("    [skip] %s: %s\n", dep.Name, decision.SkipReason)
		return
	}

	if decision.UsesFallback {
		fmt.Printf("    [%s] %s not available — using fallback (%s)\n", manager.Name(), dep.Name, decision.Dep.Type)
		// Recursively display the fallback decision
		displayDepCommands(decision.Dep, manager)
		return
	}

	// No skip, no fallback — show the exact commands
	printDepBody(decision.Dep, decision.PackageName, manager)
}

// printDepBody prints the installation commands for a dependency that is
// guaranteed not to be skipped. Used by displayDepCommands after resolution.
func printDepBody(dep yaml.Dependency, pkgName string, manager plugins.PackageManager) {
	switch dep.Type {
	case "git":
		expandedDest := system.ExpandPath(dep.Dest)
		fmt.Printf("    git clone %s %s\n", dep.URL, expandedDest)
		if dep.Ref != "" {
			fmt.Printf("    git -C %s checkout %s\n", expandedDest, dep.Ref)
		}

	case "binary":
		ctx := template.BuildContext(dep.Version, dep.Arch)
		url := template.Render(dep.URL, ctx)
		expandedDest := system.ExpandPath(dep.Dest)
		fmt.Printf("    download %s\n", url)
		fmt.Printf("      → %s\n", expandedDest)
		if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") || dep.Extract != "" {
			fmt.Printf("    tar -xzf ...\n")
		} else if strings.HasSuffix(url, ".zip") {
			fmt.Printf("    unzip -o ... -d %s\n", filepath.Dir(expandedDest))
		} else {
			fmt.Printf("    chmod 755 %s\n", expandedDest)
		}

	case "package":
		cmd := manager.InstallCommand([]string{pkgName})
		if manager.NeedsSudo() {
			cmd = append([]string{"sudo"}, cmd...)
		}
		fmt.Printf("    %s\n", strings.Join(cmd, " "))
	}

	if dep.PostInstall != "" {
		fmt.Printf("    sh -c '%s'\n", dep.PostInstall)
	}
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
