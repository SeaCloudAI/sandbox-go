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
	TemplateID string            `json:"templateID"`
	Timeout    *int64            `json:"timeout,omitempty"`
	AutoPause  *bool             `json:"autoPause,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	EnvVars    map[string]string `json:"envVars,omitempty"`
	WaitReady  *bool             `json:"waitReady,omitempty"`
}

type SandboxLifecycle struct {
	OnTimeout string `json:"onTimeout"`
}

// Sandbox is returned by create and connect endpoints.
type Sandbox struct {
	TemplateID      string     `json:"templateID"`
	SandboxID       string     `json:"sandboxID"`
	Alias           string     `json:"alias,omitempty"`
	ClientID        string     `json:"clientID"`
	EnvdAccessToken *string    `json:"envdAccessToken"`
	EnvdURL         *string    `json:"envdUrl"`
	Namespace       string     `json:"namespace,omitempty"`
	Status          string     `json:"status"`
	State           string     `json:"state,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	ActivatedAt     *time.Time `json:"activatedAt,omitempty"`
	EndAt           time.Time  `json:"endAt"`
}

// SandboxDetail is returned by GET /api/v1/sandboxes/:sandboxID.
type SandboxDetail struct {
	TemplateID      string            `json:"templateID"`
	Alias           string            `json:"alias,omitempty"`
	SandboxID       string            `json:"sandboxID"`
	ClientID        string            `json:"clientID"`
	StartedAt       time.Time         `json:"startedAt"`
	EndAt           time.Time         `json:"endAt"`
	EnvdAccessToken *string           `json:"envdAccessToken"`
	EnvdURL         *string           `json:"envdUrl"`
	CPUCount        int32             `json:"cpuCount"`
	MemoryMB        int32             `json:"memoryMB"`
	DiskSizeMB      int32             `json:"diskSizeMB"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Status          string            `json:"status"`
	State           string            `json:"state,omitempty"`
	Lifecycle       SandboxLifecycle  `json:"lifecycle"`
	VolumeMounts    []VolumeMount     `json:"volumeMounts,omitempty"`
	Namespace       string            `json:"namespace,omitempty"`
	ActivatedAt     *time.Time        `json:"activatedAt,omitempty"`
}

// ListedSandbox is returned by the list endpoint.
type ListedSandbox struct {
	TemplateID   string            `json:"templateID"`
	Alias        string            `json:"alias,omitempty"`
	SandboxID    string            `json:"sandboxID"`
	ClientID     string            `json:"clientID"`
	StartedAt    time.Time         `json:"startedAt"`
	EndAt        time.Time         `json:"endAt"`
	CPUCount     int32             `json:"cpuCount"`
	MemoryMB     int32             `json:"memoryMB"`
	DiskSizeMB   int32             `json:"diskSizeMB"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Status       string            `json:"status"`
	State        string            `json:"state,omitempty"`
	VolumeMounts []VolumeMount     `json:"volumeMounts,omitempty"`
	ActivatedAt  *time.Time        `json:"activatedAt,omitempty"`
}

// ListSandboxesParams configures GET /api/v1/sandboxes.
type ListSandboxesParams struct {
	Metadata  map[string]string
	State     []string
	Limit     int
	NextToken string
}

// SandboxMetricsRaw is the raw nano-executor metrics payload embedded in a
// control-plane metrics snapshot when available.
type SandboxMetricsRaw struct {
	Timestamp   int64   `json:"ts"`
	CPUCount    int32   `json:"cpu_count"`
	CPUUsedPct  float64 `json:"cpu_used_pct"`
	MemTotal    int64   `json:"mem_total"`
	MemUsed     int64   `json:"mem_used"`
	MemTotalMiB int64   `json:"mem_total_mib"`
	MemUsedMiB  int64   `json:"mem_used_mib"`
	MemCache    int64   `json:"mem_cache"`
	DiskUsed    int64   `json:"disk_used"`
	DiskTotal   int64   `json:"disk_total"`
	NetRxBytes  int64   `json:"net_rx_bytes"`
	NetTxBytes  int64   `json:"net_tx_bytes"`
}

