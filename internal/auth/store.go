package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ckken/deployctl/internal/types"
)

var (
	ErrMissingToken      = errors.New("missing token")
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("expired token")
	ErrRevokedToken      = errors.New("revoked token")
	ErrInvalidShareLink  = errors.New("invalid share link")
	ErrExpiredShareLink  = errors.New("expired share link")
	ErrRevokedShareLink  = errors.New("revoked share link")
	ErrClaimLimitReached = errors.New("share link claim limit reached")
)

type Store struct {
	tokenPath string
	sharePath string
	mu        sync.Mutex
	tokens    []types.TokenRecord
	shares    []types.ShareLinkRecord
}

func NewStore(dataDir string) *Store {
	return &Store{
		tokenPath: filepath.Join(dataDir, "tokens.json"),
		sharePath: filepath.Join(dataDir, "share_links.json"),
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.tokenPath), 0o755); err != nil {
		return err
	}
	if err := s.loadTokensLocked(); err != nil {
		return err
	}
	return s.loadSharesLocked()
}

func loadJSONFile(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dest)
}

func (s *Store) loadTokensLocked() error {
	s.tokens = []types.TokenRecord{}
	return loadJSONFile(s.tokenPath, &s.tokens)
}

func (s *Store) loadSharesLocked() error {
	s.shares = []types.ShareLinkRecord{}
	return loadJSONFile(s.sharePath, &s.shares)
}

func saveJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s *Store) saveTokensLocked() error {
	return saveJSONFile(s.tokenPath, s.tokens)
}

func (s *Store) saveSharesLocked() error {
	return saveJSONFile(s.sharePath, s.shares)
}

