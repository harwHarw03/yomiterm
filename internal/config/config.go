package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds user-configurable settings for YomiTerm.
type Config struct {
	DatabasePath            string `toml:"database_path"`
	ShowExamples            bool   `toml:"show_examples"`
	AutoScan                bool   `toml:"auto_scan"`
	EastAsianAmbiguousWidth int    `toml:"east_asian_ambiguous_width"`
	Theme                   string `toml:"theme"`
	MaxPopupDepth           int    `toml:"max_popup_depth"`
}

// DefaultConfig returns sensible defaults for cross-platform terminal use.
func DefaultConfig() *Config {
	return &Config{
		DatabasePath:            DefaultDBPath(),
		ShowExamples:            true,
		AutoScan:                false,
		EastAsianAmbiguousWidth: 1,
		Theme:                   "auto",
		MaxPopupDepth:           5,
	}
}

// DefaultDataDir returns the XDG data directory for yomiterm.
func DefaultDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "yomiterm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".yomiterm"
	}
	return filepath.Join(home, ".local", "share", "yomiterm")
}

// DefaultDBPath returns the default SQLite database path.
func DefaultDBPath() string {
	return filepath.Join(DefaultDataDir(), "yomiterm.db")
}

// DefaultConfigPath returns the default config.toml path.
func DefaultConfigPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "yomiterm", "config.toml")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "yomiterm", "config.toml")
	}
	return filepath.Join(dir, "yomiterm", "config.toml")
}

// Load reads config from path, falling back to defaults if the file does not exist.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		path = DefaultConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, err
	}
	if cfg.DatabasePath == "" {
		cfg.DatabasePath = DefaultDBPath()
	}
	if cfg.EastAsianAmbiguousWidth != 1 && cfg.EastAsianAmbiguousWidth != 2 {
		cfg.EastAsianAmbiguousWidth = 1
	}
	if cfg.Theme == "" {
		cfg.Theme = "auto"
	}
	if cfg.MaxPopupDepth < 1 {
		cfg.MaxPopupDepth = 5
	}
	return cfg, nil
}

// Save writes cfg to path, creating parent directories as needed.
func Save(path string, cfg *Config) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
