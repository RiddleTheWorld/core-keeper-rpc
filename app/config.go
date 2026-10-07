package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Lines struct {
	Details string `json:"details"`
	State   string `json:"state"`
}

type Config struct {
	Enabled    bool             `json:"enabled"`
	ShowTimer  bool             `json:"showTimer"`
	ShowImage  bool             `json:"showImage"`
	ImageText  string           `json:"imageText"`
	UpdateSecs int              `json:"updateSecs"` // minimum seconds between Discord updates; 3 is the fastest the app allows
	Situations map[string]Lines `json:"situations"`
}

func DefaultConfig() Config {
	return Config{
		Enabled: true, ShowTimer: true, ShowImage: true, ImageText: "Core Keeper", UpdateSecs: 3,
		Situations: map[string]Lines{
			"menu":      {Details: "In the main menu", State: ""},
			"exploring": {Details: "Exploring {biome}", State: "⛏ {held} · {distance} tiles from the Core · {partytext}"},
			"base":      {Details: "At base in {biome}", State: "{partytext} · World: {world}"},
			"dungeon":   {Details: "Exploring the {place}", State: "❤ {hp}/{maxhp} · {held} · {partytext}"},
			"boss":      {Details: "Fighting {boss}", State: "❤ {hp}/{maxhp} · {held} · {partytext}"},
			"dead":      {Details: "Knocked out in {biome}", State: "{distance} tiles from the Core · {partytext}"},
		},
	}
}

func configPath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "CKPresence", "config.json")
}

// LoadConfig returns the saved config, filling anything missing from the defaults.
func LoadConfig() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(configPath())
	if err != nil {
		return cfg
	}
	if json.Unmarshal(data, &cfg) != nil {
		return DefaultConfig() // unreadable file: start from the defaults
	}
	if cfg.UpdateSecs == 5 { // the old "Fast" setting; Fast is now 3 s
		cfg.UpdateSecs = 3
	}
	if cfg.Situations == nil {
		cfg.Situations = DefaultConfig().Situations // "situations": null
	}
	// Situations still holding a first-release default are upgraded to the current
	// one; wording the player has changed (or emptied) is kept as it is.
	defaults := DefaultConfig().Situations
	for k, prev := range previousDefaults {
		if cfg.Situations[k] == prev {
			cfg.Situations[k] = defaults[k]
		}
	}
	return cfg
}

func SaveConfig(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

// previousDefaults are the first release's defaults; untouched ones are upgraded on load.
var previousDefaults = map[string]Lines{
	"exploring": {Details: "Exploring {biome}", State: "❤ {hp}/{maxhp} · {partytext} · World: {world}"},
	"base":      {Details: "At base · {biome}", State: "{partytext} · World: {world}"},
	"dungeon":   {Details: "Exploring the {place}", State: "❤ {hp}/{maxhp} · {partytext}"},
	"boss":      {Details: "Fighting {boss}", State: "❤ {hp}/{maxhp} · {partytext}"},
	"dead":      {Details: "Knocked out in {biome}", State: "{partytext} · World: {world}"},
}
