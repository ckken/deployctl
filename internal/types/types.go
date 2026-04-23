package types

import "time"

type TokenRecord struct {
	TokenID      string     `json:"token_id"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	TokenHash    string     `json:"token_hash"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type TokenSummary struct {
	TokenID      string     `json:"token_id"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type ShareLinkRecord struct {
	ShareID        string     `json:"share_id"`
	ShareName      string     `json:"share_name"`
	ShareCodeHash  string     `json:"share_code_hash"`
	Scope          string     `json:"scope"`
	ProjectScope   string     `json:"project_scope,omitempty"`
	TokenName      string     `json:"token_name"`
	TokenExpiresIn string     `json:"token_expires_in,omitempty"`
	MaxClaims      int        `json:"max_claims"`
	ClaimCount     int        `json:"claim_count"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
}

type ShareLinkSummary struct {
	ShareID        string     `json:"share_id"`
	ShareName      string     `json:"share_name"`
	Scope          string     `json:"scope"`
	ProjectScope   string     `json:"project_scope,omitempty"`
	TokenName      string     `json:"token_name"`
	TokenExpiresIn string     `json:"token_expires_in,omitempty"`
	MaxClaims      int        `json:"max_claims"`
	ClaimCount     int        `json:"claim_count"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WhoAmIResponse struct {
	TokenID      string     `json:"token_id"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type CreateTokenRequest struct {
	Name         string `json:"name"`
	Scope        string `json:"scope"`
	ProjectScope string `json:"project_scope,omitempty"`
	ExpiresIn    string `json:"expires_in,omitempty"`
}

type CreateTokenResponse struct {
	TokenID      string     `json:"token_id"`
	Token        string     `json:"token"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type RevokeTokenResponse struct {
	TokenID   string     `json:"token_id"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type CreateShareLinkRequest struct {
	ShareName      string `json:"share_name"`
	TokenName      string `json:"token_name"`
	Scope          string `json:"scope"`
	ProjectScope   string `json:"project_scope,omitempty"`
	ShareExpiresIn string `json:"share_expires_in,omitempty"`
	TokenExpiresIn string `json:"token_expires_in,omitempty"`
	MaxClaims      int    `json:"max_claims,omitempty"`
}

type CreateShareLinkResponse struct {
	ShareID        string     `json:"share_id"`
	ShareCode      string     `json:"share_code"`
	ShareName      string     `json:"share_name"`
	TokenName      string     `json:"token_name"`
	Scope          string     `json:"scope"`
	ProjectScope   string     `json:"project_scope,omitempty"`
	TokenExpiresIn string     `json:"token_expires_in,omitempty"`
	MaxClaims      int        `json:"max_claims"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	ResolveURL     string     `json:"resolve_url"`
	ClaimURL       string     `json:"claim_url"`
}

type RevokeShareLinkResponse struct {
	ShareID   string     `json:"share_id"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type ResolveShareLinkResponse struct {
	ShareID      string     `json:"share_id"`
	ShareName    string     `json:"share_name"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	ServerURL    string     `json:"server_url"`
	ClaimURL     string     `json:"claim_url"`
	Valid        bool       `json:"valid"`
	MaxClaims    int        `json:"max_claims"`
	ClaimCount   int        `json:"claim_count"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type ClaimShareLinkRequest struct {
	ShareID string `json:"share_id"`
	Code    string `json:"code"`
}

type ClaimShareLinkResponse struct {
	ServerURL    string     `json:"server_url"`
	Token        string     `json:"token"`
	TokenID      string     `json:"token_id"`
	TokenName    string     `json:"token_name"`
	Scope        string     `json:"scope"`
	ProjectScope string     `json:"project_scope,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

type AdminBootstrapResponse struct {
	ServerURL  string             `json:"server_url"`
	Tokens     []TokenSummary     `json:"tokens"`
	ShareLinks []ShareLinkSummary `json:"share_links"`
}

type DoctorResponse struct {
	Server    string      `json:"server"`
	Reachable bool        `json:"reachable"`
	Token     DoctorToken `json:"token"`
	Auth      DoctorAuth  `json:"auth"`
}

type DoctorToken struct {
	Present bool   `json:"present"`
	Source  string `json:"source"`
}

type DoctorAuth struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
