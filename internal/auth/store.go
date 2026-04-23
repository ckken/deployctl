package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ckken/deployctl/internal/types"
)

var (
	ErrMissingToken       = errors.New("missing token")
	ErrInvalidToken       = errors.New("invalid token")
	ErrExpiredToken       = errors.New("expired token")
	ErrRevokedToken       = errors.New("revoked token")
	ErrInvalidUploadGrant = errors.New("invalid upload grant")
	ErrExpiredUploadGrant = errors.New("expired upload grant")
	ErrUploadLimitReached = errors.New("upload limit reached")
)

type Store struct {
	tokenPath   string
	grantPath   string
	uploadPath  string
	uploadsRoot string
	mu          sync.Mutex
	tokens      []types.TokenRecord
	grants      []types.UploadGrantRecord
	uploads     []types.UploadRecord
}

func NewStore(dataDir string) *Store {
	return &Store{
		tokenPath:   filepath.Join(dataDir, "tokens.json"),
		grantPath:   filepath.Join(dataDir, "upload_grants.json"),
		uploadPath:  filepath.Join(dataDir, "uploads.json"),
		uploadsRoot: filepath.Join(dataDir, "files"),
	}
}

func (s *Store) UploadRootDir() string {
	return s.uploadsRoot
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
	if err := os.MkdirAll(s.uploadsRoot, 0o755); err != nil {
		return err
	}
	if err := s.loadTokensLocked(); err != nil {
		return err
	}
	if err := s.loadGrantsLocked(); err != nil {
		return err
	}
	return s.loadUploadsLocked()
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

func saveJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s *Store) loadTokensLocked() error {
	s.tokens = []types.TokenRecord{}
	return loadJSONFile(s.tokenPath, &s.tokens)
}

func (s *Store) loadGrantsLocked() error {
	s.grants = []types.UploadGrantRecord{}
	return loadJSONFile(s.grantPath, &s.grants)
}

func (s *Store) loadUploadsLocked() error {
	s.uploads = []types.UploadRecord{}
	return loadJSONFile(s.uploadPath, &s.uploads)
}

func (s *Store) saveTokensLocked() error {
	return saveJSONFile(s.tokenPath, s.tokens)
}

func (s *Store) saveGrantsLocked() error {
	return saveJSONFile(s.grantPath, s.grants)
}

func (s *Store) saveUploadsLocked() error {
	return saveJSONFile(s.uploadPath, s.uploads)
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
	if strings.TrimSpace(expiresIn) == "" {
		return nil, nil
	}
	if strings.HasSuffix(expiresIn, "d") {
		days := strings.TrimSpace(strings.TrimSuffix(expiresIn, "d"))
		if days == "" {
			return nil, errors.New("invalid duration: missing day count")
		}
		duration, err := time.ParseDuration(days + "h")
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		expiry := time.Now().UTC().Add(duration * 24)
		return &expiry, nil
	}
	duration, err := time.ParseDuration(expiresIn)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %w", err)
	}
	expiry := time.Now().UTC().Add(duration)
	return &expiry, nil
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

func grantSummary(record types.UploadGrantRecord, publicBaseURL string) types.UploadGrantSummary {
	remaining := record.MaxFiles - record.UploadCount
	if remaining < 0 {
		remaining = 0
	}
	return types.UploadGrantSummary{
		GrantID:        record.GrantID,
		UploadURL:      buildUploadURL(publicBaseURL, record.GrantCode),
		Folder:         record.Folder,
		MaxFiles:       record.MaxFiles,
		UploadCount:    record.UploadCount,
		RemainingFiles: remaining,
		CreatedAt:      record.CreatedAt,
		ExpiresAt:      record.ExpiresAt,
	}
}

func uploadSummary(record types.UploadRecord) types.UploadSummary {
	return types.UploadSummary{
		UploadID:         record.UploadID,
		GrantID:          record.GrantID,
		OriginalFileName: record.OriginalFileName,
		SavedPath:        record.SavedPath,
		FileURL:          record.FileURL,
		SizeBytes:        record.SizeBytes,
		UploadedAt:       record.UploadedAt,
		Folder:           record.UploadGrantFolder,
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

func defaultGrantFolder(now time.Time) string {
	return path.Join("uploads", now.UTC().Format("2006/01/02"))
}

func sanitizeRelativeFolder(folder string, now time.Time) (string, error) {
	folder = strings.TrimSpace(folder)
	if folder == "" {
		return defaultGrantFolder(now), nil
	}
	folder = strings.ReplaceAll(folder, "\\", "/")
	cleaned := path.Clean("/" + folder)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return defaultGrantFolder(now), nil
	}
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", errors.New("folder must stay inside uploads root")
	}
	parts := strings.Split(cleaned, "/")
	safeParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || part == ".." {
			return "", errors.New("folder contains invalid path segments")
		}
		safeParts = append(safeParts, sanitizePathPart(part))
	}
	return path.Join(safeParts...), nil
}

