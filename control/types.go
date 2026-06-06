package control

import "time"

// ShutdownResponse is returned by POST /shutdown.
type ShutdownResponse struct {
	Message string `json:"message"`
}

// VolumeMount represents one extra sandbox volume mount.
type VolumeMount struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// NewSandboxRequest is the request body for creating a sandbox.
type NewSandboxRequest struct {
	TemplateID          string                `json:"templateID,omitempty"`
	Timeout             *int64                `json:"timeout,omitempty"`
	AutoPause           *bool                 `json:"autoPause,omitempty"`
	AutoResume          *bool                 `json:"autoResume,omitempty"`
	AllowInternetAccess *bool                 `json:"allowInternetAccess,omitempty"`
	Metadata            map[string]string     `json:"metadata,omitempty"`
	EnvVars             map[string]string     `json:"envVars,omitempty"`
	WaitReady           *bool                 `json:"waitReady,omitempty"`
	Network             *SandboxNetworkPolicy `json:"network,omitempty"`
	VolumeMounts        []VolumeMount         `json:"volumeMounts,omitempty"`
}

type SandboxLifecycle struct {
	OnTimeout  string `json:"onTimeout"`
	AutoResume bool   `json:"autoResume,omitempty"`
}

// SandboxNetworkPolicy controls per-sandbox network access.
//
// AllowInternetAccess=false enables egress isolation. AllowOut and DenyOut
// accept IPv4 CIDR ranges or single IPv4 addresses; single IPs are normalized
// by the control plane to /32.
type SandboxNetworkPolicy struct {
	AllowPublicTraffic  *bool    `json:"allowPublicTraffic,omitempty"`
	AllowInternetAccess *bool    `json:"allowInternetAccess,omitempty"`
	AllowOut            []string `json:"allowOut,omitempty"`
	DenyOut             []string `json:"denyOut,omitempty"`
}

// SandboxLifecycleEvent is a lifecycle event returned by /api/v1/events.
type SandboxLifecycleEvent struct {
	Version            string         `json:"version"`
	ID                 string         `json:"id"`
	Type               string         `json:"type"`
	EventData          map[string]any `json:"eventData,omitempty"`
	SandboxBuildID     string         `json:"sandboxBuildId,omitempty"`
	SandboxExecutionID string         `json:"sandboxExecutionId,omitempty"`
	SandboxID          string         `json:"sandboxId"`
	SandboxTeamID      string         `json:"sandboxTeamId"`
	SandboxTemplateID  string         `json:"sandboxTemplateId,omitempty"`
	Timestamp          time.Time      `json:"timestamp"`
}

// ListSandboxEventsParams configures lifecycle event queries.
type ListSandboxEventsParams struct {
	Offset   int
	Limit    int
	OrderAsc *bool
	Types    []string
}

// WebhookRetryPolicy configures webhook delivery retries and dead-letter behavior.
type WebhookRetryPolicy struct {
	MaxAttempts       int   `json:"maxAttempts"`
	DelaySeconds      []int `json:"delaySeconds,omitempty"`
	DeadLetterEnabled bool  `json:"deadLetterEnabled,omitempty"`
}

