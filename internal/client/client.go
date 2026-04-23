package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ckken/deployctl/internal/types"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func New(baseURL string, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) doJSON(ctx context.Context, method string, path string, body any, out any, extraHeaders map[string]string) error {
	var reqBody *bytes.Reader
	if body == nil {
		reqBody = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	for key, value := range extraHeaders {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Health(ctx context.Context) (*types.HealthResponse, error) {
	var out types.HealthResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/health", nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) WhoAmI(ctx context.Context) (*types.WhoAmIResponse, error) {
	var out types.WhoAmIResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/auth/whoami", nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CreateUploadLink(ctx context.Context, adminKey string, req types.CreateUploadGrantRequest) (*types.CreateUploadGrantResponse, error) {
	var out types.CreateUploadGrantResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/admin/upload-links", req, &out, map[string]string{"X-Admin-Secret": adminKey}); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) ListUploadLinks(ctx context.Context, adminKey string) ([]types.UploadGrantSummary, error) {
	var out []types.UploadGrantSummary
	if err := c.doJSON(ctx, http.MethodGet, "/v1/admin/upload-links", nil, &out, map[string]string{"X-Admin-Secret": adminKey}); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) DeleteUploadLink(ctx context.Context, adminKey string, grantID string) (*types.DeleteUploadGrantResponse, error) {
	var out types.DeleteUploadGrantResponse
	if err := c.doJSON(ctx, http.MethodDelete, "/v1/admin/upload-links/"+grantID, nil, &out, map[string]string{"X-Admin-Secret": adminKey}); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UploadInfoByURL(ctx context.Context, uploadURL string) (*types.UploadGrantInfoResponse, error) {
	var out types.UploadGrantInfoResponse
	if err := c.doAbsoluteJSON(ctx, http.MethodGet, uploadURL, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) LatestUploadByURL(ctx context.Context, uploadURL string) (*types.UploadFileResponse, error) {
	fullURL := strings.TrimRight(uploadURL, "/") + "/result"
	var out types.UploadFileResponse
	if err := c.doAbsoluteJSON(ctx, http.MethodGet, fullURL, nil, &out, nil); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doAbsoluteJSON(ctx context.Context, method string, fullURL string, body any, out any, extraHeaders map[string]string) error {
	var reqBody *bytes.Reader
	if body == nil {
		reqBody = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range extraHeaders {
		if value != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) UploadFileByURL(ctx context.Context, uploadURL string, filePath string) (*types.UploadFileResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr types.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
			return nil, fmt.Errorf("request failed with status %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s: %s", apiErr.Code, apiErr.Message)
	}

	var out types.UploadFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func ResolveBaseURL(uploadURL string) string {
	parsed, err := url.Parse(uploadURL)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
