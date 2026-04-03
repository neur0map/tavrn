package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type TavernConfig struct {
	Name    string `yaml:"name"`
	Domain  string `yaml:"domain"`
	Tagline string `yaml:"tagline"`
}

type OwnerConfig struct {
	Name        string `yaml:"name"`
	Fingerprint string `yaml:"fingerprint"`
}

type RoomConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type Config struct {
	Tavern TavernConfig `yaml:"tavern"`
	Owner  OwnerConfig  `yaml:"owner"`
	Rooms  []RoomConfig `yaml:"rooms"`
}

var validRoomTypes = map[string]bool{
	"chat":    true,
	"gallery": true,
	"games":   true,
	"wargame": true,
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Tavern.Name == "" {
		return fmt.Errorf("tavern.yaml: tavern.name is required")
	}
	if c.Tavern.Domain == "" {
		return fmt.Errorf("tavern.yaml: tavern.domain is required")
	}
	if c.Owner.Name == "" {
		return fmt.Errorf("tavern.yaml: owner.name is required")
	}
	if c.Owner.Fingerprint == "" {
		return fmt.Errorf("tavern.yaml: owner.fingerprint is required")
	}
	if len(c.Rooms) == 0 {
		return fmt.Errorf("tavern.yaml: at least one room is required")
	}
	for _, r := range c.Rooms {
		if r.Name == "" {
			return fmt.Errorf("tavern.yaml: room name cannot be empty")
		}
		if !validRoomTypes[r.Type] {
			return fmt.Errorf("tavern.yaml: invalid room type %q for room %q (valid: chat, gallery, games, wargame)", r.Type, r.Name)
		}
	}
	return nil
}

// RoomNames returns room names in config order.
func (c *Config) RoomNames() []string {
	names := make([]string, len(c.Rooms))
	for i, r := range c.Rooms {
		names[i] = r.Name
	}
	return names
}

// FirstRoom returns the first room name (the landing room).
func (c *Config) FirstRoom() string {
	if len(c.Rooms) == 0 {
		return ""
	}
	return c.Rooms[0].Name
}

// RoomIsType checks if a room has a specific type.
func (c *Config) RoomIsType(roomName, roomType string) bool {
	for _, r := range c.Rooms {
		if r.Name == roomName && r.Type == roomType {
			return true
		}
	}
	return false
}

// RoomTypeMap returns a map of room name to room type for fast lookups.
func (c *Config) RoomTypeMap() map[string]string {
	m := make(map[string]string, len(c.Rooms))
	for _, r := range c.Rooms {
		m[r.Name] = r.Type
	}
	return m
}
