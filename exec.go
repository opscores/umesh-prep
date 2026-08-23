package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runCmd runs a command in dir with env, streaming output to stdout/stderr.
func runCmd(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// walkGoFiles visits every .go file under root.
func walkGoFiles(root string, fn func(path string) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		return fn(path)
	})
}

// replaceInFile performs all non-overlapping replacements and writes the file
// back if changed. Returns whether anything changed.
func replaceInFile(path, old, new string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	content := string(b)
	if !strings.Contains(content, old) {
		return false, nil
	}
	return true, os.WriteFile(path, []byte(strings.ReplaceAll(content, old, new)), 0o644)
}
