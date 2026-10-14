// Package config loads oops settings. Every key has a default.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Config is the on-disk configuration.
type Config struct {
	Snapshot  Snapshot  `toml:"snapshot"`
	Retention Retention `toml:"retention"`
	Privacy   Privacy   `toml:"privacy"`
	UI        UI        `toml:"ui"`
	Run       Run       `toml:"run"`
}

// Snapshot controls what is captured.
type Snapshot struct {
	MaxFileSize      string   `toml:"max_file_size"`
	Timeout          string   `toml:"timeout"`
	Protect          []string `toml:"protect"`
	Exclude          []string `toml:"exclude"`
	RespectGitignore bool     `toml:"respect_gitignore"`
}

// Retention is the storage budget.
type Retention struct {
	KeepTurns    int    `toml:"keep_turns"`
	KeepDays     int    `toml:"keep_days"`
	WorkspaceMax string `toml:"workspace_max"`
	GlobalMax    string `toml:"global_max"`
}

// Privacy controls prompt capture.
type Privacy struct {
	RecordPrompts  bool `toml:"record_prompts"`
	PromptMaxChars int  `toml:"prompt_max_chars"`
}

// UI controls presentation.
type UI struct {
	Color string `toml:"color"`
	Pager string `toml:"pager"`
	Time  string `toml:"time"`
}

// Run controls the generic wrapper.
type Run struct {
	IdleSplit string `toml:"idle_split"`
}

// Defaults returns the documented defaults.
func Defaults() Config {
	return Config{
		Snapshot: Snapshot{
			MaxFileSize:      "50MB",
			Timeout:          "10s",
			Protect:          []string{".env", ".env.*", ".envrc", "*.local", "*.local.*"},
			Exclude:          nil,
			RespectGitignore: true,
		},
		Retention: Retention{
			KeepTurns:    500,
			KeepDays:     30,
			WorkspaceMax: "2GB",
			GlobalMax:    "10GB",
		},
		Privacy: Privacy{RecordPrompts: true, PromptMaxChars: 200},
		UI:      UI{Color: "auto", Pager: "auto", Time: "relative"},
		Run:     Run{IdleSplit: "3s"},
	}
}

// DataHome is the root of oops state.
func DataHome() (string, error) {
	if v := os.Getenv("OOPS_HOME"); v != "" {
		return v, nil
	}
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(base, "oops"), nil
	default:
		base := os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(base, "oops"), nil
	}
}

// ConfigPath is the user config file.
func ConfigPath() (string, error) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, "AppData", "Roaming")
		}
		return filepath.Join(base, "oops", "config.toml"), nil
	default:
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
		return filepath.Join(base, "oops", "config.toml"), nil
	}
}

// Load reads path, or returns defaults when the file is missing.
func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		return cfg, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return cfg, err
	}
	def := Defaults()
	if cfg.Snapshot.MaxFileSize == "" {
		cfg.Snapshot.MaxFileSize = def.Snapshot.MaxFileSize
	}
	if cfg.Snapshot.Timeout == "" {
		cfg.Snapshot.Timeout = def.Snapshot.Timeout
	}
	if cfg.Retention.KeepTurns == 0 {
		cfg.Retention.KeepTurns = def.Retention.KeepTurns
	}
	if cfg.Retention.KeepDays == 0 {
		cfg.Retention.KeepDays = def.Retention.KeepDays
	}
	if cfg.Retention.WorkspaceMax == "" {
		cfg.Retention.WorkspaceMax = def.Retention.WorkspaceMax
	}
	if cfg.Retention.GlobalMax == "" {
		cfg.Retention.GlobalMax = def.Retention.GlobalMax
	}
	if cfg.Privacy.PromptMaxChars == 0 {
		cfg.Privacy.PromptMaxChars = def.Privacy.PromptMaxChars
	}
	if cfg.UI.Color == "" {
		cfg.UI.Color = def.UI.Color
	}
	if cfg.UI.Pager == "" {
		cfg.UI.Pager = def.UI.Pager
	}
	if cfg.Run.IdleSplit == "" {
		cfg.Run.IdleSplit = def.Run.IdleSplit
	}
	if cfg.Snapshot.Protect == nil {
		cfg.Snapshot.Protect = def.Snapshot.Protect
	}
	return cfg, nil
}

// ParseSize parses values such as 50MB, 2GB, or a bare byte count.
func ParseSize(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	var n int64
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int64(s[i]-'0')
		i++
	}
	unit := s[i:]
	switch unit {
	case "", "B":
		return n, nil
	case "KB", "K":
		return n * 1000, nil
	case "MB", "M":
		return n << 20, nil
	case "GB", "G":
		return n << 30, nil
	default:
		d, err := time.ParseDuration(s)
		if err == nil {
			return int64(d), nil
		}
		return 0, err
	}
}

// ParseDuration parses a Go duration. Empty yields 0.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}
