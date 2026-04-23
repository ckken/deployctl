package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndSave(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := Config{
		ServerURL: "http://localhost:9999",
		Token:     "abc",
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ServerURL != cfg.ServerURL || loaded.Token != cfg.Token {
		t.Fatalf("unexpected config: %+v", loaded)
	}

	path := filepath.Join(home, ".deployctl", "config.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file at %s: %v", path, err)
	}
}

func TestResolveTokenPrecedence(t *testing.T) {
	cfg := Config{Token: "config-token"}

	if got := ResolveToken("", cfg); got.Source != "config" || got.Token != "config-token" {
		t.Fatalf("expected config token, got %+v", got)
	}

	t.Setenv(ConfigEnvToken, "env-token")
	if got := ResolveToken("", cfg); got.Source != "env" || got.Token != "env-token" {
		t.Fatalf("expected env token, got %+v", got)
	}

	if got := ResolveToken("flag-token", cfg); got.Source != "flag" || got.Token != "flag-token" {
		t.Fatalf("expected flag token, got %+v", got)
	}
}
