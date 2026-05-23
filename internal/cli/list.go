package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Wilberucx/dots/internal/resolver"
	"github.com/Wilberucx/dots/internal/system"
	"github.com/spf13/cobra"
)

func init() {
	listCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runList(cmd)
	}
}

func runList(cmd *cobra.Command) error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	linked := boolFlag(cmd, "linked")
	unlinked := boolFlag(cmd, "unlinked")
	broken := boolFlag(cmd, "broken")
	showVariant := boolFlag(cmd, "variant")
	showBackups := boolFlag(cmd, "backups")

	allModules, err := resolver.ResolveModules(cfg, nil, nil, "")
	if err != nil {
		return fmt.Errorf("resolving modules: %w", err)
	}

	results := make(map[string]bool)

	// If no flags, show all module names
	if !linked && !unlinked && !broken && !showVariant && !showBackups {
		for moduleName := range allModules {
			results[moduleName] = true
		}
	}

	// Single pass over modules
	for moduleName, statuses := range allModules {
		if linked {
			for _, st := range statuses {
				if st.State == resolver.StateLinked {
					results[moduleName] = true
					break
				}
			}
		}

		if unlinked {
			for _, st := range statuses {
				if st.State == resolver.StatePending {
					results[moduleName] = true
					break
				}
			}
		}

		if broken {
			for _, st := range statuses {
				if st.State == resolver.StateConflict || st.State == resolver.StateUnsafe {
					results[moduleName] = true
					break
				}
			}
		}

		if showVariant {
			vInfo, err := resolver.GetModuleVariantInfo(cfg, moduleName)
			if err == nil && vInfo != nil && vInfo.HasVariants {
				for _, v := range vInfo.Variants {
					results[fmt.Sprintf("%s:%s", moduleName, v)] = true
				}
			}
		}
	}

	// Search .orig files outside module loop
	if showBackups {
		home := cfg.HomeDir
		if home == "" {
			home = system.HomeDir()
		}
		filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if strings.HasSuffix(info.Name(), ".orig") {
				results[path] = true
			}
			return nil
		})
	}

	// Sort and print
	var sorted []string
	for item := range results {
		sorted = append(sorted, item)
	}
	sort.Strings(sorted)

	for _, item := range sorted {
		fmt.Println(item)
	}

	return nil
}
