package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
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

func (s *Server) Handler() http.Handler {
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

		prefix := "/v1/admin/tokens/"
		trimmed := strings.TrimPrefix(r.URL.Path, prefix)
		tokenID := strings.TrimSuffix(trimmed, "/revoke")
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

	return mux
}
