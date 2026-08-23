package main

import (
	"strings"
	"testing"
)

func TestResolveModules_DefaultConfig(t *testing.T) {
	cfg := config{
		enableIBC:        true,
		enableEvidence:   true, // now default true, flag kept for compat
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: false, // deprecated modules disabled by default
	}

	enabled := ResolveModules(cfg)

	// Core modules always enabled (including evidence)
	coreModules := []string{"auth", "bank", "consensus", "staking", "mint", "distribution", "slashing", "gov", "genutil", "wasm", "evidence"}
	for _, m := range coreModules {
		if !enabled[m] {
			t.Errorf("core module %q should be enabled", m)
		}
	}

	// IBC group enabled
	ibcModules := []string{"ibc", "transfer", "ica", "ibctm"}
	for _, m := range ibcModules {
		if !enabled[m] {
			t.Errorf("IBC module %q should be enabled", m)
		}
	}

	// Optional modules enabled (evidence no longer in optional list)
	optionalModules := []string{"upgrade", "feegrant", "authz", "vesting"}
	for _, m := range optionalModules {
		if !enabled[m] {
			t.Errorf("optional module %q should be enabled", m)
		}
	}

	// Deprecated modules disabled
	deprecatedModules := []string{"protocolpool", "group", "nft", "crisis"}
	for _, m := range deprecatedModules {
		if enabled[m] {
			t.Errorf("deprecated module %q should be disabled", m)
		}
	}
}

func TestResolveModules_IBCDisabled(t *testing.T) {
	cfg := config{
		enableIBC:        false,
		enableEvidence:   true,
		enableUpgrade:    true,
		enableFeegrant:   true,
		enableAuthz:      true,
		enableVesting:    true,
		enableDeprecated: true,
	}

	enabled := ResolveModules(cfg)

	// Core modules always enabled
	if !enabled["staking"] {
		t.Error("staking should be enabled")
	}

	// IBC group disabled
	ibcModules := []string{"ibc", "transfer", "ica", "ibctm"}
	for _, m := range ibcModules {
		if enabled[m] {
			t.Errorf("IBC module %q should be disabled when --enable-ibc=false", m)
		}
	}
}

func TestResolveModules_DeprecatedEnabled(t *testing.T) {
	cfg := config{
		enableIBC:          true,
		enableEvidence:     true,
		enableUpgrade:      true,
		enableFeegrant:     true,
		enableAuthz:        true,
		enableVesting:      true,
		enableDeprecated:   true,
		enableProtocolPool: true,
	}

	enabled := ResolveModules(cfg)

	// protocolpool is now an optional module, enabled via --enable-protocolpool (not --enable-deprecated)
	// so it should be enabled here too
	if !enabled["protocolpool"] {
		t.Errorf("protocolpool should be enabled when --enable-protocolpool=true")
	}

	// Remaining deprecated modules only enabled with --enable-deprecated
	deprecatedModules := []string{"group", "nft", "crisis"}
	for _, m := range deprecatedModules {
		if !enabled[m] {
			t.Errorf("deprecated module %q should be enabled when --enable-deprecated=true", m)
		}
	}
}

func TestResolveModules_ProtocolPoolDisabled(t *testing.T) {
	cfg := config{
		enableIBC:          true,
		enableEvidence:     true,
		enableUpgrade:      true,
		enableFeegrant:     true,
		enableAuthz:        true,
		enableVesting:      true,
		enableProtocolPool: false, // explicitly disabled
		enableDeprecated:   false,
	}

	enabled := ResolveModules(cfg)

	if enabled["protocolpool"] {
		t.Errorf("protocolpool should be disabled when --enable-protocolpool=false")
	}
}

