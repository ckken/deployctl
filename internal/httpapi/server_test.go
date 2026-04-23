package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ckken/deployctl/internal/auth"
	"github.com/ckken/deployctl/internal/types"
)

func newTestServer(t *testing.T) (*auth.Store, *httptest.Server) {
	t.Helper()
	store := auth.NewStore(t.TempDir())
	if err := store.Load(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(store, "secret").Handler(""))
	return store, srv
}

func TestHealthAndWhoAmI(t *testing.T) {
	store, srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	tokenResp, err := store.CreateToken(context.Background(), types.CreateTokenRequest{
		Name:  "test",
		Scope: "read-only",
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/auth/whoami", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var who types.WhoAmIResponse
	if err := json.NewDecoder(resp2.Body).Decode(&who); err != nil {
		t.Fatal(err)
	}
	if who.TokenName != "test" || who.Scope != "read-only" {
		t.Fatalf("unexpected whoami: %+v", who)
	}
}

func TestWhoAmIWithoutToken(t *testing.T) {
	_, srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/auth/whoami")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAdminSecretAndRevoke(t *testing.T) {
	store, srv := newTestServer(t)
	defer srv.Close()

	body := `{"name":"ci-bot","scope":"read-only"}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/tokens", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without secret, got %d", resp.StatusCode)
	}

	req2, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/tokens", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Admin-Secret", "secret")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp2.StatusCode)
	}
	var created types.CreateTokenResponse
	if err := json.NewDecoder(resp2.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	req3, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/tokens/"+created.TokenID+"/revoke", nil)
	if err != nil {
		t.Fatal(err)
	}
	req3.Header.Set("X-Admin-Secret", "secret")
	resp3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}

	if _, err := store.ValidateToken(created.Token); err != auth.ErrRevokedToken {
		t.Fatalf("expected revoked token, got %v", err)
	}
}

func TestShareLinkHTTPFlow(t *testing.T) {
	_, srv := newTestServer(t)
	defer srv.Close()

	createReq := `{"share_name":"demo handoff","token_name":"agent-bot","scope":"project:demo","project_scope":"demo","share_expires_in":"1h","token_expires_in":"12h","max_claims":1}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/share-links", strings.NewReader(createReq))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Secret", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created types.CreateShareLinkResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	resolveResp, err := http.Get(srv.URL + "/v1/share-links/resolve?code=" + created.ShareCode)
	if err != nil {
		t.Fatal(err)
	}
	if resolveResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on resolve, got %d", resolveResp.StatusCode)
	}

	claimReq, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/share-links/claim?code="+created.ShareCode, nil)
	if err != nil {
		t.Fatal(err)
	}
	claimResp, err := http.DefaultClient.Do(claimReq)
	if err != nil {
		t.Fatal(err)
	}
	if claimResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on claim, got %d", claimResp.StatusCode)
	}
	var claimed types.ClaimShareLinkResponse
	if err := json.NewDecoder(claimResp.Body).Decode(&claimed); err != nil {
		t.Fatal(err)
	}
	if claimed.Token == "" || claimed.ProjectScope != "demo" {
		t.Fatalf("unexpected claim response: %+v", claimed)
	}
}