type LifecycleWebhook struct {
	ID            string              `json:"id"`
	TeamID        string              `json:"teamId"`
	Name          string              `json:"name"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     *time.Time          `json:"updatedAt,omitempty"`
	Enabled       bool                `json:"enabled"`
	URL           string              `json:"url"`
	Events        []string            `json:"events"`
	RetryPolicy   *WebhookRetryPolicy `json:"retryPolicy,omitempty"`
	DeadLetterURL string              `json:"deadLetterUrl,omitempty"`
}

type LifecycleWebhookCreateRequest struct {
	Name            string              `json:"name"`
	URL             string              `json:"url"`
	Enabled         *bool               `json:"enabled,omitempty"`
	Events          []string            `json:"events"`
	SignatureSecret string              `json:"signatureSecret"`
	RetryPolicy     *WebhookRetryPolicy `json:"retryPolicy,omitempty"`
	DeadLetterURL   string              `json:"deadLetterUrl,omitempty"`
}

type LifecycleWebhookUpdateRequest struct {
	Name            *string             `json:"name,omitempty"`
	URL             *string             `json:"url,omitempty"`
	Enabled         *bool               `json:"enabled,omitempty"`
	Events          []string            `json:"events,omitempty"`
	SignatureSecret *string             `json:"signatureSecret,omitempty"`
	RetryPolicy     *WebhookRetryPolicy `json:"retryPolicy,omitempty"`
	DeadLetterURL   *string             `json:"deadLetterUrl,omitempty"`
}

type DeleteWebhookResponse struct {
	Deleted bool `json:"deleted"`
}

type LifecycleWebhookDelivery struct {
	ID              string     `json:"id"`
	EventID         string     `json:"eventId"`
	WebhookID       string     `json:"webhookId"`
	NamespaceID     string     `json:"namespaceId"`
	TeamID          string     `json:"teamId"`
	URL             string     `json:"url"`
	Status          string     `json:"status"`
	HTTPStatus      int        `json:"httpStatus,omitempty"`
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"maxAttempts,omitempty"`
	Error           string     `json:"error,omitempty"`
	DeadLetterURL   string     `json:"deadLetterUrl,omitempty"`
	DeadLetterError string     `json:"deadLetterError,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	LastAttemptAt   *time.Time `json:"lastAttemptAt,omitempty"`
	NextAttemptAt   *time.Time `json:"nextAttemptAt,omitempty"`
	DeliveredAt     *time.Time `json:"deliveredAt,omitempty"`
}

// ListWebhookDeliveriesParams configures webhook delivery queries.
type ListWebhookDeliveriesParams struct {
	Offset    int
	Limit     int
	OrderAsc  *bool
	WebhookID string
	EventID   string
	Status    string
}

type Volume struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
}

type VolumeAndToken struct {
	VolumeID string `json:"volumeID"`
	Name     string `json:"name"`
	Token    string `json:"token"`
}

type NewVolumeRequest struct {
	Name string `json:"name"`
}

type Team struct {
	TeamID    string `json:"teamID"`
	Name      string `json:"name"`
	APIKey    string `json:"apiKey"`
	IsDefault bool   `json:"isDefault"`
}

type TeamMetric struct {
	Timestamp           time.Time `json:"timestamp"`
	TimestampUnix       int64     `json:"timestampUnix"`
	ConcurrentSandboxes int32     `json:"concurrentSandboxes"`
	SandboxStartRate    float64   `json:"sandboxStartRate"`
}

type MaxTeamMetric struct {
	Timestamp     time.Time `json:"timestamp"`
	TimestampUnix int64     `json:"timestampUnix"`
	Value         float64   `json:"value"`
}

type TeamMetricsParams struct {
	Start int64
	End   int64
}

type TeamMetricsMaxParams struct {
	Metric string
	Start  int64
	End    int64
}

// SandboxTimelineEvent is a public lifecycle event for user-facing diagnostics.
type SandboxTimelineEvent struct {
	Phase     string    `json:"phase"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message,omitempty"`
}

// SandboxDiagnostic explains current sandbox state using public product terms.
type SandboxDiagnostic struct {
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation,omitempty"`
}

// Sandbox is returned by create and connect endpoints.
type Sandbox struct {
	TemplateID      string                 `json:"templateID"`
	SandboxID       string                 `json:"sandboxID"`
	Alias           string                 `json:"alias,omitempty"`
	ClientID        string                 `json:"clientID"`
	EnvdAccessToken *string                `json:"envdAccessToken"`
	EnvdURL         *string                `json:"envdUrl"`
	Status          string                 `json:"status"`
	State           string                 `json:"state,omitempty"`
	StartedAt       time.Time              `json:"startedAt"`
	ActivatedAt     *time.Time             `json:"activatedAt,omitempty"`
	EndAt           time.Time              `json:"endAt"`
	Timeline        []SandboxTimelineEvent `json:"timeline,omitempty"`
	Diagnostic      *SandboxDiagnostic     `json:"diagnostic,omitempty"`
	Network         *SandboxNetworkPolicy  `json:"network,omitempty"`
}

