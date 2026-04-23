package httpapi

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ckken/deployctl/internal/auth"
	"github.com/ckken/deployctl/internal/types"
)

type Server struct {
	store       *auth.Store
	adminSecret string
}

func New(store *auth.Store, adminSecret string) *Server {
	return &Server{store: store, adminSecret: adminSecret}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, types.ErrorResponse{Code: code, Message: message})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}

func (s *Server) validateAdmin(r *http.Request) bool {
	return s.adminSecret != "" && r.Header.Get("X-Admin-Secret") == s.adminSecret
}

func (s *Server) validateCaller(r *http.Request) (*types.TokenRecord, int, string, string) {
	record, err := s.store.ValidateToken(bearerToken(r))
	switch err {
	case nil:
		return record, 0, "", ""
	case auth.ErrMissingToken:
		return nil, http.StatusUnauthorized, "missing_token", "missing bearer token"
	case auth.ErrInvalidToken:
		return nil, http.StatusUnauthorized, "invalid_token", "token is invalid"
	case auth.ErrExpiredToken:
		return nil, http.StatusUnauthorized, "expired_token", "token has expired"
	case auth.ErrRevokedToken:
		return nil, http.StatusUnauthorized, "revoked_token", "token has been revoked"
	default:
		return nil, http.StatusInternalServerError, "server_error", err.Error()
	}
}

func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = forwarded
	} else if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Secret")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func staticHandler(webDir string) http.Handler {
	if webDir == "" {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.Dir(webDir))
	indexPath := filepath.Join(webDir, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(r.URL.Path, "/")
		if requestPath == "" {
			requestPath = "index.html"
		}

		candidate := filepath.Join(webDir, filepath.Clean(requestPath))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}
		http.NotFound(w, r)
	})
}

func readUploadedFile(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		return nil, nil, err
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, nil, err
	}
	return file, header, nil
}

func (s *Server) adminBootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.validateAdmin(r) {
		writeError(w, http.StatusUnauthorized, "invalid_admin_secret", "admin secret is invalid")
		return
	}
	grants, err := s.store.ListUploadGrants(publicBaseURL(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	uploads, err := s.store.ListUploads()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, types.AdminBootstrapResponse{
		ServerURL:   publicBaseURL(r),
		UploadLinks: grants,
		Uploads:     uploads,
	})
}

