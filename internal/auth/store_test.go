package auth

import (
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

func TestLoadPersistsTokens(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}
	resp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:  "persisted",
		Scope: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	store2 := NewStore(dir)
	if err := store2.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := store2.ValidateToken(resp.Token); err != nil {
		t.Fatalf("expected token to survive reload from %s: %v", filepath.Join(dir, "tokens.json"), err)
	}
}

func TestShareLinkResolveAndClaim(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}

	shareResp, err := store.CreateShareLink(types.CreateShareLinkRequest{
		ShareName:      "agent access",
		TokenName:      "agent-bot",
		Scope:          "project:demo",
		ProjectScope:   "demo",
		ShareExpiresIn: "1h",
		TokenExpiresIn: "24h",
		MaxClaims:      1,
	}, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}

	resolveResp, err := store.ResolveShareLink("", shareResp.ShareCode, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !resolveResp.Valid || resolveResp.ServerURL != "https://deploy.example.com" {
		t.Fatalf("unexpected resolve response: %+v", resolveResp)
	}

	claimResp, err := store.ClaimShareLink(types.ClaimShareLinkRequest{
		ShareID: shareResp.ShareID,
		Code:    shareResp.ShareCode,
	}, "https://deploy.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if claimResp.Token == "" || claimResp.Scope != "project:demo" || claimResp.ProjectScope != "demo" {
		t.Fatalf("unexpected claim response: %+v", claimResp)
	}
	if _, err := store.ValidateToken(claimResp.Token); err != nil {
		t.Fatalf("expected claimed token to validate: %v", err)
	}
	if _, err := store.ClaimShareLink(types.ClaimShareLinkRequest{
		Code: shareResp.ShareCode,
	}, "https://deploy.example.com"); err != ErrClaimLimitReached {
		t.Fatalf("expected claim limit error, got %v", err)
	}
}
