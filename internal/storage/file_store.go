package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ToolRecord struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Condition string    `json:"condition"`
	UpdatedAt time.Time `json:"updated_at"`
}
type State struct {
	Tools   []ToolRecord `json:"tools"`
	SavedAt time.Time    `json:"saved_at"`
}
type FileStore struct {
	mu   sync.Mutex
	path string
}

func NewFileStore(path string) *FileStore { return &FileStore{path: path} }
func (s *FileStore) Load() (State, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.loadUnlocked() }
func (s *FileStore) loadUnlocked() (State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, err
	}
	return state, nil
}
func (s *FileStore) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked(state)
}
func (s *FileStore) saveUnlocked(state State) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	state.SavedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, data, 0644); err != nil {
		return err
	}
	return os.Rename(temp, s.path)
}
func (s *FileStore) UpdateTool(tool ToolRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	found := false
	for i := range state.Tools {
		if state.Tools[i].ID == tool.ID {
			state.Tools[i] = tool
			found = true
			break
		}
	}
	if !found {
		state.Tools = append(state.Tools, tool)
	}
	return s.saveUnlocked(state)
}
