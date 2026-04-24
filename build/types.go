package build

import (
	"encoding/json"
	"time"
)

// DirectBuildRequest is the request body for POST /build.
type DirectBuildRequest struct {
	Project    string `json:"project"`
	Image      string `json:"image"`
	Tag        string `json:"tag"`
	Dockerfile string `json:"dockerfile"`
}

// DirectBuildResponse is returned by POST /build.
type DirectBuildResponse struct {
	TemplateID    string `json:"templateID"`
	BuildID       string `json:"buildID"`
	ImageFullName string `json:"imageFullName"`
}

// TemplateCreateRequest is the request body for POST /api/v1/templates.
type TemplateCreateRequest struct {
	Name           string            `json:"name,omitempty"`
	Visibility     string            `json:"visibility,omitempty"`
	BaseTemplateID string            `json:"baseTemplateID,omitempty"`
	Dockerfile     string            `json:"dockerfile,omitempty"`
	Image          string            `json:"image,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	CPUCount       *int32            `json:"cpuCount,omitempty"`
	MemoryMB       *int32            `json:"memoryMB,omitempty"`
	DiskSizeMB     *int32            `json:"diskSizeMB,omitempty"`
	TTLSeconds     *int32            `json:"ttlSeconds,omitempty"`
	Port           *int32            `json:"port,omitempty"`
	StartCmd       string            `json:"startCmd,omitempty"`
	ReadyCmd       string            `json:"readyCmd,omitempty"`
}

// TemplateUpdateRequest is the request body for PATCH /api/v1/templates/:id.
type TemplateUpdateRequest struct {
	Name           *string           `json:"name,omitempty"`
	Visibility     *string           `json:"visibility,omitempty"`
	BaseTemplateID *string           `json:"baseTemplateID,omitempty"`
	Dockerfile     *string           `json:"dockerfile,omitempty"`
	Image          *string           `json:"image,omitempty"`
	Envs           map[string]string `json:"envs,omitempty"`
	CPUCount       *int32            `json:"cpuCount,omitempty"`
	MemoryMB       *int32            `json:"memoryMB,omitempty"`
	DiskSizeMB     *int32            `json:"diskSizeMB,omitempty"`
	TTLSeconds     *int32            `json:"ttlSeconds,omitempty"`
	Port           *int32            `json:"port,omitempty"`
	StartCmd       *string           `json:"startCmd,omitempty"`
	ReadyCmd       *string           `json:"readyCmd,omitempty"`
}

// TemplateCreateResponse is the minimal create response.
type TemplateCreateResponse struct {
	TemplateID string   `json:"templateID"`
	BuildID    string   `json:"buildID"`
	Public     bool     `json:"public"`
	Names      []string `json:"names"`
	Tags       []string `json:"tags"`
	Aliases    []string `json:"aliases"`
}

// TemplateUpdateResponse is the minimal update response.
type TemplateUpdateResponse struct {
	Names []string `json:"names"`
}

// ListTemplatesParams configures GET /api/v1/templates.
type ListTemplatesParams struct {
	Visibility string
	TeamID     string
	Limit      int
	Offset     int
}

// GetTemplateParams configures GET /api/v1/templates/:id.
type GetTemplateParams struct {
	Limit     int
	NextToken string
}

// TemplateAliasResponse is the minimal alias lookup response.
type TemplateAliasResponse struct {
	TemplateID string `json:"templateID"`
	Public     bool   `json:"public"`
}

// TemplateUser describes the creator of a template.
type TemplateUser struct {
	ID    string `json:"id"`
	Email string `json:"email,omitempty"`
}

// ListedTemplate is one item returned by GET /api/v1/templates.
type ListedTemplate struct {
	TemplateID  string        `json:"templateID"`
	BuildID     string        `json:"buildID"`
	BuildStatus string        `json:"buildStatus"`
	Public      bool          `json:"public"`
	Names       []string      `json:"names"`
	Aliases     []string      `json:"aliases"`
	CreatedBy   *TemplateUser `json:"createdBy"`
}

// TemplateResponse describes a template and its build history.
type TemplateResponse struct {
	TemplateID     string          `json:"templateID"`
	BuildID        string          `json:"buildID"`
	BuildStatus    string          `json:"buildStatus"`
	Public         bool            `json:"public"`
	Names          []string        `json:"names"`
	Aliases        []string        `json:"aliases"`
	Tags           []string        `json:"tags"`
	Name           string          `json:"name"`
	Visibility     string          `json:"visibility"`
	BaseTemplateID string          `json:"baseTemplateID,omitempty"`
	Image          string          `json:"image"`
	ImageSource    string          `json:"imageSource"`
	EnvdVersion    string          `json:"envdVersion"`
	CPUCount       int32           `json:"cpuCount"`
	MemoryMB       int32           `json:"memoryMB"`
	DiskSizeMB     int32           `json:"diskSizeMB"`
	CreatedBy      *TemplateUser   `json:"createdBy"`
	CreatedByID    string          `json:"createdByID"`
	ProjectID      string          `json:"projectID"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	LastSpawnedAt  *time.Time      `json:"lastSpawnedAt"`
	SpawnCount     int             `json:"spawnCount"`
	BuildCount     int             `json:"buildCount"`
	StorageType    string          `json:"storageType"`
	TTLSeconds     int32           `json:"ttlSeconds"`
	Port           int32           `json:"port"`
	StartCmd       string          `json:"startCmd"`
	ReadyCmd       string          `json:"readyCmd"`
	Builds         []BuildResponse `json:"builds,omitempty"`
	NextToken      string          `json:"nextToken,omitempty"`
}

