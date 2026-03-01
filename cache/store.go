package cache

import (
	"fmt"
	"sync"
)

// Store represents a thread-safe Key-Value store
type Store struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

func NewStore() *Store {
	return &Store{
		data: make(map[string]interface{}),
	}
}

// Set stores a value for a given key. It enforces the value types to be int, bool, string, or float64.
func (s *Store) Set(key string, value interface{}) error {
	switch value.(type) {
	case int, bool, string, float64:
		// These types are allowed
	case float32: // In Go, JSON numbers typically unmarshal to float64, but treating float32 as float64
		value = float64(value.(float32))
	default:
		return fmt.Errorf("invalid value type: %T. Only int, bool, string, and float are allowed", value)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return nil
}

// Get retrieves the value for a given key. Returns nil, false if not found.
func (s *Store) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Delete removes a key from the store.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Keys returns a snapshot of all keys currently in the store.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}
