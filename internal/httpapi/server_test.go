package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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

func TestUploadLinkHTTPFlow(t *testing.T) {
	_, srv := newTestServer(t)
	defer srv.Close()

	createReq := `{"folder":"releases/demo","expires_in":"1h","max_files":1}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/upload-links", strings.NewReader(createReq))
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
	var created types.CreateUploadGrantResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	infoResp, err := http.Get(created.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	if infoResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on inspect, got %d", infoResp.StatusCode)
	}

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	part, err := writer.CreateFormFile("file", "bundle.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	uploadReq, err := http.NewRequest(http.MethodPost, created.UploadURL, &uploadBody)
	if err != nil {
		t.Fatal(err)
	}
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		t.Fatal(err)
	}
	if uploadResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on upload, got %d", uploadResp.StatusCode)
	}
	var uploaded types.UploadFileResponse
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.FileURL == "" || uploaded.SavedPath == "" {
		t.Fatalf("unexpected upload response: %+v", uploaded)
	}

	resultResp, err := http.Get(created.UploadURL + "/result")
	if err != nil {
		t.Fatal(err)
	}
	if resultResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on result, got %d", resultResp.StatusCode)
	}
}

func TestDeletedUploadLinkDisappearsFromBootstrap(t *testing.T) {
	_, srv := newTestServer(t)
	defer srv.Close()

	createReq := `{"folder":"handoff/demo","max_files":1}`
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/admin/upload-links", strings.NewReader(createReq))
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
	var created types.CreateUploadGrantResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}

	deleteReq, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/admin/upload-links/"+created.GrantID, nil)
	if err != nil {
		t.Fatal(err)
	}
	deleteReq.Header.Set("X-Admin-Secret", "secret")
	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatal(err)
	}
	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", deleteResp.StatusCode)
	}

	bootstrapReq, err := http.NewRequest(http.MethodGet, srv.URL+"/v1/admin/bootstrap", nil)
	if err != nil {
		t.Fatal(err)
	}
	bootstrapReq.Header.Set("X-Admin-Secret", "secret")
	bootstrapResp, err := http.DefaultClient.Do(bootstrapReq)
	if err != nil {
		t.Fatal(err)
	}
	if bootstrapResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on bootstrap, got %d", bootstrapResp.StatusCode)
	}
	var bootstrap types.AdminBootstrapResponse
	if err := json.NewDecoder(bootstrapResp.Body).Decode(&bootstrap); err != nil {
		t.Fatal(err)
	}
	if len(bootstrap.UploadLinks) != 0 {
		t.Fatalf("expected deleted upload link to disappear, got %d items", len(bootstrap.UploadLinks))
	}

	infoResp, err := http.Get(created.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	if infoResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after delete, got %d", infoResp.StatusCode)
	}
}