// SandboxDetail is returned by GET /api/v1/sandboxes/:sandboxID.
type SandboxDetail struct {
	TemplateID      string                 `json:"templateID"`
	Alias           string                 `json:"alias,omitempty"`
	SandboxID       string                 `json:"sandboxID"`
	ClientID        string                 `json:"clientID"`
	StartedAt       time.Time              `json:"startedAt"`
	EndAt           time.Time              `json:"endAt"`
	EnvdAccessToken *string                `json:"envdAccessToken"`
	EnvdURL         *string                `json:"envdUrl"`
	CPUCount        int32                  `json:"cpuCount"`
	MemoryMB        int32                  `json:"memoryMB"`
	DiskSizeMB      int32                  `json:"diskSizeMB"`
	Metadata        map[string]string      `json:"metadata,omitempty"`
	Status          string                 `json:"status"`
	State           string                 `json:"state,omitempty"`
	Lifecycle       SandboxLifecycle       `json:"lifecycle"`
	VolumeMounts    []VolumeMount          `json:"volumeMounts,omitempty"`
	ActivatedAt     *time.Time             `json:"activatedAt,omitempty"`
	Timeline        []SandboxTimelineEvent `json:"timeline,omitempty"`
	Diagnostic      *SandboxDiagnostic     `json:"diagnostic,omitempty"`
	Network         *SandboxNetworkPolicy  `json:"network,omitempty"`
}

// ListedSandbox is returned by the list endpoint.
type ListedSandbox struct {
	TemplateID   string                 `json:"templateID"`
	Alias        string                 `json:"alias,omitempty"`
	SandboxID    string                 `json:"sandboxID"`
	ClientID     string                 `json:"clientID"`
	StartedAt    time.Time              `json:"startedAt"`
	EndAt        time.Time              `json:"endAt"`
	CPUCount     int32                  `json:"cpuCount"`
	MemoryMB     int32                  `json:"memoryMB"`
	DiskSizeMB   int32                  `json:"diskSizeMB"`
	Metadata     map[string]string      `json:"metadata,omitempty"`
	Status       string                 `json:"status"`
	State        string                 `json:"state,omitempty"`
	VolumeMounts []VolumeMount          `json:"volumeMounts,omitempty"`
	ActivatedAt  *time.Time             `json:"activatedAt,omitempty"`
	Timeline     []SandboxTimelineEvent `json:"timeline,omitempty"`
	Diagnostic   *SandboxDiagnostic     `json:"diagnostic,omitempty"`
	Network      *SandboxNetworkPolicy  `json:"network,omitempty"`
}

// ListSandboxesParams configures GET /api/v1/sandboxes.
type ListSandboxesParams struct {
	Metadata  map[string]string
	State     []string
	Limit     int
	NextToken string
}

// SandboxesPage includes the array response plus pagination headers.
type SandboxesPage struct {
	Items     []ListedSandbox
	NextToken string
	HasNext   bool
}

// SandboxMetricSnapshot is one sandbox control-plane metrics snapshot.
type SandboxMetricSnapshot struct {
	SandboxID   string    `json:"sandboxID"`
	CollectedAt time.Time `json:"collectedAt"`
	Error       string    `json:"error,omitempty"`

	Load1         *float64 `json:"load1,omitempty"`
	Load5         *float64 `json:"load5,omitempty"`
	Load15        *float64 `json:"load15,omitempty"`
	CPUUserRate   *float64 `json:"cpuUserRate,omitempty"`
	CPUSystemRate *float64 `json:"cpuSystemRate,omitempty"`
	CPUIOWaitRate *float64 `json:"cpuIOWaitRate,omitempty"`
	CPUStealRate  *float64 `json:"cpuStealRate,omitempty"`

	MemoryAvailableBytes *float64 `json:"memoryAvailableBytes,omitempty"`
	MemoryUsagePercent   *float64 `json:"memoryUsagePercent,omitempty"`
	SwapTotalBytes       *float64 `json:"swapTotalBytes,omitempty"`
	SwapFreeBytes        *float64 `json:"swapFreeBytes,omitempty"`
	SwapCachedBytes      *float64 `json:"swapCachedBytes,omitempty"`

	DiskReadOpsPerSecond    *float64 `json:"diskReadOpsPerSecond,omitempty"`
	DiskWriteOpsPerSecond   *float64 `json:"diskWriteOpsPerSecond,omitempty"`
	DiskReadBytesPerSecond  *float64 `json:"diskReadBytesPerSecond,omitempty"`
	DiskWriteBytesPerSecond *float64 `json:"diskWriteBytesPerSecond,omitempty"`

	NetworkRecvBytesPerSecond   *float64 `json:"networkRecvBytesPerSecond,omitempty"`
	NetworkSentBytesPerSecond   *float64 `json:"networkSentBytesPerSecond,omitempty"`
	NetworkRecvPacketsPerSecond *float64 `json:"networkRecvPacketsPerSecond,omitempty"`
	NetworkSentPacketsPerSecond *float64 `json:"networkSentPacketsPerSecond,omitempty"`
	NetworkRecvErrorsPerSecond  *float64 `json:"networkRecvErrorsPerSecond,omitempty"`
	NetworkSentErrorsPerSecond  *float64 `json:"networkSentErrorsPerSecond,omitempty"`
	NetworkRecvDropsPerSecond   *float64 `json:"networkRecvDropsPerSecond,omitempty"`
	NetworkSentDropsPerSecond   *float64 `json:"networkSentDropsPerSecond,omitempty"`
	TaskCurrent                 *float64 `json:"taskCurrent,omitempty"`
	TaskMax                     *float64 `json:"taskMax,omitempty"`
}

