// Command umeshprep derives the Umesh source tree from wasmd.
//
// It mirrors scripts/common/prepare-umesh-from-wasmd.sh as a standalone Go
// tool: clone wasmd at a pinned version, rename the module, apply the Cosmos
// SDK v0.54 patches via Go AST rewrites, run go mod tidy, and finalize the
// result in OUTPUT_DIR (default: ./src).
//
// Every option can be set as a flag, an environment variable, or a default
// (priority: flag > env > default).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// config holds the full umeshprep configuration resolved from env and flags.
type config struct {
	wasmdVersion    string
	wasmdRepo       string
	outputDir       string
	targetModule    string
	bech32Prefix    string
	nodeDir         string
	binaryName      string
	versionName     string
	sdkVersion      string
	cometbftVersion string
	capabilities    []string
	keepTests       bool
	skipTidy        bool
	skipBuild       bool
	inContainer     bool
	goproxy         string

	// Module toggling flags
	enableIBC          bool
	enableEvidence     bool
	enableUpgrade      bool
	enableFeegrant     bool
	enableAuthz        bool
	enableVesting      bool
	enableProtocolPool bool
	enableDeprecated   bool

	// Feature flags (opt-in, default false)
	enableBlockSTM bool
	enableIAVLX    bool
	enablePoA      bool
	enableEpochs   bool
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		fatal("config: %v", err)
	}

	steps := []struct {
		name string
		run  func(cfg config) error
	}{
		{"clone wasmd", stepClone},
		{"rename module and binary", stepRename},
		{"patch app for SDK v0.54", stepPatch},
		{"adjust go.mod", stepGomod},
		{"finalize", stepFinalize},
	}
	for _, s := range steps {
		fmt.Printf("[umeshprep] >>> %s...\n", s.name)
		if err := s.run(cfg); err != nil {
			fatal("%s: %v", s.name, err)
		}
		fmt.Printf("[umeshprep]     [OK] %s\n", s.name)
	}
	fmt.Println("[umeshprep] Done.")
}

// defaultCapabilities is the wasm capability set hardcoded in app/app.go in
// place of wasmkeeper.BuiltInCapabilities().
const defaultCapabilities = "iterator,staking,stargate,ibc2,cosmwasm_1_1,cosmwasm_1_2,cosmwasm_1_3,cosmwasm_1_4,cosmwasm_2_0,cosmwasm_2_1,cosmwasm_2_2,cosmwasm_3_0"