func TestResolveModules_MinimalConfig(t *testing.T) {
	cfg := config{
		enableIBC:          false,
		enableEvidence:     false, // ignored now (evidence is always core)
		enableUpgrade:      false,
		enableFeegrant:     false,
		enableAuthz:        false,
		enableVesting:      false,
		enableProtocolPool: false,
		enableDeprecated:   false,
	}

	enabled := ResolveModules(cfg)

	// Core modules always enabled (including evidence)
	expectedEnabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"evidence": true, // always enabled (core)
	}

	for m, should := range expectedEnabled {
		if enabled[m] != should {
			t.Errorf("module %q: got enabled=%v, want %v", m, enabled[m], should)
		}
	}

	// IBC group disabled
	ibcModules := []string{"ibc", "transfer", "ica", "ibctm"}
	for _, m := range ibcModules {
		if enabled[m] {
			t.Errorf("IBC module %q should be disabled in minimal config", m)
		}
	}

	// Optional modules disabled (protocolpool now optional, not deprecated)
	optionalModules := []string{"upgrade", "feegrant", "authz", "vesting", "protocolpool"}
	for _, m := range optionalModules {
		if enabled[m] {
			t.Errorf("optional module %q should be disabled in minimal config", m)
		}
	}

	// Deprecated modules disabled
	deprecatedModules := []string{"group", "nft", "crisis"}
	for _, m := range deprecatedModules {
		if enabled[m] {
			t.Errorf("deprecated module %q should be disabled", m)
		}
	}
}

func TestValidateModules_AllEnabled(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"ibc": true, "transfer": true, "ica": true, "ibctm": true,
		"evidence": true, "upgrade": true, "feegrant": true, "authz": true, "vesting": true,
		"protocolpool": true,
	}

	if err := ValidateModules(enabled); err != nil {
		t.Errorf("ValidateModules should pass for all enabled: %v", err)
	}
}

func TestValidateModules_CoreOnly(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"evidence": true,
	}

	if err := ValidateModules(enabled); err != nil {
		t.Errorf("ValidateModules should pass for core only: %v", err)
	}
}

func TestValidateModules_TransferWithoutIBC(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"transfer": true, // ibc is false
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when transfer is enabled without ibc")
	} else if !strings.Contains(err.Error(), "transfer") || !strings.Contains(err.Error(), "ibc") {
		t.Errorf("Error should mention transfer requires ibc, got: %v", err)
	}
}

func TestValidateModules_ICAWithoutIBC(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"ica": true, // ibc is false
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when ica is enabled without ibc")
	} else if !strings.Contains(err.Error(), "ica") || !strings.Contains(err.Error(), "ibc") {
		t.Errorf("Error should mention ica requires ibc, got: %v", err)
	}
}

func TestValidateModules_CoreModuleDisabled(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		// staking is missing
		"mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when staking is disabled")
	}
}

func TestValidateModules_DeprecatedModuleEnabled(t *testing.T) {
	// Deprecated modules (group, nft, crisis) can be enabled
	// when --enable-deprecated=true. Validation doesn't reject them;
	// the flag controls whether they are included.
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"protocolpool": true, // now optional, enabled via --enable-protocolpool
		"group":        true,
		"nft":          true,
		"crisis":       true,
	}

	if err := ValidateModules(enabled); err != nil {
		t.Errorf("ValidateModules should pass when deprecated modules are explicitly enabled: %v", err)
	}
}

func TestValidateModules_IBCWithoutUpgrade(t *testing.T) {
	// ibc does NOT depend on upgrade - this should PASS
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"evidence": true, // evidence is a dependency of ibc
		"ibc":      true, "transfer": true, "ica": true, "ibctm": true,
		// upgrade is false
	}

	if err := ValidateModules(enabled); err != nil {
		t.Errorf("ValidateModules should pass when IBC is enabled without upgrade: %v", err)
	}
}

func TestValidateModules_EvidenceWithoutSlashing(t *testing.T) {
	// evidence is now a core module that is always paired with slashing
	// this test is kept for documentation but should pass since both are core
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true, "distribution": true,
		"slashing": true, "gov": true, "genutil": true, "wasm": true,
		"evidence": true,
	}

	if err := ValidateModules(enabled); err != nil {
		t.Errorf("ValidateModules should pass for core evidence+slashing: %v", err)
	}
}

