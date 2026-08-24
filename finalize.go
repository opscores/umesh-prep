package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// stepFinalize cleans the tree, writes the baseline marker, runs a build gate,
// and commits the result into a fresh git repo inside OUTPUT_DIR.
func stepFinalize(cfg config) error {
	root := cfg.outputDir

	for _, dir := range []string{"contrib", "docs", "testing"} {
		_ = os.RemoveAll(filepath.Join(root, dir))
	}
	if !cfg.keepTests {
		if err := walkGoFiles(root, func(path string) error {
			if strings.HasSuffix(path, "_test.go") {
				return os.Remove(path)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	if err := os.WriteFile(filepath.Join(root, ".wasmd-baseline"), []byte(cfg.wasmdVersion+"\n"), 0o644); err != nil {
		return err
	}

	if !cfg.skipBuild {
		fmt.Println("[umeshprep]     running go build gate...")
		// Production build is CGO_ENABLED=1 (required by CosmWasm wasmvm).
		// wasmd's own !cgo build-tag path does not compile, so we match the
		// real build configuration rather than the no-cgo one.
		if err := runCmd(root, []string{"CGO_ENABLED=1"}, "go", "build", "./..."); err != nil {
			return fmt.Errorf("go build gate failed: %w", err)
		}
	}

	// Re-create a clean git history.
	if err := runCmd(root, nil, "git", "init", "-b", "main"); err != nil {
		return err
	}
	if err := runCmd(root, nil, "git", "config", "user.email", "umesh@local"); err != nil {
		return err
	}
	if err := runCmd(root, nil, "git", "config", "user.name", "Umesh Bot"); err != nil {
		return err
	}
	if err := runCmd(root, nil, "git", "add", "."); err != nil {
		return err
	}
	if err := runCmd(root, nil, "git", "commit", "-m",
		fmt.Sprintf("feat: initialize Umesh from wasmd %s", cfg.wasmdVersion)); err != nil {
		return err
	}
	return nil
}

// stepClone performs a shallow clone of wasmd at the pinned version into
// OUTPUT_DIR. A non-empty existing directory is wiped first.
func stepClone(cfg config) error {
	root := cfg.outputDir
	if info, err := os.Stat(root); err == nil && info.IsDir() {
		if err := os.RemoveAll(root); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(root), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--branch", cfg.wasmdVersion, "--depth", "1", cfg.wasmdRepo, root)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone wasmd %s: %w", cfg.wasmdVersion, err)
	}
	// Drop the upstream history; the finalized tree gets its own fresh commit.
	if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
		return err
	}
	return nil
}
