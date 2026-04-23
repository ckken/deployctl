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