func (s *Server) APIHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, types.HealthResponse{Status: "ok"})
	})

	mux.HandleFunc("/v1/auth/whoami", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		record, status, code, message := s.validateCaller(r)
		if status != 0 {
			writeError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, types.WhoAmIResponse{
			TokenID:      record.TokenID,
			TokenName:    record.TokenName,
			Scope:        record.Scope,
			ProjectScope: record.ProjectScope,
			ExpiresAt:    record.ExpiresAt,
		})
	})

	mux.HandleFunc("/v1/admin/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		s.adminBootstrap(w, r)
	})

	mux.HandleFunc("/v1/admin/tokens", func(w http.ResponseWriter, r *http.Request) {
		if !s.validateAdmin(r) {
			writeError(w, http.StatusUnauthorized, "invalid_admin_secret", "admin secret is invalid")
			return
		}
		switch r.Method {
		case http.MethodGet:
			records, err := s.store.ListTokens()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "server_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, records)
		case http.MethodPost:
			var req types.CreateTokenRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid json")
				return
			}
			resp, err := s.store.CreateToken(context.Background(), req)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, resp)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})

	mux.HandleFunc("/v1/admin/tokens/", func(w http.ResponseWriter, r *http.Request) {
		if !s.validateAdmin(r) {
			writeError(w, http.StatusUnauthorized, "invalid_admin_secret", "admin secret is invalid")
			return
		}
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/revoke") {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		tokenID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/tokens/"), "/revoke")
		tokenID = strings.TrimSuffix(tokenID, "/")
		if tokenID == "" {
			writeError(w, http.StatusBadRequest, "invalid_token_id", "token id is required")
			return
		}
		resp, err := s.store.RevokeToken(tokenID)
		if err == auth.ErrInvalidToken {
			writeError(w, http.StatusNotFound, "not_found", "token not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/v1/admin/upload-links", func(w http.ResponseWriter, r *http.Request) {
		if !s.validateAdmin(r) {
			writeError(w, http.StatusUnauthorized, "invalid_admin_secret", "admin secret is invalid")
			return
		}
		switch r.Method {
		case http.MethodGet:
			records, err := s.store.ListUploadGrants(publicBaseURL(r))
			if err != nil {
				writeError(w, http.StatusInternalServerError, "server_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, records)
		case http.MethodPost:
			var req types.CreateUploadGrantRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid json")
				return
			}
			resp, err := s.store.CreateUploadGrant(req, publicBaseURL(r))
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, resp)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})

	mux.HandleFunc("/v1/admin/upload-links/", func(w http.ResponseWriter, r *http.Request) {
		if !s.validateAdmin(r) {
			writeError(w, http.StatusUnauthorized, "invalid_admin_secret", "admin secret is invalid")
			return
		}
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		grantID := strings.TrimPrefix(r.URL.Path, "/v1/admin/upload-links/")
		grantID = strings.TrimSuffix(grantID, "/")
		if grantID == "" {
			writeError(w, http.StatusBadRequest, "invalid_grant_id", "grant id is required")
			return
		}
		resp, err := s.store.DeleteUploadGrant(grantID)
		if err == auth.ErrInvalidUploadGrant {
			writeError(w, http.StatusNotFound, "not_found", "upload link not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	return withCORS(mux)
}

func (s *Server) uploadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathValue := strings.TrimPrefix(r.URL.Path, "/u/")
		pathValue = strings.Trim(pathValue, "/")
		if pathValue == "" {
			writeError(w, http.StatusNotFound, "not_found", "upload link not found")
			return
		}

		parts := strings.Split(pathValue, "/")
		code := parts[0]
		isResult := len(parts) == 2 && parts[1] == "result"
		if len(parts) > 2 || (len(parts) == 2 && !isResult) {
			writeError(w, http.StatusNotFound, "not_found", "upload link not found")
			return
		}

		switch {
		case isResult && r.Method == http.MethodGet:
			resp, err := s.store.LatestUploadForGrant(code)
			switch err {
			case nil:
				writeJSON(w, http.StatusOK, resp)
			case auth.ErrInvalidUploadGrant:
				writeError(w, http.StatusNotFound, "not_found", "upload result not found")
			default:
				writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			}
			return
		case !isResult && r.Method == http.MethodGet:
			resp, err := s.store.InspectUploadGrant(code, publicBaseURL(r))
			switch err {
			case nil:
				writeJSON(w, http.StatusOK, resp)
			case auth.ErrInvalidUploadGrant:
				writeError(w, http.StatusUnauthorized, "invalid_upload_link", "upload link is invalid")
			case auth.ErrExpiredUploadGrant:
				writeError(w, http.StatusUnauthorized, "expired_upload_link", "upload link has expired")
			case auth.ErrUploadLimitReached:
				writeError(w, http.StatusUnauthorized, "upload_limit_reached", "upload link has reached its limit")
			default:
				writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			}
			return
		case !isResult && r.Method == http.MethodPost:
			file, header, err := readUploadedFile(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_upload", "request must include multipart field file")
				return
			}
			defer file.Close()
			resp, err := s.store.SaveUploadedFile(code, header.Filename, header.Header.Get("Content-Type"), file, publicBaseURL(r))
			switch err {
			case nil:
				writeJSON(w, http.StatusCreated, resp)
			case auth.ErrInvalidUploadGrant:
				writeError(w, http.StatusUnauthorized, "invalid_upload_link", "upload link is invalid")
			case auth.ErrExpiredUploadGrant:
				writeError(w, http.StatusUnauthorized, "expired_upload_link", "upload link has expired")
			case auth.ErrUploadLimitReached:
				writeError(w, http.StatusUnauthorized, "upload_limit_reached", "upload link has reached its limit")
			default:
				writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			}
			return
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})
}

func fileHandler(root string) http.Handler {
	if root == "" {
		return http.NotFoundHandler()
	}
	return http.StripPrefix("/files/", http.FileServer(http.Dir(root)))
}

func (s *Server) Handler(webDir string) http.Handler {
	api := s.APIHandler()
	uploads := s.uploadHandler()
	files := fileHandler(s.store.UploadRootDir())
	static := staticHandler(webDir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/"):
			api.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/u/"):
			uploads.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/files/"):
			files.ServeHTTP(w, r)
		default:
			static.ServeHTTP(w, r)
		}
	})
}