// SandboxMetricSnapshot is one sandbox control-plane metrics snapshot.
type SandboxMetricSnapshot struct {
	SandboxID   string    `json:"sandboxID"`
	CollectedAt time.Time `json:"collectedAt"`
	Error       string    `json:"error,omitempty"`

	CPUCount      int32    `json:"cpuCount"`
	CPUUsedPct    float64  `json:"cpuUsedPct"`
	Load1         *float64 `json:"load1,omitempty"`
	Load5         *float64 `json:"load5,omitempty"`
	Load15        *float64 `json:"load15,omitempty"`
	CPUUserRate   *float64 `json:"cpuUserRate,omitempty"`
	CPUSystemRate *float64 `json:"cpuSystemRate,omitempty"`
	CPUIOWaitRate *float64 `json:"cpuIOWaitRate,omitempty"`
	CPUStealRate  *float64 `json:"cpuStealRate,omitempty"`

	MemTotal             int64    `json:"memTotal"`
	MemUsed              int64    `json:"memUsed"`
	MemTotalMiB          int64    `json:"memTotalMiB"`
	MemUsedMiB           int64    `json:"memUsedMiB"`
	MemCache             int64    `json:"memCache"`
	MemoryAvailableBytes *int64   `json:"memoryAvailableBytes,omitempty"`
	MemoryUsagePercent   *float64 `json:"memoryUsagePercent,omitempty"`
	SwapTotalBytes       *int64   `json:"swapTotalBytes,omitempty"`
	SwapFreeBytes        *int64   `json:"swapFreeBytes,omitempty"`
	SwapCachedBytes      *int64   `json:"swapCachedBytes,omitempty"`

	DiskUsed                int64    `json:"diskUsed"`
	DiskTotal               int64    `json:"diskTotal"`
	DiskReadOpsPerSecond    *float64 `json:"diskReadOpsPerSecond,omitempty"`
	DiskWriteOpsPerSecond   *float64 `json:"diskWriteOpsPerSecond,omitempty"`
	DiskReadBytesPerSecond  *float64 `json:"diskReadBytesPerSecond,omitempty"`
	DiskWriteBytesPerSecond *float64 `json:"diskWriteBytesPerSecond,omitempty"`

	NetRxBytes                  int64    `json:"netRxBytes"`
	NetTxBytes                  int64    `json:"netTxBytes"`
	NetworkRecvBytesPerSecond   *float64 `json:"networkRecvBytesPerSecond,omitempty"`
	NetworkSentBytesPerSecond   *float64 `json:"networkSentBytesPerSecond,omitempty"`
	NetworkRecvPacketsPerSecond *float64 `json:"networkRecvPacketsPerSecond,omitempty"`
	NetworkSentPacketsPerSecond *float64 `json:"networkSentPacketsPerSecond,omitempty"`
	NetworkRecvErrorsPerSecond  *float64 `json:"networkRecvErrorsPerSecond,omitempty"`
	NetworkSentErrorsPerSecond  *float64 `json:"networkSentErrorsPerSecond,omitempty"`
	NetworkRecvDropsPerSecond   *float64 `json:"networkRecvDropsPerSecond,omitempty"`
	NetworkSentDropsPerSecond   *float64 `json:"networkSentDropsPerSecond,omitempty"`
	TaskCurrent                 *int64   `json:"taskCurrent,omitempty"`
	TaskMax                     *int64   `json:"taskMax,omitempty"`

	Raw *SandboxMetricsRaw `json:"raw,omitempty"`
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

// SandboxLogsResponse wraps sandbox log records.
type SandboxLogsResponse struct {
	Logs []SandboxLogEntry `json:"logs"`
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