func sanitizePathPart(input string) string {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "item"
	}
	return out
}

func buildUploadURL(publicBaseURL string, grantCode string) string {
	base := strings.TrimRight(publicBaseURL, "/")
	return base + "/u/" + url.PathEscape(grantCode)
}

func buildFileURL(publicBaseURL string, savedPath string) string {
	base, _ := url.Parse(strings.TrimRight(publicBaseURL, "/"))
	base.Path = path.Join(base.Path, "/files/", savedPath)
	return base.String()
}

func findGrantRecord(records []types.UploadGrantRecord, grantID string, code string) *types.UploadGrantRecord {
	hashedCode := ""
	if code != "" {
		hashedCode = hashSecret(code)
	}
	for i := range records {
		record := &records[i]
		if grantID != "" && record.GrantID == grantID {
			return record
		}
		if hashedCode != "" && record.GrantCodeHash == hashedCode {
			return record
		}
	}
	return nil
}

func validateGrant(record *types.UploadGrantRecord, code string) error {
	if record == nil {
		return ErrInvalidUploadGrant
	}
	if code != "" && hashSecret(code) != record.GrantCodeHash {
		return ErrInvalidUploadGrant
	}
	if record.ExpiresAt != nil && time.Now().UTC().After(*record.ExpiresAt) {
		return ErrExpiredUploadGrant
	}
	if record.MaxFiles > 0 && record.UploadCount >= record.MaxFiles {
		return ErrUploadLimitReached
	}
	return nil
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	stem = sanitizePathPart(stem)
	ext = sanitizePathPart(strings.TrimPrefix(ext, "."))
	if ext == "item" {
		ext = ""
	}
	if ext != "" {
		return stem + "." + ext
	}
	return stem
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

func (s *Store) CreateUploadGrant(req types.CreateUploadGrantRequest, publicBaseURL string) (*types.CreateUploadGrantResponse, error) {
	now := time.Now().UTC()
	folder, err := sanitizeRelativeFolder(req.Folder, now)
	if err != nil {
		return nil, err
	}
	if req.MaxFiles <= 0 {
		req.MaxFiles = 1
	}
	if strings.TrimSpace(req.ExpiresIn) == "" {
		req.ExpiresIn = "24h"
	}
	expiresAt, err := parseExpiry(req.ExpiresIn)
	if err != nil {
		return nil, err
	}
	grantID, err := newOpaque("grt_", 8)
	if err != nil {
		return nil, err
	}
	grantCode, err := newOpaque("upc_", 12)
	if err != nil {
		return nil, err
	}

	record := types.UploadGrantRecord{
		GrantID:       grantID,
		GrantCode:     grantCode,
		GrantCodeHash: hashSecret(grantCode),
		Folder:        folder,
		MaxFiles:      req.MaxFiles,
		UploadCount:   0,
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	s.grants = append(s.grants, record)
	if err := s.saveGrantsLocked(); err != nil {
		return nil, err
	}

	return &types.CreateUploadGrantResponse{
		GrantID:    grantID,
		GrantCode:  grantCode,
		UploadURL:  buildUploadURL(publicBaseURL, grantCode),
		Folder:     folder,
		MaxFiles:   req.MaxFiles,
		CreatedAt:  now,
		ExpiresAt:  expiresAt,
		UploadPath: "/" + folder,
	}, nil
}

func (s *Store) ListUploadGrants(publicBaseURL string) ([]types.UploadGrantSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	out := make([]types.UploadGrantSummary, 0, len(s.grants))
	for _, record := range s.grants {
		out = append(out, grantSummary(record, publicBaseURL))
	}
	return out, nil
}

func (s *Store) DeleteUploadGrant(grantID string) (*types.DeleteUploadGrantResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	for i := range s.grants {
		if s.grants[i].GrantID != grantID {
			continue
		}
		s.grants = append(s.grants[:i], s.grants[i+1:]...)
		if err := s.saveGrantsLocked(); err != nil {
			return nil, err
		}
		return &types.DeleteUploadGrantResponse{GrantID: grantID}, nil
	}
	return nil, ErrInvalidUploadGrant
}

func (s *Store) ListUploads() ([]types.UploadSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	out := make([]types.UploadSummary, 0, len(s.uploads))
	for _, record := range s.uploads {
		out = append(out, uploadSummary(record))
	}
	return out, nil
}

func (s *Store) InspectUploadGrant(code string, publicBaseURL string) (*types.UploadGrantInfoResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	record := findGrantRecord(s.grants, "", code)
	if err := validateGrant(record, code); err != nil {
		return nil, err
	}
	summary := grantSummary(*record, publicBaseURL)
	return &types.UploadGrantInfoResponse{
		UploadURL:      buildUploadURL(publicBaseURL, code),
		Folder:         summary.Folder,
		MaxFiles:       summary.MaxFiles,
		UploadCount:    summary.UploadCount,
		RemainingFiles: summary.RemainingFiles,
		CreatedAt:      summary.CreatedAt,
		ExpiresAt:      summary.ExpiresAt,
		Valid:          true,
	}, nil
}

func (s *Store) LatestUploadForGrant(code string) (*types.UploadFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	record := findGrantRecord(s.grants, "", code)
	if record == nil || hashSecret(code) != record.GrantCodeHash {
		return nil, ErrInvalidUploadGrant
	}
	for i := len(s.uploads) - 1; i >= 0; i-- {
		if s.uploads[i].GrantID != record.GrantID {
			continue
		}
		upload := s.uploads[i]
		return &types.UploadFileResponse{
			UploadID:         upload.UploadID,
			GrantID:          upload.GrantID,
			OriginalFileName: upload.OriginalFileName,
			SavedPath:        upload.SavedPath,
			FileURL:          upload.FileURL,
			SizeBytes:        upload.SizeBytes,
			UploadedAt:       upload.UploadedAt,
			Folder:           upload.UploadGrantFolder,
		}, nil
	}
	return nil, ErrInvalidUploadGrant
}

func (s *Store) SaveUploadedFile(code string, originalName string, contentType string, body io.Reader, publicBaseURL string) (*types.UploadFileResponse, error) {
	if strings.TrimSpace(originalName) == "" {
		return nil, errors.New("file name is required")
	}
	if body == nil {
		return nil, errors.New("file body is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	record := findGrantRecord(s.grants, "", code)
	if err := validateGrant(record, code); err != nil {
		return nil, err
	}

	uploadID, err := newOpaque("upl_", 8)
	if err != nil {
		return nil, err
	}
	safeName := sanitizeFileName(originalName)
	timestamp := time.Now().UTC().Format("20060102-150405")
	storedName := timestamp + "-" + uploadID + "-" + safeName
	folderPath := filepath.Join(s.uploadsRoot, filepath.FromSlash(record.Folder))
	if err := os.MkdirAll(folderPath, 0o755); err != nil {
		return nil, err
	}
	absolutePath := filepath.Join(folderPath, storedName)
	file, err := os.Create(absolutePath)
	if err != nil {
		return nil, err
	}
	size, copyErr := io.Copy(file, body)
	closeErr := file.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	savedPath := path.Join(record.Folder, storedName)
	now := time.Now().UTC()
	uploadRecord := types.UploadRecord{
		UploadID:          uploadID,
		GrantID:           record.GrantID,
		OriginalFileName:  filepath.Base(strings.TrimSpace(originalName)),
		StoredFileName:    storedName,
		SavedPath:         savedPath,
		FileURL:           buildFileURL(publicBaseURL, savedPath),
		ContentType:       contentType,
		SizeBytes:         size,
		UploadedAt:        now,
		UploadGrantFolder: record.Folder,
	}
	record.UploadCount++
	s.uploads = append(s.uploads, uploadRecord)
	if err := s.saveUploadsLocked(); err != nil {
		return nil, err
	}
	if err := s.saveGrantsLocked(); err != nil {
		return nil, err
	}
	return &types.UploadFileResponse{
		UploadID:         uploadRecord.UploadID,
		GrantID:          uploadRecord.GrantID,
		OriginalFileName: uploadRecord.OriginalFileName,
		SavedPath:        uploadRecord.SavedPath,
		FileURL:          uploadRecord.FileURL,
		SizeBytes:        uploadRecord.SizeBytes,
		UploadedAt:       uploadRecord.UploadedAt,
		Folder:           uploadRecord.UploadGrantFolder,
	}, nil
}
