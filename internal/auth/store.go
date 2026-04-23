package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ckken/deployctl/internal/types"
)

var (
	ErrMissingToken = errors.New("missing token")
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
	ErrRevokedToken = errors.New("revoked token")
)

type Store struct {
	path   string
	mu     sync.Mutex
	tokens []types.TokenRecord
}

func NewStore(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, "tokens.json")}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.loadLocked()
}

func (s *Store) loadLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.tokens = []types.TokenRecord{}
			return nil
		}
		return err
	}
	if len(data) == 0 {
		s.tokens = []types.TokenRecord{}
		return nil
	}
	return json.Unmarshal(data, &s.tokens)
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newTokenString() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "dpt_" + hex.EncodeToString(buf), nil
}

func newTokenID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "tok_" + hex.EncodeToString(buf), nil
}

func parseExpiry(expiresIn string) (*time.Time, error) {
	if expiresIn == "" {
		return nil, nil
	}
	dur, err := time.ParseDuration(expiresIn)
	if err != nil {
		return nil, fmt.Errorf("invalid expires_in: %w", err)
	}
	t := time.Now().UTC().Add(dur)
	return &t, nil
}

func validateScope(scope string, projectScope string) error {
	switch {
	case scope == "admin":
		return nil
	case scope == "read-only":
		return nil
	case strings.HasPrefix(scope, "project:"):
		if projectScope == "" {
			return errors.New("project scope requires project_scope")
		}
		return nil
	default:
		return fmt.Errorf("unsupported scope: %s", scope)
	}
}

func (s *Store) CreateToken(ctx context.Context, req types.CreateTokenRequest) (*types.CreateTokenResponse, error) {
	_ = ctx

	if req.Name == "" {
		return nil, errors.New("token name is required")
	}
	if req.Scope == "" {
		return nil, errors.New("scope is required")
	}
	if err := validateScope(req.Scope, req.ProjectScope); err != nil {
		return nil, err
	}

	expiresAt, err := parseExpiry(req.ExpiresIn)
	if err != nil {
		return nil, err
	}

	token, err := newTokenString()
	if err != nil {
		return nil, err
	}
	tokenID, err := newTokenID()
	if err != nil {
		return nil, err
	}

	record := types.TokenRecord{
		TokenID:      tokenID,
		TokenName:    req.Name,
		Scope:        req.Scope,
		ProjectScope: req.ProjectScope,
		TokenHash:    hashToken(token),
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    expiresAt,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	s.tokens = append(s.tokens, record)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return &types.CreateTokenResponse{
		TokenID:      record.TokenID,
		Token:        token,
		TokenName:    record.TokenName,
		Scope:        record.Scope,
		ProjectScope: record.ProjectScope,
		ExpiresAt:    record.ExpiresAt,
	}, nil
}

func (s *Store) ListTokens() ([]types.TokenSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}

	out := make([]types.TokenSummary, 0, len(s.tokens))
	for _, record := range s.tokens {
		out = append(out, types.TokenSummary{
			TokenID:      record.TokenID,
			TokenName:    record.TokenName,
			Scope:        record.Scope,
			ProjectScope: record.ProjectScope,
			CreatedAt:    record.CreatedAt,
			ExpiresAt:    record.ExpiresAt,
			RevokedAt:    record.RevokedAt,
		})
	}
	return out, nil
}

func (s *Store) RevokeToken(tokenID string) (*types.RevokeTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}

	for i := range s.tokens {
		if s.tokens[i].TokenID != tokenID {
			continue
		}
		now := time.Now().UTC()
		s.tokens[i].RevokedAt = &now
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return &types.RevokeTokenResponse{TokenID: tokenID, RevokedAt: &now}, nil
	}
	return nil, ErrInvalidToken
}

func (s *Store) ValidateToken(raw string) (*types.TokenRecord, error) {
	if raw == "" {
		return nil, ErrMissingToken
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}

	hashed := hashToken(raw)
	for i := range s.tokens {
		record := s.tokens[i]
		if record.TokenHash != hashed {
			continue
		}
		if record.RevokedAt != nil {
			return nil, ErrRevokedToken
		}
		if record.ExpiresAt != nil && time.Now().UTC().After(*record.ExpiresAt) {
			return nil, ErrExpiredToken
		}
		return &record, nil
	}
	return nil, ErrInvalidToken
}
