// Package storage provides storage implementations for LightRAG.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sirupsen/logrus"
)

// JsonKVStorage implements BaseKVStorage using JSON files for persistence.
type JsonKVStorage struct {
	namespace    string
	workspace    string
	globalConfig map[string]interface{}

	data        map[string]map[string]interface{}
	mu          sync.RWMutex
	dirty       bool
	initialized bool

	logger *logrus.Logger
}

// NewJsonKVStorage creates a new JsonKVStorage instance.
func NewJsonKVStorage(config *StorageConfig, logger *logrus.Logger) *JsonKVStorage {
	if logger == nil {
		logger = logrus.New()
	}
	return &JsonKVStorage{
		namespace:    config.Namespace,
		workspace:    config.Workspace,
		globalConfig: config.GlobalConfig,
		data:         make(map[string]map[string]interface{}),
		logger:       logger,
	}
}

// Namespace returns the namespace for this storage.
func (s *JsonKVStorage) Namespace() string {
	return s.namespace
}

// Workspace returns the workspace path.
func (s *JsonKVStorage) Workspace() string {
	return s.workspace
}

// getFilePath returns the path to the JSON storage file.
func (s *JsonKVStorage) getFilePath() string {
	return filepath.Join(s.workspace, fmt.Sprintf("kv_store_%s.json", s.namespace))
}

// Initialize sets up the storage by loading existing data from disk.
func (s *JsonKVStorage) Initialize(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return nil
	}

	filePath := s.getFilePath()

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// No existing data, start fresh
		s.data = make(map[string]map[string]interface{})
		s.initialized = true
		s.logger.WithField("namespace", s.namespace).Debug("JsonKVStorage initialized with empty data")
		return nil
	}

	// Load existing data
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("failed to parse storage file: %w", err)
	}

	s.initialized = true
	s.logger.WithFields(logrus.Fields{
		"namespace": s.namespace,
		"count":     len(s.data),
	}).Debug("JsonKVStorage initialized with existing data")

	return nil
}

// Finalize cleans up storage resources.
func (s *JsonKVStorage) Finalize(ctx context.Context) error {
	// Ensure any pending changes are saved
	return s.IndexDoneCallback(ctx)
}

// IndexDoneCallback commits storage operations by persisting data to disk.
func (s *JsonKVStorage) IndexDoneCallback(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}

	// Ensure workspace directory exists
	if err := os.MkdirAll(s.workspace, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	filePath := s.getFilePath()

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	s.dirty = false
	s.logger.WithFields(logrus.Fields{
		"namespace": s.namespace,
		"count":     len(s.data),
	}).Debug("JsonKVStorage persisted to disk")

	return nil
}

// Drop removes all data from storage.
func (s *JsonKVStorage) Drop(ctx context.Context) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear in-memory data
	s.data = make(map[string]map[string]interface{})
	s.dirty = false

	// Remove file from disk
	filePath := s.getFilePath()
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return map[string]string{
			"status":  "error",
			"message": fmt.Sprintf("failed to remove file: %v", err),
		}, nil
	}

	s.logger.WithField("namespace", s.namespace).Debug("JsonKVStorage dropped")

	return map[string]string{
		"status":  "success",
		"message": "data dropped",
	}, nil
}

// GetByID retrieves a value by its ID.
func (s *JsonKVStorage) GetByID(ctx context.Context, id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if val, ok := s.data[id]; ok {
		// Return a copy to prevent modification
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = v
		}
		return result, nil
	}

	return nil, nil
}

// GetByIDs retrieves multiple values by their IDs.
func (s *JsonKVStorage) GetByIDs(ctx context.Context, ids []string) ([]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]map[string]interface{}, 0, len(ids))

	for _, id := range ids {
		if val, ok := s.data[id]; ok {
			// Return a copy to prevent modification
			result := make(map[string]interface{})
			for k, v := range val {
				result[k] = v
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// FilterKeys returns keys that don't exist in the storage.
func (s *JsonKVStorage) FilterKeys(ctx context.Context, keys map[string]struct{}) (map[string]struct{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nonExistent := make(map[string]struct{})

	for key := range keys {
		if _, ok := s.data[key]; !ok {
			nonExistent[key] = struct{}{}
		}
	}

	return nonExistent, nil
}

// Upsert inserts or updates data.
func (s *JsonKVStorage) Upsert(ctx context.Context, data map[string]map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, value := range data {
		// Create a copy to prevent external modification
		copied := make(map[string]interface{})
		for k, v := range value {
			copied[k] = v
		}
		s.data[id] = copied
	}

	s.dirty = true
	return nil
}

// Delete removes specific records by their IDs.
func (s *JsonKVStorage) Delete(ctx context.Context, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		delete(s.data, id)
	}

	s.dirty = true
	return nil
}

// IsEmpty checks if the storage is empty.
func (s *JsonKVStorage) IsEmpty(ctx context.Context) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data) == 0, nil
}

// Count returns the number of items in storage.
func (s *JsonKVStorage) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.data)
}

// Keys returns all keys in the storage.
func (s *JsonKVStorage) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// GetAll returns all stored data as a copy.
func (s *JsonKVStorage) GetAll(ctx context.Context) (map[string]map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]map[string]interface{}, len(s.data))
	for id, val := range s.data {
		copied := make(map[string]interface{}, len(val))
		for k, v := range val {
			copied[k] = v
		}
		result[id] = copied
	}
	return result, nil
}

// GetNamespace implements base.StorageNameSpace.
func (s *JsonKVStorage) GetNamespace() string {
	return s.namespace
}

// GetWorkspace implements base.StorageNameSpace.
func (s *JsonKVStorage) GetWorkspace() string {
	return s.workspace
}
