package tokens

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/toto/withingy/internal/paths"
)

// Token represents persisted OAuth token data.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Scope        []string  `json:"scope"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Store manages secure persistence of tokens on disk.
type Store struct {
	path string
	mu   sync.Mutex
}

// Path returns the absolute file path backing this store.
func (s *Store) Path() string {
	return s.path
}

// NewStore creates a token store using the default token path unless an override is provided.
func NewStore(customPath string) (*Store, error) {
	var path string
	if customPath != "" {
		path = customPath
	} else {
		var err error
		path, err = paths.TokensFile()
		if err != nil {
			return nil, err
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("prepare token directory: %w", err)
	}

	return &Store{path: path}, nil
}

// Load returns the stored token or nil if none exists.
func (s *Store) Load() (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parse token file: %w", err)
	}
	return &token, nil
}

// Save persists the provided token atomically.
func (s *Store) Save(token *Token) error {
	if token == nil {
		return errors.New("token is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return fmt.Errorf("write temp token file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename token file: %w", err)
	}
	return nil
}

// Clear removes the stored token from disk.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove token file: %w", err)
	}
	return nil
}
