package main

import (
	"fmt"
	"strings"
)

// ModuleDef describes a module and its dependencies.
type ModuleDef struct {
	Name       string
	Required   bool
	DependsOn  []string
	Deprecated bool
	Group      string
}

// moduleRegistry maps module name to its definition.
var moduleRegistry = map[string]ModuleDef{
	// Core modules - always enabled, no flags
	"auth":         {Name: "auth", Required: true, Group: "core"},
	"bank":         {Name: "bank", Required: true, DependsOn: []string{"auth"}, Group: "core"},
	"consensus":    {Name: "consensus", Required: true, Group: "core"},
	"staking":      {Name: "staking", Required: true, DependsOn: []string{"auth", "bank"}, Group: "core"},
	"mint":         {Name: "mint", Required: true, DependsOn: []string{"staking", "auth", "bank"}, Group: "core"},
	"distribution": {Name: "distribution", Required: true, DependsOn: []string{"staking", "auth", "bank"}, Group: "core"},
	"slashing":     {Name: "slashing", Required: true, DependsOn: []string{"staking"}, Group: "core"},
	"gov":          {Name: "gov", Required: true, DependsOn: []string{"auth", "bank", "staking", "distribution"}, Group: "core"},
	"genutil":      {Name: "genutil", Required: true, DependsOn: []string{"auth", "bank", "staking"}, Group: "core"},
	"wasm":         {Name: "wasm", Required: true, DependsOn: []string{"auth", "bank", "staking"}, Group: "core"},
	"evidence":     {Name: "evidence", Required: true, DependsOn: []string{"staking", "slashing"}, Group: "core"},

	// IBC group - controlled by --enable-ibc flag
	"ibc":      {Name: "ibc", DependsOn: []string{"evidence"}, Group: "ibc"},
	"transfer": {Name: "transfer", DependsOn: []string{"ibc", "auth", "bank"}, Group: "ibc"},
	"ica":      {Name: "ica", DependsOn: []string{"ibc"}, Group: "ibc"},
	"ibctm":    {Name: "ibctm", DependsOn: []string{"ibc"}, Group: "ibc"},

	// Optional modules - individual flags
	"upgrade":      {Name: "upgrade", Group: "optional"},
	"feegrant":     {Name: "feegrant", DependsOn: []string{"auth"}, Group: "optional"},
	"authz":        {Name: "authz", DependsOn: []string{"auth"}, Group: "optional"},
	"vesting":      {Name: "vesting", DependsOn: []string{"auth", "bank"}, Group: "optional"},
	"protocolpool": {Name: "protocolpool", DependsOn: []string{"distribution"}, Group: "optional"},

	// Deprecated modules - always removed when --disable-deprecated (default true)
	"group":  {Name: "group", Deprecated: true, Group: "deprecated"},
	"nft":    {Name: "nft", Deprecated: true, Group: "deprecated"},
	"crisis": {Name: "crisis", Deprecated: true, Group: "deprecated"},
}

// ResolveModules converts config flags into a map of module -> enabled.
func ResolveModules(cfg config) map[string]bool {
	enabled := make(map[string]bool)

	// Core modules are always enabled
	for name, def := range moduleRegistry {
		if def.Group == "core" {
			enabled[name] = true
		}
	}

	// IBC group
	if cfg.enableIBC {
		for name, def := range moduleRegistry {
			if def.Group == "ibc" {
				enabled[name] = true
			}
		}
	}

	// Optional modules
	if cfg.enableUpgrade {
		enabled["upgrade"] = true
	}
	if cfg.enableFeegrant {
		enabled["feegrant"] = true
	}
	if cfg.enableAuthz {
		enabled["authz"] = true
	}
	if cfg.enableVesting {
		enabled["vesting"] = true
	}
	if cfg.enableProtocolPool {
		enabled["protocolpool"] = true
	}

	// Deprecated modules - only enabled if --enable-deprecated (default false)
	if cfg.enableDeprecated {
		for name, def := range moduleRegistry {
			if def.Group == "deprecated" {
				enabled[name] = true
			}
		}
	}

	return enabled
}

// ValidateModules checks module dependencies and conflicts.
func ValidateModules(enabled map[string]bool) error {
	// Build list of enabled modules
	var enabledList []string
	for name, on := range enabled {
		if on {
			enabledList = append(enabledList, name)
		}
	}

	// Check each enabled module
	for _, name := range enabledList {
		def, ok := moduleRegistry[name]
		if !ok {
			continue // unknown module, skip
		}

		// Check dependencies
		for _, dep := range def.DependsOn {
			if !enabled[dep] {
				return fmt.Errorf("module %q requires %q which is disabled", name, dep)
			}
		}
	}

	// Check for disabled modules that are required by enabled ones
	for _, name := range enabledList {
		def, ok := moduleRegistry[name]
		if !ok {
			continue
		}
		for _, dep := range def.DependsOn {
			if !enabled[dep] {
				return fmt.Errorf("module %q depends on %q but it is not enabled", name, dep)
			}
		}
	}

	// Check for cycles
	if err := detectCycles(enabled); err != nil {
		return err
	}

	return nil
}

// detectCycles uses DFS to detect circular dependencies among enabled modules.
func detectCycles(enabled map[string]bool) error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(name string) error
	dfs = func(name string) error {
		if !enabled[name] {
			return nil
		}
		if recStack[name] {
			return fmt.Errorf("circular dependency detected involving %q", name)
		}
		if visited[name] {
			return nil
		}

		visited[name] = true
		recStack[name] = true

		def := moduleRegistry[name]
		for _, dep := range def.DependsOn {
			if err := dfs(dep); err != nil {
				return err
			}
		}

		recStack[name] = false
		return nil
	}

	for name := range enabled {
		if enabled[name] && !visited[name] {
			if err := dfs(name); err != nil {
				return err
			}
		}
	}

	return nil
}

// EnabledModulesList returns a sorted list of enabled module names.
func EnabledModulesList(enabled map[string]bool) []string {
	var list []string
	for name, on := range enabled {
		if on {
			list = append(list, name)
		}
	}
	// Sort for deterministic output
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i] > list[j] {
				list[i], list[j] = list[j], list[i]
			}
		}
	}
	return list
}

// ModuleSummary returns a human-readable summary of module states.
func ModuleSummary(cfg config) string {
	enabled := ResolveModules(cfg)
	var parts []string
	for name, def := range moduleRegistry {
		on := enabled[name]
		status := "off"
		if on {
			status = "on"
		}
		group := def.Group
		if group == "" {
			group = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s(%s)=%s", name, group, status))
	}
	return strings.Join(parts, ", ")
}