func hashSecret(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newOpaque(prefix string, n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(buf), nil
}

func parseExpiry(expiresIn string) (*time.Time, error) {
	if expiresIn == "" {
		return nil, nil
	}
	if strings.HasSuffix(expiresIn, "d") {
		days := strings.TrimSuffix(expiresIn, "d")
		days = strings.TrimSpace(days)
		if days == "" {
			return nil, errors.New("invalid duration: missing day count")
		}
		dayCount, err := time.ParseDuration(days + "h")
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		t := time.Now().UTC().Add(dayCount * 24)
		return &t, nil
	}
	dur, err := time.ParseDuration(expiresIn)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
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

func tokenSummary(record types.TokenRecord) types.TokenSummary {
	return types.TokenSummary{
		TokenID:      record.TokenID,
		TokenName:    record.TokenName,
		Scope:        record.Scope,
		ProjectScope: record.ProjectScope,
		CreatedAt:    record.CreatedAt,
		ExpiresAt:    record.ExpiresAt,
		RevokedAt:    record.RevokedAt,
	}
}

func shareSummary(record types.ShareLinkRecord) types.ShareLinkSummary {
	return types.ShareLinkSummary{
		ShareID:        record.ShareID,
		ShareName:      record.ShareName,
		Scope:          record.Scope,
		ProjectScope:   record.ProjectScope,
		TokenName:      record.TokenName,
		TokenExpiresIn: record.TokenExpiresIn,
		MaxClaims:      record.MaxClaims,
		ClaimCount:     record.ClaimCount,
		CreatedAt:      record.CreatedAt,
		ExpiresAt:      record.ExpiresAt,
		RevokedAt:      record.RevokedAt,
		ClaimedAt:      record.ClaimedAt,
	}
}

func createTokenRecord(name string, scope string, projectScope string, expiresIn string) (types.TokenRecord, string, error) {
	if name == "" {
		return types.TokenRecord{}, "", errors.New("token name is required")
	}
	if scope == "" {
		return types.TokenRecord{}, "", errors.New("scope is required")
	}
	if err := validateScope(scope, projectScope); err != nil {
		return types.TokenRecord{}, "", err
	}
	expiresAt, err := parseExpiry(expiresIn)
	if err != nil {
		return types.TokenRecord{}, "", err
	}
	token, err := newOpaque("dpt_", 32)
	if err != nil {
		return types.TokenRecord{}, "", err
	}
	tokenID, err := newOpaque("tok_", 8)
	if err != nil {
		return types.TokenRecord{}, "", err
	}
	record := types.TokenRecord{
		TokenID:      tokenID,
		TokenName:    name,
		Scope:        scope,
		ProjectScope: projectScope,
		TokenHash:    hashSecret(token),
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    expiresAt,
	}
	return record, token, nil
}

func (s *Store) CreateToken(ctx context.Context, req types.CreateTokenRequest) (*types.CreateTokenResponse, error) {
	_ = ctx

	record, token, err := createTokenRecord(req.Name, req.Scope, req.ProjectScope, req.ExpiresIn)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	s.tokens = append(s.tokens, record)
	if err := s.saveTokensLocked(); err != nil {
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
		out = append(out, tokenSummary(record))
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
		if err := s.saveTokensLocked(); err != nil {
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
	hashed := hashSecret(raw)
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

func buildClaimURL(publicBaseURL string, shareID string) string {
	base := strings.TrimRight(publicBaseURL, "/")
	return base + "/v1/share-links/claim?share_id=" + url.QueryEscape(shareID)
}

func buildResolveURL(publicBaseURL string, shareID string) string {
	base := strings.TrimRight(publicBaseURL, "/")
	return base + "/v1/share-links/resolve?share_id=" + url.QueryEscape(shareID)
}

func (s *Store) CreateShareLink(req types.CreateShareLinkRequest, publicBaseURL string) (*types.CreateShareLinkResponse, error) {
	if req.ShareName == "" {
		return nil, errors.New("share_name is required")
	}
	if req.TokenName == "" {
		return nil, errors.New("token_name is required")
	}
	if req.Scope == "" {
		return nil, errors.New("scope is required")
	}
	if err := validateScope(req.Scope, req.ProjectScope); err != nil {
		return nil, err
	}
	if req.MaxClaims <= 0 {
		req.MaxClaims = 1
	}
	expiresAt, err := parseExpiry(req.ShareExpiresIn)
	if err != nil {
		return nil, err
	}
	shareID, err := newOpaque("shr_", 8)
	if err != nil {
		return nil, err
	}
	shareCode, err := newOpaque("shc_", 12)
	if err != nil {
		return nil, err
	}
	record := types.ShareLinkRecord{
		ShareID:        shareID,
		ShareName:      req.ShareName,
		ShareCodeHash:  hashSecret(shareCode),
		Scope:          req.Scope,
		ProjectScope:   req.ProjectScope,
		TokenName:      req.TokenName,
		TokenExpiresIn: req.TokenExpiresIn,
		MaxClaims:      req.MaxClaims,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      expiresAt,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	s.shares = append(s.shares, record)
	if err := s.saveSharesLocked(); err != nil {
		return nil, err
	}

	return &types.CreateShareLinkResponse{
		ShareID:        shareID,
		ShareCode:      shareCode,
		ShareName:      record.ShareName,
		TokenName:      record.TokenName,
		Scope:          record.Scope,
		ProjectScope:   record.ProjectScope,
		TokenExpiresIn: record.TokenExpiresIn,
		MaxClaims:      record.MaxClaims,
		ExpiresAt:      record.ExpiresAt,
		ResolveURL:     buildResolveURL(publicBaseURL, shareID),
		ClaimURL:       buildClaimURL(publicBaseURL, shareID),
	}, nil
}

func (s *Store) ListShareLinks() ([]types.ShareLinkSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	out := make([]types.ShareLinkSummary, 0, len(s.shares))
	for _, record := range s.shares {
		out = append(out, shareSummary(record))
	}
	return out, nil
}

func (s *Store) RevokeShareLink(shareID string) (*types.RevokeShareLinkResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	for i := range s.shares {
		if s.shares[i].ShareID != shareID {
			continue
		}
		now := time.Now().UTC()
		s.shares[i].RevokedAt = &now
		if err := s.saveSharesLocked(); err != nil {
			return nil, err
		}
		return &types.RevokeShareLinkResponse{ShareID: shareID, RevokedAt: &now}, nil
	}
	return nil, ErrInvalidShareLink
}

func validateShare(record *types.ShareLinkRecord, code string) error {
	if record == nil {
		return ErrInvalidShareLink
	}
	if record.RevokedAt != nil {
		return ErrRevokedShareLink
	}
	if record.ExpiresAt != nil && time.Now().UTC().After(*record.ExpiresAt) {
		return ErrExpiredShareLink
	}
	if record.MaxClaims > 0 && record.ClaimCount >= record.MaxClaims {
		return ErrClaimLimitReached
	}
	if hashSecret(code) != record.ShareCodeHash {
		return ErrInvalidShareLink
	}
	return nil
}

func (s *Store) ResolveShareLink(shareID string, code string, publicBaseURL string) (*types.ResolveShareLinkResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	for _, record := range s.shares {
		if record.ShareID != shareID {
			continue
		}
		if err := validateShare(&record, code); err != nil {
			return nil, err
		}
		return &types.ResolveShareLinkResponse{
			ShareID:      record.ShareID,
			ShareName:    record.ShareName,
			TokenName:    record.TokenName,
			Scope:        record.Scope,
			ProjectScope: record.ProjectScope,
			ServerURL:    strings.TrimRight(publicBaseURL, "/"),
			ClaimURL:     buildClaimURL(publicBaseURL, record.ShareID),
			Valid:        true,
			MaxClaims:    record.MaxClaims,
			ClaimCount:   record.ClaimCount,
			ExpiresAt:    record.ExpiresAt,
		}, nil
	}
	return nil, ErrInvalidShareLink
}

func (s *Store) ClaimShareLink(req types.ClaimShareLinkRequest, publicBaseURL string) (*types.ClaimShareLinkResponse, error) {
	if req.ShareID == "" || req.Code == "" {
		return nil, ErrInvalidShareLink
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}

	for i := range s.shares {
		record := &s.shares[i]
		if record.ShareID != req.ShareID {
			continue
		}
		if err := validateShare(record, req.Code); err != nil {
			return nil, err
		}

		tokenRecord, rawToken, err := createTokenRecord(record.TokenName, record.Scope, record.ProjectScope, record.TokenExpiresIn)
		if err != nil {
			return nil, err
		}
		s.tokens = append(s.tokens, tokenRecord)
		record.ClaimCount++
		now := time.Now().UTC()
		if record.ClaimedAt == nil {
			record.ClaimedAt = &now
		}
		if err := s.saveTokensLocked(); err != nil {
			return nil, err
		}
		if err := s.saveSharesLocked(); err != nil {
			return nil, err
		}

		return &types.ClaimShareLinkResponse{
			ServerURL:    strings.TrimRight(publicBaseURL, "/"),
			Token:        rawToken,
			TokenID:      tokenRecord.TokenID,
			TokenName:    tokenRecord.TokenName,
			Scope:        tokenRecord.Scope,
			ProjectScope: tokenRecord.ProjectScope,
			ExpiresAt:    tokenRecord.ExpiresAt,
		}, nil
	}
	return nil, ErrInvalidShareLink
}
