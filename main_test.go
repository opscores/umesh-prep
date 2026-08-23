package main

import (
	"reflect"
	"testing"
)

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.wasmdVersion != "v0.70.3" {
		t.Errorf("wasmdVersion = %q, want default", cfg.wasmdVersion)
	}
	if cfg.wasmdRepo != "https://github.com/CosmWasm/wasmd.git" {
		t.Errorf("wasmdRepo = %q, want default", cfg.wasmdRepo)
	}
	if cfg.sdkVersion != "v0.54.3" {
		t.Errorf("sdkVersion = %q, want default", cfg.sdkVersion)
	}
	if cfg.cometbftVersion != "v0.39.3" {
		t.Errorf("cometbftVersion = %q, want default", cfg.cometbftVersion)
	}
	if len(cfg.capabilities) == 0 {
		t.Error("capabilities empty by default")
	}
}

func TestParseConfigFlagOverridesEnv(t *testing.T) {
	t.Setenv("OUTPUT_DIR", "/from-env")
	cfg, err := parseConfig([]string{"-output-dir", "/from-flag", "-sdk-version", "v0.54.9"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.outputDir != "/from-flag" {
		t.Errorf("outputDir = %q, want flag to win over env", cfg.outputDir)
	}
	if cfg.sdkVersion != "v0.54.9" {
		t.Errorf("sdkVersion = %q, want flag", cfg.sdkVersion)
	}
}

func TestParseConfigEnvWhenNoFlag(t *testing.T) {
	t.Setenv("OUTPUT_DIR", "/from-env")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.outputDir != "/from-env" {
		t.Errorf("outputDir = %q, want env when no flag", cfg.outputDir)
	}
}

func TestParseConfigRealFlagForms(t *testing.T) {
	cfg, err := parseConfig([]string{"--skip-tidy", "--skip-build=false", "-keep-tests"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.skipTidy {
		t.Error("--skip-tidy should be true")
	}
	if cfg.skipBuild {
		t.Error("--skip-build=false should be false")
	}
	if !cfg.keepTests {
		t.Error("-keep-tests should be true")
	}
}

func TestParseConfigCapabilitiesCSV(t *testing.T) {
	cfg, err := parseConfig([]string{"-capabilities", " a, b ,c ,"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(cfg.capabilities, want) {
		t.Errorf("capabilities = %v, want %v", cfg.capabilities, want)
	}
}

func TestParseConfigRejectsPositionals(t *testing.T) {
	if _, err := parseConfig([]string{"/tmp/out"}); err == nil {
		t.Error("expected error for unexpected positional argument")
	}
}

func TestParseConfigRejectsUnknownFlag(t *testing.T) {
	if _, err := parseConfig([]string{"-bogus", "x"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}
