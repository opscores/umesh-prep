package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// stepGomod adds the replace directives and runs go get + go mod tidy.
// In-container mode runs Go natively; otherwise it shells out to a pinned
// golang image to avoid requiring a host toolchain.
func stepGomod(cfg config) error {
	root := cfg.outputDir
	if err := addReplaces(root, cfg.sdkVersion, cfg.cometbftVersion); err != nil {
		return err
	}
	if cfg.skipTidy {
		return nil
	}
	if cfg.inContainer || goAvailable() {
		return tidyNative(root, cfg)
	}
	return tidyDocker(root, cfg)
}

// addReplaces appends the cometbft/cosmos-sdk replace block if missing.
func addReplaces(root, sdkVersion, cometbftVersion string) error {
	f := filepath.Join(root, "go.mod")
	b, err := os.ReadFile(f)
	if err != nil {
		return err
	}
	content := string(b)
	if strings.Contains(content, "github.com/cosmos/cosmos-sdk/store/v2 =>") {
		return nil // already replaced
	}
	block := fmt.Sprintf(`
replace (
github.com/cometbft/cometbft => github.com/cometbft/cometbft %s
github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk %s
)
`, cometbftVersion, sdkVersion)
	return os.WriteFile(f, []byte(content+"\n"+block), 0o644)
}

// goAvailable reports whether a usable go binary is on PATH.
func goAvailable() bool {
	p, err := exec.LookPath("go")
	return err == nil && p != ""
}

// tidyNative runs the module resolution steps with the local Go toolchain.
func tidyNative(root string, cfg config) error {
	env := []string{"GOPROXY=" + cfg.goproxy}
	cmds := [][]string{
		{"go", "mod", "edit", "-replace", "github.com/cometbft/cometbft=github.com/cometbft/cometbft@" + cfg.cometbftVersion},
		{"go", "mod", "edit", "-replace", "github.com/cosmos/cosmos-sdk=github.com/cosmos/cosmos-sdk@" + cfg.sdkVersion},
		{"go", "get", "github.com/cosmos/cosmos-sdk@" + cfg.sdkVersion},
		{"go", "get", "github.com/cometbft/cometbft@" + cfg.cometbftVersion},
		{"go", "mod", "tidy"},
	}
	for _, c := range cmds {
		if err := runCmd(root, env, c[0], c[1:]...); err != nil {
			return err
		}
	}
	return os.Chmod(filepath.Join(root, "go.mod"), 0o644)
}

// tidyDocker runs the same steps inside a pinned golang image, preserving the
// host UID/GID for the written files.
func tidyDocker(root string, cfg config) error {
	env := []string{"GOPROXY=" + cfg.goproxy}
	script := strings.Join([]string{
		"apk add --no-cache git >/dev/null 2>&1",
		"go mod edit -replace github.com/cometbft/cometbft=github.com/cometbft/cometbft@" + cfg.cometbftVersion,
		"go mod edit -replace github.com/cosmos/cosmos-sdk=github.com/cosmos/cosmos-sdk@" + cfg.sdkVersion,
		"go get github.com/cosmos/cosmos-sdk@" + cfg.sdkVersion,
		"go get github.com/cometbft/cometbft@" + cfg.cometbftVersion,
		"go mod tidy",
	}, " && ")
	uid, gid := os.Getuid(), os.Getgid()
	args := []string{"run", "--rm", "--network=host",
		"-u", fmt.Sprintf("%d:%d", uid, gid),
		"-v", root + ":/app", "-w", "/app",
		"-e", env[0],
		"docker.io/library/golang:1.25-alpine",
		"sh", "-c", script,
	}
	if err := runCmd(root, nil, "docker", args...); err != nil {
		return err
	}
	return os.Chmod(filepath.Join(root, "go.mod"), 0o644)
}
