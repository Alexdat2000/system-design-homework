package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"

	"external/api"
)

// Storage holds in-memory data for all entities
type Storage struct {
	mu       sync.RWMutex
	scooters map[string]*api.ScooterData
	users    map[string]*api.UserProfile
	zones    map[string]*api.TariffZone
	configs  map[string]any
}

// NewStorage creates a new storage instance
func NewStorage() *Storage {
	return &Storage{
		scooters: make(map[string]*api.ScooterData),
		users:    make(map[string]*api.UserProfile),
		zones:    make(map[string]*api.TariffZone),
		configs:  make(map[string]any),
	}
}

// LoadFromJSONFiles loads data from JSON files in data/ directory
func (s *Storage) LoadFromJSONFiles() error {
	// Load scooters
	if err := s.loadScooters("data/scooters.json"); err != nil {
		return fmt.Errorf("failed to load scooters: %w", err)
	}

	// Load users
	if err := s.loadUsers("data/users.json"); err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}

	// Load zones
	if err := s.loadZones("data/zones.json"); err != nil {
		return fmt.Errorf("failed to load zones: %w", err)
	}

	// Load configs
	if err := s.loadConfigs("data/configs.json"); err != nil {
		return fmt.Errorf("failed to load configs: %w", err)
	}

	return nil
}

func (s *Storage) loadScooters(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var scooters []api.ScooterData
	if err := json.Unmarshal(data, &scooters); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range scooters {
		s.scooters[scooters[i].Id] = &scooters[i]
	}

	return nil
}

func (s *Storage) loadUsers(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var users []api.UserProfile
	if err := json.Unmarshal(data, &users); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range users {
		s.users[users[i].Id] = &users[i]
	}

	return nil
}

func (s *Storage) loadZones(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var zones []api.TariffZone
	if err := json.Unmarshal(data, &zones); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range zones {
		s.zones[zones[i].Id] = &zones[i]
	}

	return nil
}

func (s *Storage) loadConfigs(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(data, &s.configs)
}

// GetScooter returns scooter by ID
func (s *Storage) GetScooter(id string) (*api.ScooterData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	scooter, ok := s.scooters[id]
	return scooter, ok
}

// GetUser returns user by ID
func (s *Storage) GetUser(id string) (*api.UserProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return user, ok
}

// GetZone returns zone by ID
func (s *Storage) GetZone(id string) (*api.TariffZone, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	zone, ok := s.zones[id]
	return zone, ok
}

// GetConfigs returns all configs
func (s *Storage) GetConfigs() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Return a copy to avoid race conditions
	result := make(map[string]any)
	maps.Copy(result, s.configs)
	return result
}
