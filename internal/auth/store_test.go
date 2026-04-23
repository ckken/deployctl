package auth

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ckken/deployctl/internal/types"
)

func TestCreateTokenStoresOnlyHash(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	resp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:  "ci-bot",
		Scope: "read-only",
	})
	if err != nil {
		t.Fatal(err)
	}

	records, err := store.ListTokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 token, got %d", len(records))
	}
	data, err := os.ReadFile(filepath.Join(dir, "tokens.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), resp.Token) {
		t.Fatal("token plaintext leaked into persisted file")
	}
}

func TestValidateTokenExpiredAndRevoked(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	expiredResp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:      "expired",
		Scope:     "read-only",
		ExpiresIn: "1ms",
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	if _, err := store.ValidateToken(expiredResp.Token); err != ErrExpiredToken {
		t.Fatalf("expected ErrExpiredToken, got %v", err)
	}

	validResp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:         "writer",
		Scope:        "project:demo",
		ProjectScope: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.RevokeToken(validResp.TokenID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ValidateToken(validResp.Token); err != ErrRevokedToken {
		t.Fatalf("expected ErrRevokedToken, got %v", err)
	}
}

func TestCreateUploadGrantUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	resp, err := store.CreateUploadGrant(types.CreateUploadGrantRequest{}, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resp.Folder, "uploads/") {
		t.Fatalf("expected default folder, got %s", resp.Folder)
	}
	if resp.MaxFiles != 1 {
		t.Fatalf("expected default max_files=1, got %d", resp.MaxFiles)
	}
	if !strings.HasPrefix(resp.UploadURL, "https://deploy.example.com/u/upc_") {
		t.Fatalf("unexpected upload url: %s", resp.UploadURL)
	}

}

func TestUploadGrantInspectUploadAndLimit(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	grantResp, err := store.CreateUploadGrant(types.CreateUploadGrantRequest{
		Folder:    "releases/demo",
		ExpiresIn: "1h",
		MaxFiles:  1,
	}, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}

	info, err := store.InspectUploadGrant(grantResp.GrantCode, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Valid || info.Folder != "releases/demo" {
		t.Fatalf("unexpected info response: %+v", info)
	}

	uploadResp, err := store.SaveUploadedFile(grantResp.GrantCode, "build.zip", "application/zip", bytes.NewBufferString("payload"), "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if uploadResp.FileURL == "" || !strings.HasPrefix(uploadResp.SavedPath, "releases/demo/") {
		t.Fatalf("unexpected upload response: %+v", uploadResp)
	}
	absoluteFile := filepath.Join(store.UploadRootDir(), filepath.FromSlash(uploadResp.SavedPath))
	if _, err := os.Stat(absoluteFile); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}

	latest, err := store.LatestUploadForGrant(grantResp.GrantCode)
	if err != nil {
		t.Fatal(err)
	}
	if latest.UploadID != uploadResp.UploadID {
		t.Fatalf("expected latest upload %s, got %s", uploadResp.UploadID, latest.UploadID)
	}

	if _, err := store.SaveUploadedFile(grantResp.GrantCode, "again.zip", "application/zip", bytes.NewBufferString("payload"), "https://deploy.example.com"); err != ErrUploadLimitReached {
		t.Fatalf("expected limit error, got %v", err)
	}
}

func TestDeleteUploadGrantRemovesRecord(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	grantResp, err := store.CreateUploadGrant(types.CreateUploadGrantRequest{
		Folder: "handoff/demo",
	}, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.DeleteUploadGrant(grantResp.GrantID); err != nil {
		t.Fatal(err)
	}

	records, err := store.ListUploadGrants("https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected upload grant to be deleted, got %d", len(records))
	}
	if _, err := store.InspectUploadGrant(grantResp.GrantCode, "https://deploy.example.com"); err != ErrInvalidUploadGrant {
		t.Fatalf("expected invalid upload grant after delete, got %v", err)
	}
}
