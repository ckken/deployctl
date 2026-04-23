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

type UploadGrantRecord struct {
	GrantID       string     `json:"grant_id"`
	GrantCode     string     `json:"grant_code"`
	GrantCodeHash string     `json:"grant_code_hash"`
	Folder        string     `json:"folder"`
	MaxFiles      int        `json:"max_files"`
	UploadCount   int        `json:"upload_count"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

type UploadGrantSummary struct {
	GrantID        string     `json:"grant_id"`
	UploadURL      string     `json:"upload_url"`
	Folder         string     `json:"folder"`
	MaxFiles       int        `json:"max_files"`
	UploadCount    int        `json:"upload_count"`
	RemainingFiles int        `json:"remaining_files"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type UploadRecord struct {
	UploadID          string    `json:"upload_id"`
	GrantID           string    `json:"grant_id"`
	OriginalFileName  string    `json:"original_file_name"`
	StoredFileName    string    `json:"stored_file_name"`
	SavedPath         string    `json:"saved_path"`
	FileURL           string    `json:"file_url"`
	ContentType       string    `json:"content_type,omitempty"`
	SizeBytes         int64     `json:"size_bytes"`
	UploadedAt        time.Time `json:"uploaded_at"`
	UploadGrantFolder string    `json:"upload_grant_folder"`
}

type UploadSummary struct {
	UploadID         string    `json:"upload_id"`
	GrantID          string    `json:"grant_id"`
	OriginalFileName string    `json:"original_file_name"`
	SavedPath        string    `json:"saved_path"`
	FileURL          string    `json:"file_url"`
	SizeBytes        int64     `json:"size_bytes"`
	UploadedAt       time.Time `json:"uploaded_at"`
	Folder           string    `json:"folder"`
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

type CreateUploadGrantRequest struct {
	Folder    string `json:"folder,omitempty"`
	ExpiresIn string `json:"expires_in,omitempty"`
	MaxFiles  int    `json:"max_files,omitempty"`
}

type CreateUploadGrantResponse struct {
	GrantID    string     `json:"grant_id"`
	GrantCode  string     `json:"grant_code"`
	UploadURL  string     `json:"upload_url"`
	Folder     string     `json:"folder"`
	MaxFiles   int        `json:"max_files"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	UploadPath string     `json:"upload_path"`
}

type DeleteUploadGrantResponse struct {
	GrantID string `json:"grant_id"`
}

type UploadGrantInfoResponse struct {
	UploadURL      string     `json:"upload_url"`
	Folder         string     `json:"folder"`
	MaxFiles       int        `json:"max_files"`
	UploadCount    int        `json:"upload_count"`
	RemainingFiles int        `json:"remaining_files"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Valid          bool       `json:"valid"`
}

type UploadFileResponse struct {
	UploadID         string    `json:"upload_id"`
	GrantID          string    `json:"grant_id"`
	OriginalFileName string    `json:"original_file_name"`
	SavedPath        string    `json:"saved_path"`
	FileURL          string    `json:"file_url"`
	SizeBytes        int64     `json:"size_bytes"`
	UploadedAt       time.Time `json:"uploaded_at"`
	Folder           string    `json:"folder"`
}

type AdminBootstrapResponse struct {
	ServerURL   string               `json:"server_url"`
	UploadLinks []UploadGrantSummary `json:"upload_links"`
	Uploads     []UploadSummary      `json:"uploads"`
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