// parseConfig resolves the configuration from CLI flags, then env vars, then
// defaults. The stdlib flag package is used with env-derived defaults so that
// flag > env > default precedence comes for free.
func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("umeshprep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		wasmdVersion    = fs.String("wasmd-version", getEnv("WASMD_VERSION", "v0.70.3"), "wasmd version tag to clone")
		wasmdRepo       = fs.String("wasmd-repo", getEnv("WASMD_REPO", "https://github.com/CosmWasm/wasmd.git"), "wasmd repository URL")
		outputDir       = fs.String("output-dir", getEnv("OUTPUT_DIR", "./src"), "output directory for the derived source tree")
		targetModule    = fs.String("target-module", getEnv("TARGET_MODULE", "github.com/umesh-network/umesh"), "Go module path of the Umesh fork")
		bech32Prefix    = fs.String("bech32-prefix", getEnv("BECH32_PREFIX", "umesh"), "Bech32 account prefix")
		nodeDir         = fs.String("node-dir", getEnv("NODE_DIR", ".umeshnode"), "default node home directory name")
		binaryName      = fs.String("binary-name", getEnv("BINARY_NAME", "umeshnode"), "resulting binary name")
		versionName     = fs.String("version-name", getEnv("VERSION_NAME", "umesh"), "application version.Name")
		sdkVersion      = fs.String("sdk-version", getEnv("SDK_VERSION", "v0.54.3"), "cosmos-sdk version for the go.mod replace directive (must stay on v0.54.x)")
		cometbftVersion = fs.String("cometbft-version", getEnv("COMETBFT_VERSION", "v0.39.3"), "cometbft version for the go.mod replace directive")
		capabilities    = fs.String("capabilities", getEnv("CAPABILITIES", defaultCapabilities), "comma-separated wasm capabilities")
		goproxy         = fs.String("goproxy", getEnv("GOPROXY", "https://proxy.golang.org,direct"), "GOPROXY value for go commands")
		keepTests       = fs.Bool("keep-tests", getEnv("KEEP_TESTS", "") == "true", "keep *_test.go files (default: remove them)")
		skipTidy        = fs.Bool("skip-tidy", getEnv("SKIP_TIDY", "") == "1", "skip go mod tidy / go get")
		skipBuild       = fs.Bool("skip-build", getEnv("SKIP_BUILD", "") == "1", "skip the CGO go build gate")
		inContainer     = fs.Bool("in-container", getEnv("UMESHPREP_IN_CONTAINER", "") == "1", "assume Go toolchain is available locally (container)")

		// Module toggling flags
		enableIBC          = fs.Bool("enable-ibc", getEnv("ENABLE_IBC", "true") == "true", "enable IBC stack (ibc, transfer, ica, ibctm, ibccallbacks)")
		enableEvidence     = fs.Bool("enable-evidence", getEnv("ENABLE_EVIDENCE", "true") == "true", "enable evidence module (core; flag is a no-op kept for backward compatibility)")
		enableUpgrade      = fs.Bool("enable-upgrade", getEnv("ENABLE_UPGRADE", "true") == "true", "enable upgrade module")
		enableFeegrant     = fs.Bool("enable-feegrant", getEnv("ENABLE_FEAGRANT", "true") == "true", "enable feegrant module (fee delegation)")
		enableAuthz        = fs.Bool("enable-authz", getEnv("ENABLE_AUTHZ", "true") == "true", "enable authz module (message authorization)")
		enableVesting      = fs.Bool("enable-vesting", getEnv("ENABLE_VESTING", "true") == "true", "enable vesting module")
		enableProtocolPool = fs.Bool("enable-protocolpool", getEnv("ENABLE_PROTOCOLPOOL", "") == "true", "enable protocolpool module (community pool redistribution)")
		enableDeprecated   = fs.Bool("enable-deprecated", getEnv("ENABLE_DEPRECATED", "") == "true", "enable deprecated modules (group, nft, crisis)")

		// Feature flags (opt-in, default false)
		enableBlockSTM = fs.Bool("enable-blockstm", getEnv("ENABLE_BLOCKSTM", "") == "true", "enable BlockSTM parallel execution")
		enableIAVLX    = fs.Bool("enable-iavlx", getEnv("ENABLE_IAVLX", "") == "true", "enable IAVLx store optimization")
		enablePoA      = fs.Bool("enable-poa", getEnv("ENABLE_POA", "") == "true", "enable Proof-of-Authority consensus (replaces staking)")
		enableEpochs   = fs.Bool("enable-epochs", getEnv("ENABLE_EPOCHS", "") == "true", "enable x/epochs module")
	)

	fs.Usage = func() {
		_, _ = fmt.Fprintln(fs.Output(), "umeshprep derives the Umesh source tree from wasmd.")
		_, _ = fmt.Fprintln(fs.Output())
		_, _ = fmt.Fprintln(fs.Output(), "Usage: umeshprep [flags]")
		_, _ = fmt.Fprintln(fs.Output(), "Flags (priority: flag > env > default):")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	if fs.NArg() > 0 {
		return config{}, fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}

	return config{
		wasmdVersion:    *wasmdVersion,
		wasmdRepo:       *wasmdRepo,
		outputDir:       *outputDir,
		targetModule:    *targetModule,
		bech32Prefix:    *bech32Prefix,
		nodeDir:         *nodeDir,
		binaryName:      *binaryName,
		versionName:     *versionName,
		sdkVersion:      *sdkVersion,
		cometbftVersion: *cometbftVersion,
		capabilities:    splitCSV(*capabilities),
		keepTests:       *keepTests,
		skipTidy:        *skipTidy,
		skipBuild:       *skipBuild,
		inContainer:     *inContainer,
		goproxy:         *goproxy,

		enableIBC:          *enableIBC,
		enableEvidence:     *enableEvidence,
		enableUpgrade:      *enableUpgrade,
		enableFeegrant:     *enableFeegrant,
		enableAuthz:        *enableAuthz,
		enableVesting:      *enableVesting,
		enableProtocolPool: *enableProtocolPool,
		enableDeprecated:   *enableDeprecated,

		enableBlockSTM: *enableBlockSTM,
		enableIAVLX:    *enableIAVLX,
		enablePoA:      *enablePoA,
		enableEpochs:   *enableEpochs,
	}, nil
}

// splitCSV splits a comma-separated list, trimming whitespace and dropping
// empty entries.
func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[umeshprep] [ERROR] "+format+"\n", a...)
	os.Exit(1)
}