// SandboxMetricsParams configures GET /api/v1/sandboxes/metrics.
type SandboxMetricsParams struct {
	SandboxIDs []string
	Limit      int
}

// SandboxMetricsResponse is returned by batch sandbox metrics collection.
type SandboxMetricsResponse struct {
	CollectedAt time.Time                        `json:"collectedAt"`
	Items       []SandboxMetricSnapshot          `json:"items"`
	Sandboxes   map[string]SandboxMetricSnapshot `json:"sandboxes"`
}

type UsageLimitValue struct {
	Limit     int     `json:"limit"`
	Used      int     `json:"used"`
	Remaining int     `json:"remaining"`
	ResetAt   *string `json:"resetAt,omitempty"`
	Enforced  bool    `json:"enforced"`
}

type UsageLimitScope struct {
	ID     string                     `json:"id,omitempty"`
	Usage  map[string]int             `json:"usage,omitempty"`
	Limits map[string]UsageLimitValue `json:"limits,omitempty"`
}

type RuntimeLimitInfo struct {
	MaxRuntimeSeconds int32 `json:"maxRuntimeSeconds,omitempty"`
}

type SandboxUsageLimits struct {
	Resource  string            `json:"resource"`
	Unlimited bool              `json:"unlimited,omitempty"`
	User      *UsageLimitScope  `json:"user,omitempty"`
	Project   *UsageLimitScope  `json:"project,omitempty"`
	Runtime   *RuntimeLimitInfo `json:"runtime,omitempty"`
}

type TemplateResourceLimitInfo struct {
	MaxTemplateCPU       int32 `json:"maxTemplateCPU,omitempty"`
	MaxTemplateMemoryMB  int32 `json:"maxTemplateMemoryMB,omitempty"`
	MaxTemplateStorageGB int32 `json:"maxTemplateStorageGB,omitempty"`
}

type TemplateUsageLimits struct {
	Resource  string                     `json:"resource"`
	Unlimited bool                       `json:"unlimited,omitempty"`
	User      *UsageLimitScope           `json:"user,omitempty"`
	Project   *UsageLimitScope           `json:"project,omitempty"`
	Resources *TemplateResourceLimitInfo `json:"resources,omitempty"`
}

type ObservabilitySignal struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ObservabilityCheck struct {
	Status        string `json:"status"`
	Scope         string `json:"scope"`
	Resource      string `json:"resource"`
	Metric        string `json:"metric"`
	Used          int    `json:"used"`
	Limit         int    `json:"limit"`
	Remaining     int    `json:"remaining"`
	Message       string `json:"message"`
	UsageEndpoint string `json:"usageEndpoint"`
}

type ObservabilityAction struct {
	Status   string `json:"status"`
	Scope    string `json:"scope,omitempty"`
	Resource string `json:"resource,omitempty"`
	Message  string `json:"message"`
	Endpoint string `json:"endpoint,omitempty"`
}