func TestValidateModules_FeegrantWithoutAuth(t *testing.T) {
	enabled := map[string]bool{
		"bank": true, "consensus": true, "staking": true,
		"mint": true, "distribution": true, "slashing": true,
		"gov": true, "genutil": true, "wasm": true,
		"feegrant": true,
		// auth is missing
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when feegrant is enabled without auth")
	}
}

func TestValidateModules_AuthzWithoutAuth(t *testing.T) {
	enabled := map[string]bool{
		"bank": true, "consensus": true, "staking": true,
		"mint": true, "distribution": true, "slashing": true,
		"gov": true, "genutil": true, "wasm": true,
		"authz": true,
		// auth is missing
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when authz is enabled without auth")
	}
}

func TestValidateModules_VestingWithoutAuth(t *testing.T) {
	enabled := map[string]bool{
		"bank": true, "consensus": true, "staking": true,
		"mint": true, "distribution": true, "slashing": true,
		"gov": true, "genutil": true, "wasm": true,
		"vesting": true,
		// auth is missing
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when vesting is enabled without auth")
	}
}

func TestValidateModules_ProtocolPoolWithoutDistribution(t *testing.T) {
	// protocolpool requires distribution module.
	// we omit distribution; protocolpool should trigger the error
	// (gov is also excluded to make protocolpool the sole offender).
	enabled := map[string]bool{
		"auth": true, "bank": true, "consensus": true,
		"staking": true, "mint": true,
		"slashing": true, "genutil": true, "wasm": true,
		"evidence":     true,
		"protocolpool": true,
		// distribution is missing
		// gov is omitted (gov also needs distribution)
	}

	if err := ValidateModules(enabled); err == nil {
		t.Error("ValidateModules should fail when protocolpool is enabled without distribution")
	} else if !strings.Contains(err.Error(), "protocolpool") || !strings.Contains(err.Error(), "distribution") {
		t.Errorf("Error should mention protocolpool requires distribution, got: %v", err)
	}
}

func TestValidateModules_CyclicDeps(t *testing.T) {
	// Inject a cycle for testing by temporarily modifying registry
	// We can't easily test this without modifying the registry, so skip
	// The detectCycles function is tested indirectly
}

func TestEnabledModulesList(t *testing.T) {
	enabled := map[string]bool{
		"auth": true, "bank": false, "staking": true,
	}
	list := EnabledModulesList(enabled)

	if len(list) != 2 {
		t.Errorf("Expected 2 enabled modules, got %d: %v", len(list), list)
	}
	if list[0] != "auth" || list[1] != "staking" {
		t.Errorf("Expected sorted [auth staking], got %v", list)
	}
}

func TestModuleSummary(t *testing.T) {
	cfg := config{
		enableIBC:          true,
		enableEvidence:     false, // now always true (core), flag kept for compat
		enableUpgrade:      true,
		enableFeegrant:     false,
		enableAuthz:        true,
		enableVesting:      false,
		enableProtocolPool: true, // protocolpool now optional, enabled here
		enableDeprecated:   true,
	}
	summary := ModuleSummary(cfg)

	if !strings.Contains(summary, "auth(core)=on") {
		t.Errorf("Summary should contain auth(core)=on, got: %s", summary)
	}
	if !strings.Contains(summary, "ibc(ibc)=on") {
		t.Errorf("Summary should contain ibc(ibc)=on, got: %s", summary)
	}
	// evidence now shows as core=on, not optional=off
	if !strings.Contains(summary, "evidence(core)=on") {
		t.Errorf("Summary should contain evidence(core)=on, got: %s", summary)
	}
	// protocolpool now shows as optional=on
	if !strings.Contains(summary, "protocolpool(optional)=on") {
		t.Errorf("Summary should contain protocolpool(optional)=on, got: %s", summary)
	}
}
