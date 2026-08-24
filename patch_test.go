package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenApp is a synthetic app.go exercising every deprecated module and
// SetOrderEndBlockers pattern the patch must handle.
const goldenApp = `package app

import (
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	crisis "github.com/cosmos/cosmos-sdk/x/crisis"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	grouptypes "github.com/cosmos/cosmos-sdk/x/group/types"
	group "github.com/cosmos/cosmos-sdk/x/group"
	nftkeeper "github.com/cosmos/cosmos-sdk/x/nft/keeper"
	nfttypes "github.com/cosmos/cosmos-sdk/x/nft/types"
	nft "github.com/cosmos/cosmos-sdk/x/nft"
	protocolpoolkeeper "github.com/cosmos/cosmos-sdk/x/protocolpool/keeper"
	protocolpooltypes "github.com/cosmos/cosmos-sdk/x/protocolpool/types"
	protocolpool "github.com/cosmos/cosmos-sdk/x/protocolpool"
)

type App struct {
	ProtocolPoolKeeper protocolpoolkeeper.Keeper
	GroupKeeper        groupkeeper.Keeper
	NFTKeeper          nftkeeper.Keeper
	CrisisKeeper       crisiskeeper.Keeper
	BankKeeper         banktypes.Keeper
}

func NewApp() *App {
	app := &App{}
	app.ProtocolPoolKeeper = protocolpoolkeeper.NewKeeper(app.BankKeeper)
	app.GroupKeeper = groupkeeper.NewKeeper(app.BankKeeper)
	app.NFTKeeper = nftkeeper.NewKeeper(app.BankKeeper)
	app.CrisisKeeper = crisiskeeper.NewKeeper(app.BankKeeper)
	return app
}

func SetupModuleManager(app *App) {
	app.ModuleManager = NewManager(
		protocolpool.NewAppModule(app.ProtocolPoolKeeper),
		group.NewAppModule(app.GroupKeeper),
		nft.NewAppModule(app.NFTKeeper),
		crisis.NewAppModule(app.CrisisKeeper),
		distrkeeper.WithExternalCommunityPool(app.ProtocolPoolKeeper),
	)
	app.ModuleManager.SetOrderEndBlockers(
		govtypes.ModuleName,
		banktypes.ModuleName,
		protocolpooltypes.ModuleName,
		grouptypes.ModuleName,
		nfttypes.ModuleName,
		crisistypes.ModuleName,
		wasmtypes.ModuleName,
	)
}

func InitParams() []string {
	return []string{
		authtypes.ModuleName,
		protocolpooltypes.StoreKey,
		grouptypes.StoreKey,
		nfttypes.StoreKey,
		crisistypes.StoreKey,
		banktypes.ModuleName,
	}
}

func MaccPerms() map[string]bool {
	return map[string]bool{
		authtypes.ModuleName:                    true,
		protocolpooltypes.ModuleName:            true,
		protocolpooltypes.ProtocolPoolEscrowAccount: true,
		grouptypes.ModuleName:                    true,
		nfttypes.ModuleName:                     true,
		crisistypes.ModuleName:                  true,
		banktypes.ModuleName:                    true,
	}
}
`

func TestPatchAppFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenApp), 0o644); err != nil {
		t.Fatal(err)
	}
	// Use default config (all modules enabled except deprecated)
	cfg := config{
		enableIBC:          true,
		enableEvidence:     true,
		enableUpgrade:      true,
		enableFeegrant:     true,
		enableAuthz:        true,
		enableVesting:      true,
		enableProtocolPool: true,  // protocolpool is now optional (enabled by default)
		enableDeprecated:   false, // deprecated modules removed by default
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// protocolpool should remain (optional module enabled by default)
	if !strings.Contains(got, "protocolpool") {
		t.Errorf("protocolpool should remain when --enable-protocolpool=true")
	}
	if !strings.Contains(got, "ProtocolPoolKeeper") {
		t.Errorf("ProtocolPoolKeeper should remain when --enable-protocolpool=true")
	}
	if strings.Contains(got, "group") {
		t.Errorf("group should be removed (deprecated)")
	}
	if strings.Contains(got, "nft") {
		t.Errorf("nft should be removed (deprecated)")
	}
	if strings.Contains(got, "crisis") {
		t.Errorf("crisis should be removed (deprecated)")
	}
	if !strings.Contains(got, "banktypes.ModuleName") {
		t.Fatalf("banktypes.ModuleName removed entirely:\n%s", got)
	}
	if !strings.Contains(got, "distrkeeper") {
		t.Errorf("distrkeeper import removed (may still be used elsewhere):\n%s", got)
	}

	// banktypes must be the first arg of SetOrderEndBlockers.
	seg := got[strings.Index(got, "SetOrderEndBlockers("):]
	first := strings.Index(seg, "ModuleName")
	prefix := seg[:first]
	if !strings.Contains(prefix, "banktypes") {
		t.Errorf("banktypes.ModuleName is not first in SetOrderEndBlockers:\n%s", seg)
	}

	// Generated code must still parse and format cleanly.
	if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPatchCapabilities verifies the capability patch both replaces the builtin
// call and validates against the exact requested set.
func TestPatchCapabilities(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	src := "package app\n\nconst _ = wasmkeeper.BuiltInCapabilities()\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	// Custom capability list, not the default set.
	if err := patchCapabilities(path, []string{"a", "b", "cosmwasm_3_0"}); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{`[]string{"a", "b", "cosmwasm_3_0"}`, "cosmwasm_3_0"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q after patch:\n%s", want, got)
		}
	}
	if strings.Contains(got, "BuiltInCapabilities()") {
		t.Errorf("BuiltInCapabilities() still present:\n%s", got)
	}

	// Second call must fail: the builtin marker is gone.
	if err := patchCapabilities(path, []string{"x"}); err == nil {
		t.Errorf("expected error when BuiltInCapabilities() already replaced")
	}

	// A marker file with no builtin must error.
	if err := os.WriteFile(path, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := patchCapabilities(path, []string{"a"}); err == nil {
		t.Errorf("expected error when BuiltInCapabilities() absent")
	}
}

func TestFilterCapabilities(t *testing.T) {
	// Test with IBC enabled - all capabilities preserved
	cfg := config{enableIBC: true, capabilities: []string{"iterator", "staking", "ibc2", "cosmwasm_3_0"}}
	filtered := filterCapabilities(cfg.capabilities, cfg)
	if len(filtered) != 4 || !contains(filtered, "ibc2") {
		t.Errorf("expected all capabilities preserved with IBC enabled, got: %v", filtered)
	}

	// Test with IBC disabled - ibc2 removed
	cfg = config{enableIBC: false, capabilities: []string{"iterator", "staking", "ibc2", "cosmwasm_3_0"}}
	filtered = filterCapabilities(cfg.capabilities, cfg)
	if len(filtered) != 3 || contains(filtered, "ibc2") {
		t.Errorf("expected ibc2 removed with IBC disabled, got: %v", filtered)
	}
	if !contains(filtered, "iterator") || !contains(filtered, "staking") || !contains(filtered, "cosmwasm_3_0") {
		t.Errorf("expected other capabilities preserved, got: %v", filtered)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// TestRemoveV060Upgrade verifies the inherited v0.60 upgrade handler is dropped
// from both the filesystem and the app/upgrades.go wiring.
func TestRemoveV060Upgrade(t *testing.T) {
	dir := t.TempDir()
	v060Dir := filepath.Join(dir, "app", "upgrades", "v060")
	if err := os.MkdirAll(v060Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(v060Dir, "upgrades.go"),
		[]byte("package v060\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	upgradesFile := filepath.Join(dir, "app", "upgrades.go")
	src := `package app

import (
	"github.com/umesh-network/umesh/app/upgrades"
	"github.com/umesh-network/umesh/app/upgrades/noop"
	v060 "github.com/umesh-network/umesh/app/upgrades/v060"
)

var Upgrades = []upgrades.Upgrade{v060.Upgrade}
`
	if err := os.WriteFile(upgradesFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeV060Upgrade(dir); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(v060Dir); !os.IsNotExist(err) {
		t.Errorf("v060 dir still exists")
	}
	out, err := os.ReadFile(upgradesFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if strings.Contains(got, "v060") {
		t.Errorf("v060 still referenced:\n%s", got)
	}
	if !strings.Contains(got, "var Upgrades = []upgrades.Upgrade{}") {
		t.Errorf("Upgrades not reset to empty:\n%s", got)
	}
	if !strings.Contains(got, "noop") {
		t.Errorf("noop fallback removed:\n%s", got)
	}
}

// TestPatchStringLiterals verifies the temp dir prefixes and BaseApp name are
// rewritten from the wasmd/simapp literals.
func TestPatchStringLiterals(t *testing.T) {
	cfg := config{binaryName: "umeshd", bech32Prefix: "umesh"}
	dir := t.TempDir()

	cmdDir := filepath.Join(dir, "cmd", cfg.binaryName)
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	commands := `package main

var tempDir = func() string {
	dir, err := os.MkdirTemp("", "wasmd")
	if err != nil {
		panic(err)
	}
	return dir
}
`
	if err := os.WriteFile(filepath.Join(cmdDir, "commands.go"), []byte(commands), 0o644); err != nil {
		t.Fatal(err)
	}

	appDir := filepath.Join(dir, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	appGo := `package app

const appName = "WasmApp"
`
	if err := os.WriteFile(filepath.Join(appDir, "app.go"), []byte(appGo), 0o644); err != nil {
		t.Fatal(err)
	}
	helpers := `package app

func f() {
	dir, err := os.MkdirTemp("", "simapp")
	_, _ = dir, err
}
`
	if err := os.WriteFile(filepath.Join(appDir, "test_helpers.go"), []byte(helpers), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := patchStringLiterals(dir, cfg); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		filepath.Join(cmdDir, "commands.go"):     `os.MkdirTemp("", "umeshd")`,
		filepath.Join(appDir, "app.go"):          `const appName = "UmeshApp"`,
		filepath.Join(appDir, "test_helpers.go"): `os.MkdirTemp("", "umesh")`,
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), want) {
			t.Errorf("%s missing %q:\n%s", path, want, b)
		}
	}
}

// goldenAppWithOptional extends goldenApp with IBC, evidence, upgrade, feegrant, authz, vesting modules.
const goldenAppWithOptional = `package app

import (
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	authzmodule "github.com/cosmos/cosmos-sdk/x/authz/module"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	crisiskeeper "github.com/cosmos/cosmos-sdk/x/crisis/keeper"
	crisistypes "github.com/cosmos/cosmos-sdk/x/crisis/types"
	crisis "github.com/cosmos/cosmos-sdk/x/crisis"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	evidencekeeper "github.com/cosmos/cosmos-sdk/x/evidence/keeper"
	evidencetypes "github.com/cosmos/cosmos-sdk/x/evidence/types"
	evidence "github.com/cosmos/cosmos-sdk/x/evidence"
	feegrantkeeper "github.com/cosmos/cosmos-sdk/x/feegrant/keeper"
	feegrantmodule "github.com/cosmos/cosmos-sdk/x/feegrant/module"
	feegrant "github.com/cosmos/cosmos-sdk/x/feegrant"
	groupkeeper "github.com/cosmos/cosmos-sdk/x/group/keeper"
	grouptypes "github.com/cosmos/cosmos-sdk/x/group/types"
	group "github.com/cosmos/cosmos-sdk/x/group"
	ibcexported "github.com/cosmos/ibc-go/v11/modules/core/exported"
	ibckeeper "github.com/cosmos/ibc-go/v11/modules/core/keeper"
	ibctransferkeeper "github.com/cosmos/ibc-go/v11/modules/apps/transfer/keeper"
	ibctransfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	transfer "github.com/cosmos/ibc-go/v11/modules/apps/transfer"
	icacontrollerkeeper "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/controller/keeper"
	icacontrollertypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/controller/types"
	icacontroller "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/controller"
	icahostkeeper "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/host/keeper"
	icahosttypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/host/types"
	icahost "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/host"
	icatypes "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts/types"
	ica "github.com/cosmos/ibc-go/v11/modules/apps/27-interchain-accounts"
	ibctm "github.com/cosmos/ibc-go/v11/modules/light-clients/07-tendermint"
	nftkeeper "github.com/cosmos/cosmos-sdk/x/nft/keeper"
	nfttypes "github.com/cosmos/cosmos-sdk/x/nft/types"
	nft "github.com/cosmos/cosmos-sdk/x/nft"
	protocolpoolkeeper "github.com/cosmos/cosmos-sdk/x/protocolpool/keeper"
	protocolpooltypes "github.com/cosmos/cosmos-sdk/x/protocolpool/types"
	protocolpool "github.com/cosmos/cosmos-sdk/x/protocolpool"
	upgradekeeper "github.com/cosmos/cosmos-sdk/x/upgrade/keeper"
	upgradetypes "github.com/cosmos/cosmos-sdk/x/upgrade/types"
	upgrade "github.com/cosmos/cosmos-sdk/x/upgrade"
	vesting "github.com/cosmos/cosmos-sdk/x/auth/vesting"
)

type App struct {
	ProtocolPoolKeeper    protocolpoolkeeper.Keeper
	GroupKeeper           groupkeeper.Keeper
	NFTKeeper             nftkeeper.Keeper
	CrisisKeeper          crisiskeeper.Keeper
	BankKeeper            banktypes.Keeper
	EvidenceKeeper        evidencekeeper.Keeper
	UpgradeKeeper         *upgradekeeper.Keeper
	FeeGrantKeeper        feegrantkeeper.Keeper
	AuthzKeeper           authzkeeper.Keeper
	IBCKeeper             *ibckeeper.Keeper
	ICAControllerKeeper   *icacontrollerkeeper.Keeper
	ICAHostKeeper         *icahostkeeper.Keeper
	TransferKeeper        *ibctransferkeeper.Keeper
}

func NewApp() *App {
	app := &App{}
	app.ProtocolPoolKeeper = protocolpoolkeeper.NewKeeper(app.BankKeeper)
	app.GroupKeeper = groupkeeper.NewKeeper(app.BankKeeper)
	app.NFTKeeper = nftkeeper.NewKeeper(app.BankKeeper)
	app.CrisisKeeper = crisiskeeper.NewKeeper(app.BankKeeper)
	app.EvidenceKeeper = evidencekeeper.NewKeeper(app.BankKeeper)
	app.UpgradeKeeper = upgradekeeper.NewKeeper(app.BankKeeper)
	app.FeeGrantKeeper = feegrantkeeper.NewKeeper(app.BankKeeper)
	app.AuthzKeeper = authzkeeper.NewKeeper(app.BankKeeper)
	app.IBCKeeper = ibckeeper.NewKeeper(app.BankKeeper)
	app.ICAControllerKeeper = icacontrollerkeeper.NewKeeper(app.BankKeeper)
	app.ICAHostKeeper = icahostkeeper.NewKeeper(app.BankKeeper)
	app.TransferKeeper = ibctransferkeeper.NewKeeper(app.BankKeeper)
	return app
}

func SetupModuleManager(app *App) {
	app.ModuleManager = NewManager(
		protocolpool.NewAppModule(app.ProtocolPoolKeeper),
		group.NewAppModule(app.GroupKeeper),
		nft.NewAppModule(app.NFTKeeper),
		crisis.NewAppModule(app.CrisisKeeper),
		evidence.NewAppModule(app.EvidenceKeeper),
		upgrade.NewAppModule(app.UpgradeKeeper, nil),
		feegrantmodule.NewAppModule(nil, app.FeeGrantKeeper, nil, nil),
		authzmodule.NewAppModule(nil, app.AuthzKeeper, nil, nil),
		transfer.NewAppModule(app.TransferKeeper),
		ica.NewAppModule(app.ICAControllerKeeper, app.ICAHostKeeper),
		ibctm.NewAppModule(nil),
		vesting.NewAppModule(nil, nil),
		distrkeeper.WithExternalCommunityPool(app.ProtocolPoolKeeper),
	)
	app.ModuleManager.SetOrderBeginBlockers(
		minttypes.ModuleName,
		distrtypes.ModuleName,
		slashingtypes.ModuleName,
		evidencetypes.ModuleName,
		stakingtypes.ModuleName,
		genutiltypes.ModuleName,
		authzkeeper.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		icatypes.ModuleName,
		wasmtypes.ModuleName,
	)
	app.ModuleManager.SetOrderEndBlockers(
		govtypes.ModuleName,
		banktypes.ModuleName,
		protocolpooltypes.ModuleName,
		grouptypes.ModuleName,
		nfttypes.ModuleName,
		crisistypes.ModuleName,
		ibctransfertypes.ModuleName,
		ibcexported.ModuleName,
		icatypes.ModuleName,
		wasmtypes.ModuleName,
	)
}

func InitParams() []string {
	return []string{
		authtypes.ModuleName,
		protocolpooltypes.StoreKey,
		grouptypes.StoreKey,
		nfttypes.StoreKey,
		crisistypes.StoreKey,
		evidencetypes.StoreKey,
		upgradetypes.StoreKey,
		feegrant.StoreKey,
		authzkeeper.StoreKey,
		ibcexported.StoreKey,
		ibctransfertypes.StoreKey,
		icahosttypes.StoreKey,
		icacontrollertypes.StoreKey,
		banktypes.ModuleName,
	}
}

func MaccPerms() map[string]bool {
	return map[string]bool{
		authtypes.ModuleName:                         true,
		protocolpooltypes.ModuleName:                 true,
		protocolpooltypes.ProtocolPoolEscrowAccount:  true,
		grouptypes.ModuleName:                        true,
		nfttypes.ModuleName:                          true,
		crisistypes.ModuleName:                       true,
		ibctransfertypes.ModuleName:                  true,
		icatypes.ModuleName:                          true,
		banktypes.ModuleName:                         true,
	}
}
`

func TestPatchAppFile_DisableIBC(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        false,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// IBC modules should be removed
	for _, mod := range []string{"ibc", "transfer", "ica", "ibctm", "ibccallbacks"} {
		if strings.Contains(got, mod) {
			t.Errorf("IBC module %q still present after patch with --enable-ibc=false:\n%s", mod, got)
		}
	}
	// Core modules should remain
	if !strings.Contains(got, "banktypes.ModuleName") {
		t.Errorf("banktypes.ModuleName removed:\n%s", got)
	}
	// evidence should remain
	if !strings.Contains(got, "evidence") {
		t.Errorf("evidence should remain when --enable-evidence=true:\n%s", got)
	}
}

func TestPatchAppFile_EvidenceAlwaysOn(t *testing.T) {
	// evidence is now a core module, always retained even when --enable-evidence=false
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   false, // flag kept for compat but ignored
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// evidence should remain (core module)
	if !strings.Contains(got, "evidence") {
		t.Errorf("evidence should remain when --enable-evidence=false (now core module):\n%s", got)
	}
	// IBC should also remain (enableIBC=true)
	if !strings.Contains(got, "ibc") {
		t.Errorf("ibc should remain when --enable-ibc=true:\n%s", got)
	}
}

func TestPatchAppFile_DisableAuthz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      false,
		enableVesting:    true,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if strings.Contains(got, "authz") {
		t.Errorf("authz still present after patch with --enable-authz=false:\n%s", got)
	}
}

func TestPatchAppFile_DisableFeegrant(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   false,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if strings.Contains(got, "feegrant") {
		t.Errorf("feegrant still present after patch with --enable-feegrant=false:\n%s", got)
	}
}

func TestPatchAppFile_DisableVesting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    false,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if strings.Contains(got, "vesting") {
		t.Errorf("vesting still present after patch with --enable-vesting=false:\n%s", got)
	}
}

func TestPatchAppFile_DisableUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true,
		enableUpgrade:    false,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if strings.Contains(got, "upgrade") {
		t.Errorf("upgrade still present after patch with --enable-upgrade=false:\n%s", got)
	}
}

func TestPatchAppFile_MinimalConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        false,
		enableEvidence:   false,
		enableUpgrade:    false,
		enableFeegrant:   false,
		enableAuthz:      false,
		enableVesting:    false,
		enableDeprecated: false, // deprecated modules should be removed
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// Only core modules should remain (no ibc, no optional, no deprecated)
	// evidence now remains (core module)
	for _, mod := range []string{"ibc", "transfer", "ica", "ibctm", "upgrade", "feegrant", "authz", "vesting",
		"protocolpool", "group", "nft", "crisis"} {
		if strings.Contains(got, mod) {
			t.Errorf("module %q should be removed in minimal config:\n%s", mod, got)
		}
	}
	// evidence should remain (core module)
	if !strings.Contains(got, "evidence") {
		t.Errorf("evidence should remain in minimal config (core module):\n%s", got)
	}
	// Core should remain
	if !strings.Contains(got, "banktypes.ModuleName") {
		t.Errorf("core banktypes.ModuleName should remain:\n%s", got)
	}
	if !strings.Contains(got, "stakingtypes.ModuleName") {
		t.Errorf("core stakingtypes.ModuleName should remain:\n%s", got)
	}
}

func TestPatchAppFile_FullConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true, // keep deprecated
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// All modules should remain (including deprecated)
	for _, mod := range []string{"ibc", "transfer", "ica", "evidence", "upgrade", "feegrant", "authz", "vesting",
		"protocolpool", "group", "nft", "crisis"} {
		if !strings.Contains(got, mod) {
			t.Errorf("module %q should remain in full config:\n%s", mod, got)
		}
	}
}

func TestPatchAppFile_DeprecatedDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:          true,
		enableEvidence:     true,
		enableUpgrade:      true,
		enableFeegrant:     true,
		enableAuthz:        true,
		enableVesting:      true,
		enableProtocolPool: true,  // protocolpool is now optional (not deprecated)
		enableDeprecated:   false, // deprecated: group, nft, crisis
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// Protocolpool is optional — it should remain when --enable-protocolpool=true
	if !strings.Contains(got, "protocolpool") {
		t.Errorf("protocolpool should remain when --enable-protocolpool=true")
	}

	// Deprecated modules should be removed (group, nft, crisis only)
	for _, mod := range []string{"group", "nft", "crisis"} {
		if strings.Contains(got, mod) {
			t.Errorf("deprecated module %q should be removed when --enable-deprecated=false:\n%s", mod, got)
		}
	}
}

func TestPatchAppFile_DisableProtocolPool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.go")
	if err := os.WriteFile(path, []byte(goldenAppWithOptional), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config{
		enableIBC:          true,
		enableEvidence:     true,
		enableUpgrade:      true,
		enableFeegrant:     true,
		enableAuthz:        true,
		enableVesting:      true,
		enableProtocolPool: false, // explicitly disable protocolpool
		enableDeprecated:   false,
	}
	if err := patchAppFile(path, cfg); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	if strings.Contains(got, "protocolpool") {
		t.Errorf("protocolpool should be removed when --enable-protocolpool=false:\n%s", got)
	}
	if strings.Contains(got, "ProtocolPoolKeeper") {
		t.Errorf("ProtocolPoolKeeper should be removed when --enable-protocolpool=false:\n%s", got)
	}
	if strings.Contains(got, "WithExternalCommunityPool") {
		t.Errorf("WithExternalCommunityPool should be removed when --enable-protocolpool=false:\n%s", got)
	}
	if !strings.Contains(got, "distrkeeper") {
		t.Errorf("distrkeeper import should remain (used by distribution core module)")
	}
}