type ObservabilityEndpointHints struct {
	SandboxUsage   string `json:"sandboxUsage"`
	TemplateUsage  string `json:"templateUsage"`
	SandboxDetail  string `json:"sandboxDetail,omitempty"`
	SandboxMetrics string `json:"sandboxMetrics,omitempty"`
	SandboxLogs    string `json:"sandboxLogs"`
	BuildStatus    string `json:"buildStatus,omitempty"`
	BuildLogs      string `json:"buildLogs"`
}

type ObservabilityUsage struct {
	Sandboxes *SandboxUsageLimits  `json:"sandboxes,omitempty"`
	Templates *TemplateUsageLimits `json:"templates,omitempty"`
}

type ObservabilitySummary struct {
	Status       string                         `json:"status"`
	ProjectID    string                         `json:"projectID,omitempty"`
	UserID       string                         `json:"userID,omitempty"`
	Usage        *ObservabilityUsage            `json:"usage,omitempty"`
	Availability map[string]ObservabilitySignal `json:"availability"`
	Checks       []ObservabilityCheck           `json:"checks"`
	Actions      []ObservabilityAction          `json:"actions"`
	Endpoints    ObservabilityEndpointHints     `json:"endpoints"`
}

// SandboxLogsParams configures GET /api/v1/sandboxes/:sandboxID/logs.
type SandboxLogsParams struct {
	Cursor    *int64
	Limit     *int
	Direction string
	Level     string
	Search    string
}

// SandboxLogEntry is one log record.
type SandboxLogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Message   string            `json:"message"`
	Level     string            `json:"level"`
	Fields    map[string]string `json:"fields"`
}

// LogDiagnostic explains an empty log response using public product terms.
type LogDiagnostic struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// SandboxLogsQuery echoes the normalized query used for a sandbox logs request.
type SandboxLogsQuery struct {
	SandboxID string `json:"sandboxID,omitempty"`
	Direction string `json:"direction,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Level     string `json:"level,omitempty"`
	Search    string `json:"search,omitempty"`
}

// SandboxLogsResponse wraps sandbox log records.
type SandboxLogsResponse struct {
	Logs       []SandboxLogEntry `json:"logs"`
	NextCursor *int64            `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
	Query      *SandboxLogsQuery `json:"query,omitempty"`
	Diagnostic *LogDiagnostic    `json:"diagnostic,omitempty"`
}

// ConnectSandboxRequest is the request body for POST /connect.
type ConnectSandboxRequest struct {
	Timeout int64 `json:"timeout"`
}

// ConnectSandboxResponse keeps both the sandbox payload and HTTP status.
type ConnectSandboxResponse struct {
	StatusCode int
	Sandbox    *Sandbox
}

// TimeoutRequest is the request body for POST /timeout.
type TimeoutRequest struct {
	Timeout int64 `json:"timeout"`
}

// RefreshSandboxRequest is the request body for POST /refreshes.
type RefreshSandboxRequest struct {
	Duration *int32 `json:"duration,omitempty"`
}

// HeartbeatRequest is the request body for POST /heartbeat.
type HeartbeatRequest struct {
	Status string `json:"status"`
}

// HeartbeatResponse is the wrapped success payload from POST /heartbeat.
type HeartbeatResponse struct {
	Received  bool   `json:"received"`
	Status    string `json:"status"`
	RequestID string `json:"request_id,omitempty"`
}

// PoolStatus is the wrapped success payload from GET /admin/pool/status.
type PoolStatus struct {
	Total       int     `json:"total"`
	Warm        int     `json:"warm"`
	Active      int     `json:"active"`
	Creating    int     `json:"creating"`
	Stopped     int     `json:"stopped"`
	Deleting    int     `json:"deleting"`
	Deleted     int     `json:"deleted"`
	Utilization float64 `json:"utilization"`
	RequestID   string  `json:"request_id,omitempty"`
}

// RollingStartRequest is the request body for POST /admin/rolling/start.
type RollingStartRequest struct {
	TemplateID string `json:"templateId"`
}

// RollingUpdateStatus is the wrapped success payload for admin rolling APIs.
type RollingUpdateStatus struct {
	Phase       string     `json:"phase"`
	Progress    float64    `json:"progress"`
	WarmTotal   int        `json:"warm_total"`
	WarmUpdated int        `json:"warm_updated"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Duration    string     `json:"duration,omitempty"`
	RequestID   string     `json:"request_id,omitempty"`
}
