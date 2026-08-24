package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const sourceModule = "github.com/CosmWasm/wasmd"

// stepRename renames cmd/wasmd -> cmd/<binary>, rewrites the Go module path,
// and patches the well-known literals (AppName, DefaultNodeHome, Bech32Prefix,
// NodeDir) plus the Makefile build targets.
func stepRename(cfg config) error {
	root := cfg.outputDir

	oldCmd := filepath.Join(root, "cmd", "wasmd")
	newCmd := filepath.Join(root, "cmd", cfg.binaryName)
	if _, err := os.Stat(oldCmd); err == nil {
		if err := os.Rename(oldCmd, newCmd); err != nil {
			return err
		}
	}

	// Module path in all .go files and go.mod.
	srcPkg := sourceModule
	dstPkg := cfg.targetModule
	if err := walkGoFiles(root, func(path string) error {
		_, err := replaceInFile(path, srcPkg, dstPkg)
		return err
	}); err != nil {
		return err
	}
	if _, err := replaceInFile(filepath.Join(root, "go.mod"), srcPkg, dstPkg); err != nil {
		return err
	}

	// Well-known literals.
	if err := patchRootCmd(root, cfg); err != nil {
		return err
	}
	if err := patchAppLiterals(root, cfg); err != nil {
		return err
	}
	if err := patchStringLiterals(root, cfg); err != nil {
		return err
	}
	return patchMakefile(root, cfg)
}

// patchStringLiterals fixes user-visible string literals not covered by the
// module-path rewrite: the temp dir prefixes and the internal BaseApp name.
func patchStringLiterals(root string, cfg config) error {
	commandsFile := filepath.Join(root, "cmd", cfg.binaryName, "commands.go")
	if _, err := replaceInFile(commandsFile,
		`os.MkdirTemp("", "wasmd")`,
		`os.MkdirTemp("", "`+cfg.binaryName+`")`); err != nil {
		return err
	}
	appFile := filepath.Join(root, "app", "app.go")
	if _, err := replaceInFile(appFile,
		`const appName = "WasmApp"`,
		`const appName = "`+appNameValue(cfg)+`"`); err != nil {
		return err
	}
	testHelpers := filepath.Join(root, "app", "test_helpers.go")
	if _, err := replaceInFile(testHelpers,
		`os.MkdirTemp("", "simapp")`,
		`os.MkdirTemp("", "`+cfg.bech32Prefix+`")`); err != nil {
		return err
	}
	return nil
}

// appNameValue derives the BaseApp name from the binary name:
// wasmd -> WasmApp, umeshd -> UmeshApp, umeshnode -> UmeshNodeApp.
func appNameValue(cfg config) string {
	name := cfg.binaryName
	if strings.HasSuffix(name, "node") {
		base := strings.TrimSuffix(name, "node")
		if base != "" {
			return strings.ToUpper(base[:1]) + base[1:] + "NodeApp"
		}
	}
	name = strings.TrimSuffix(name, "d")
	if name == "" {
		return "App"
	}
	return strings.ToUpper(name[:1]) + name[1:] + "App"
}

// patchRootCmd rewrites AppName and DefaultNodeHome in the binary root.go.
func patchRootCmd(root string, cfg config) error {
	candidates := []string{
		filepath.Join(root, "cmd", cfg.binaryName, "cmd", "root.go"),
		filepath.Join(root, "cmd", cfg.binaryName, "root.go"),
	}
	for _, f := range candidates {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		if _, err := replaceInFile(f, `AppName = "wasmd"`, `AppName = "`+cfg.binaryName+`"`); err != nil {
			return err
		}
		if _, err := replaceInFile(f, `DefaultNodeHome = ".wasmd"`, `DefaultNodeHome = "`+cfg.nodeDir+`"`); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("root.go not found under cmd/%s", cfg.binaryName)
}

// patchAppLiterals rewrites Bech32Prefix and NodeDir in app/app.go.
func patchAppLiterals(root string, cfg config) error {
	f := filepath.Join(root, "app", "app.go")
	b, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	content := string(b)
	changed := false

	if strings.Contains(content, `Bech32Prefix = "wasm"`) {
		content = strings.ReplaceAll(content, `Bech32Prefix = "wasm"`, `Bech32Prefix = "`+cfg.bech32Prefix+`"`)
		changed = true
	}
	if strings.Contains(content, `NodeDir      = ".wasmd"`) {
		content = strings.ReplaceAll(content, `NodeDir      = ".wasmd"`, `NodeDir      = "`+cfg.nodeDir+`"`)
		changed = true
	}
	if changed {
		return os.WriteFile(f, []byte(content), 0o644)
	}
	return nil
}

// patchMakefile rewrites build/wasmd -> build/<binary> and ldflags.
func patchMakefile(root string, cfg config) error {
	f := filepath.Join(root, "Makefile")
	b, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	content := string(b)
	content = strings.ReplaceAll(content, "build/wasmd.exe", "build/"+cfg.binaryName+".exe")
	content = strings.ReplaceAll(content, "build/wasmd ", "build/"+cfg.binaryName+" ")
	content = strings.ReplaceAll(content, "./cmd/wasmd", "./cmd/"+cfg.binaryName)
	content = strings.ReplaceAll(content,
		sourceModule+"/app.Bech32Prefix=wasm",
		cfg.targetModule+"/app.Bech32Prefix="+cfg.bech32Prefix)
	content = strings.ReplaceAll(content, "version.Name=wasm", "version.Name="+cfg.versionName)
	content = strings.ReplaceAll(content, "version.AppName=wasmd", "version.AppName="+cfg.binaryName)
	return os.WriteFile(f, []byte(content), 0o644)
}