// BuildStep is one build step in an E2B-compatible rebuild request.
type BuildStep struct {
	Type      string   `json:"type,omitempty"`
	FilesHash string   `json:"filesHash,omitempty"`
	Args      []string `json:"args,omitempty"`
	Force     *bool    `json:"force,omitempty"`
}

// BuildRequest is the request body for POST /api/v1/templates/:id/builds.
type BuildRequest struct {
	BuildID           string      `json:"buildID,omitempty"`
	FromTemplate      string      `json:"fromTemplate,omitempty"`
	FromImage         string      `json:"fromImage,omitempty"`
	FromImageRegistry string      `json:"fromImageRegistry,omitempty"`
	Force             *bool       `json:"force,omitempty"`
	Steps             []BuildStep `json:"steps,omitempty"`
	FilesHash         string      `json:"filesHash,omitempty"`
	StartCmd          string      `json:"startCmd,omitempty"`
	ReadyCmd          string      `json:"readyCmd,omitempty"`
}

// BuildResponse describes one build record.
type BuildResponse struct {
	BuildID      string     `json:"buildID"`
	TemplateID   string     `json:"templateID"`
	Status       string     `json:"status"`
	Image        string     `json:"image"`
	ErrorMessage string     `json:"errorMessage"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

// BuildTriggerResponse captures both the native build response and the E2B empty object response.
type BuildTriggerResponse struct {
	BuildResponse
	Empty bool `json:"-"`
}

// FilePresenceResponse is returned by GET /api/v1/templates/:id/files/:hash.
type FilePresenceResponse struct {
	Present bool   `json:"present"`
	URL     string `json:"url,omitempty"`
}

// RollbackRequest is the request body for POST /api/v1/templates/:id/rollback.
type RollbackRequest struct {
	BuildID string `json:"buildID"`
}

// BuildHistoryResponse is returned by GET /api/v1/templates/:id/builds.
type BuildHistoryResponse struct {
	Builds []BuildResponse `json:"builds"`
	Total  int             `json:"total"`
}

// BuildStatusParams configures GET /api/v1/templates/:id/builds/:buildID/status.
type BuildStatusParams struct {
	LogsOffset *int
	Limit      *int
	Level      string
}

// BuildStatusResponse is returned by GET /status.
type BuildStatusResponse struct {
	BuildID    string          `json:"buildID"`
	TemplateID string          `json:"templateID"`
	Status     string          `json:"status"`
	Logs       []string        `json:"logs"`
	LogEntries []BuildLogEntry `json:"logEntries"`
	Reason     any             `json:"reason"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

// BuildLogsParams configures GET /logs.
type BuildLogsParams struct {
	Cursor    *int64
	Limit     *int
	Direction string
	Level     string
	Source    string
}

// BuildLogEntry is one structured build log line.
type BuildLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Step      string    `json:"step"`
	Message   string    `json:"message"`
}

// BuildLogsResponse wraps structured build logs.
type BuildLogsResponse struct {
	Logs []BuildLogEntry `json:"logs"`
}

func (r *BuildTriggerResponse) normalize() {
	if r == nil {
		return
	}
	r.Empty = r.BuildID == "" &&
		r.TemplateID == "" &&
		r.Status == "" &&
		r.Image == "" &&
		r.ErrorMessage == "" &&
		r.CreatedAt.IsZero() &&
		r.UpdatedAt.IsZero() &&
		r.FinishedAt == nil
}

func (r *BuildStatusResponse) UnmarshalJSON(data []byte) error {
	type rawBuildStatusResponse struct {
		BuildID    string          `json:"buildID"`
		TemplateID string          `json:"templateID"`
		Status     string          `json:"status"`
		Logs       json.RawMessage `json:"logs"`
		LogEntries json.RawMessage `json:"logEntries"`
		Reason     any             `json:"reason"`
		CreatedAt  time.Time       `json:"createdAt"`
		UpdatedAt  time.Time       `json:"updatedAt"`
	}

	var raw rawBuildStatusResponse
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*r = BuildStatusResponse{
		BuildID:    raw.BuildID,
		TemplateID: raw.TemplateID,
		Status:     raw.Status,
		Reason:     raw.Reason,
		CreatedAt:  raw.CreatedAt,
		UpdatedAt:  raw.UpdatedAt,
	}

	if len(raw.Logs) > 0 && string(raw.Logs) != "null" {
		if err := json.Unmarshal(raw.Logs, &r.Logs); err != nil {
			var legacy []BuildLogEntry
			if err := json.Unmarshal(raw.Logs, &legacy); err == nil {
				r.LogEntries = legacy
			} else {
				return err
			}
		}
	}
	if len(raw.LogEntries) > 0 && string(raw.LogEntries) != "null" {
		if err := json.Unmarshal(raw.LogEntries, &r.LogEntries); err != nil {
			return err
		}
	}
	if len(r.LogEntries) == 0 && len(raw.Logs) > 0 && string(raw.Logs) != "null" {
		var legacy []BuildLogEntry
		if err := json.Unmarshal(raw.Logs, &legacy); err == nil {
			r.LogEntries = legacy
		}
	}
	return nil
}
