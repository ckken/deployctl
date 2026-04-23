package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultServerURL = "http://127.0.0.1:7319"
	ConfigEnvToken   = "DEPLOYCTL_TOKEN"
)

type Config struct {
	ServerURL string
	Token     string
}

type TokenSource struct {
	Token  string
	Source string
}

func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".deployctl"), nil
}

func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{ServerURL: DefaultServerURL}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return Config{}, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		switch key {
		case "server":
			cfg.ServerURL = value
		case "token":
			cfg.Token = value
		}
	}
	return cfg, nil
}

func Save(cfg Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = DefaultServerURL
	}
	content := fmt.Sprintf("server = %q\ntoken = %q\n", cfg.ServerURL, cfg.Token)
	return os.WriteFile(path, []byte(content), 0o600)
}

func ResolveToken(flagToken string, cfg Config) TokenSource {
	if flagToken != "" {
		return TokenSource{Token: flagToken, Source: "flag"}
	}
	if envToken := os.Getenv(ConfigEnvToken); envToken != "" {
		return TokenSource{Token: envToken, Source: "env"}
	}
	if cfg.Token != "" {
		return TokenSource{Token: cfg.Token, Source: "config"}
	}
	return TokenSource{Source: "missing"}
}
