package sandbox

import (
	"context"
	"strings"
	"sync"

	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

type Sandbox struct {
	*control.Sandbox
	gateway               *gatewayServices
	runtime               *Runtime
	codeContextMu         sync.Mutex
	codeContexts          map[string]*PythonCodeContextSession
	statelessCodeContexts map[string]*CodeContext
	defaultCodeContext    *PythonCodeContextSession
}

type SandboxDetail struct {
	*control.SandboxDetail
	gateway               *gatewayServices
	runtime               *Runtime
	codeContextMu         sync.Mutex
	codeContexts          map[string]*PythonCodeContextSession
	statelessCodeContexts map[string]*CodeContext
	defaultCodeContext    *PythonCodeContextSession
}

type SandboxHandle struct {
	*control.ListedSandbox
	gateway *gatewayServices
}

type ConnectSandboxResponse struct {
	StatusCode int
	Sandbox    *Sandbox
}

func (g *gatewayServices) createSandbox(ctx context.Context, req *control.NewSandboxRequest) (*Sandbox, error) {
	sandbox, err := g.control.CreateSandbox(ctx, req)
	if err != nil {
		return nil, err
	}
	return bindSandbox(g, sandbox), nil
}

func (g *gatewayServices) getSandbox(ctx context.Context, sandboxID string) (*SandboxDetail, error) {
	sandbox, err := g.control.GetSandbox(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	return bindSandboxDetail(g, sandbox), nil
}

func (g *gatewayServices) listSandboxes(
	ctx context.Context,
	params *control.ListSandboxesParams,
) ([]*SandboxHandle, error) {
	sandboxes, err := g.control.ListSandboxes(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]*SandboxHandle, 0, len(sandboxes))
	for i := range sandboxes {
		sandbox := sandboxes[i]
		out = append(out, bindSandboxHandle(g, &sandbox))
	}
	return out, nil
}

func (g *gatewayServices) connectSandbox(
	ctx context.Context,
	sandboxID string,
	req *control.ConnectSandboxRequest,
) (*ConnectSandboxResponse, error) {
	resp, err := g.control.ConnectSandbox(ctx, sandboxID, req)
	if err != nil {
		return nil, err
	}
	return &ConnectSandboxResponse{
		StatusCode: resp.StatusCode,
		Sandbox:    bindSandbox(g, resp.Sandbox),
	}, nil
}

func (s *Sandbox) Runtime() (*Runtime, error) {
	if s == nil || s.EnvdURL == nil || strings.TrimSpace(*s.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}
	if s.runtime != nil {
		return s.runtime, nil
	}
	runtime, err := RuntimeFromSandbox(s.Sandbox)
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	return s.runtime, nil
}

func (s *Sandbox) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox and returns the running sandbox handle.
func (s *Sandbox) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout < 0 {
		timeout = 300
	}
	resp, err := s.gateway.connectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo fetches the latest sandbox detail for this sandbox ID.
func (s *Sandbox) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

// GetMetrics reads runtime metrics for sandboxes that expose nano-executor access.
func (s *Sandbox) GetMetrics(ctx context.Context) (*cmd.MetricsResponse, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Metrics(ctx)
}

func (s *Sandbox) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.gateway.control.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *Sandbox) Pause(ctx context.Context) error {
	return s.gateway.control.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *Sandbox) Kill(ctx context.Context) error {
	s.closeAllCodeContexts(ctx)
	return s.Delete(ctx)
}

func (s *Sandbox) Delete(ctx context.Context) error {
	s.closeAllCodeContexts(ctx)
	return s.gateway.control.DeleteSandbox(ctx, s.SandboxID)
}

func (s *Sandbox) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.gateway.control.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *Sandbox) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.gateway.control.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *Sandbox) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.gateway.connectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *Sandbox) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func (s *SandboxDetail) Runtime() (*Runtime, error) {
	if s == nil || s.EnvdURL == nil || strings.TrimSpace(*s.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}
	if s.runtime != nil {
		return s.runtime, nil
	}
	runtime, err := RuntimeFromDetail(s.SandboxDetail)
	if err != nil {
		return nil, err
	}
	s.runtime = runtime
	return s.runtime, nil
}

func (s *SandboxDetail) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox detail and returns a running sandbox handle.
func (s *SandboxDetail) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout < 0 {
		timeout = 300
	}
	resp, err := s.gateway.connectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo refreshes the sandbox detail for this sandbox ID.
func (s *SandboxDetail) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

// GetMetrics reads runtime metrics for sandboxes that expose nano-executor access.
func (s *SandboxDetail) GetMetrics(ctx context.Context) (*cmd.MetricsResponse, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Metrics(ctx)
}

func (s *SandboxDetail) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.gateway.control.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *SandboxDetail) Pause(ctx context.Context) error {
	return s.gateway.control.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *SandboxDetail) Kill(ctx context.Context) error {
	s.closeAllCodeContexts(ctx)
	return s.Delete(ctx)
}

func (s *SandboxDetail) Delete(ctx context.Context) error {
	s.closeAllCodeContexts(ctx)
	return s.gateway.control.DeleteSandbox(ctx, s.SandboxID)
}

func (s *SandboxDetail) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.gateway.control.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *SandboxDetail) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.gateway.control.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *SandboxDetail) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.gateway.connectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *SandboxDetail) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func (s *SandboxHandle) Reload(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

// Resume reconnects to a paused sandbox handle and returns a running sandbox handle.
func (s *SandboxHandle) Resume(ctx context.Context, timeout int32) (*Sandbox, error) {
	if timeout < 0 {
		timeout = 300
	}
	resp, err := s.gateway.connectSandbox(ctx, s.SandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

// GetInfo fetches the latest sandbox detail for this sandbox ID.
func (s *SandboxHandle) GetInfo(ctx context.Context) (*SandboxDetail, error) {
	return s.gateway.getSandbox(ctx, s.SandboxID)
}

func (s *SandboxHandle) Logs(ctx context.Context, params *control.SandboxLogsParams) (*control.SandboxLogsResponse, error) {
	return s.gateway.control.GetSandboxLogs(ctx, s.SandboxID, params)
}

func (s *SandboxHandle) Pause(ctx context.Context) error {
	return s.gateway.control.PauseSandbox(ctx, s.SandboxID)
}

// Kill deletes the sandbox.
func (s *SandboxHandle) Kill(ctx context.Context) error {
	return s.Delete(ctx)
}

func (s *SandboxHandle) Delete(ctx context.Context) error {
	return s.gateway.control.DeleteSandbox(ctx, s.SandboxID)
}

func (s *SandboxHandle) Refresh(ctx context.Context, req *control.RefreshSandboxRequest) error {
	return s.gateway.control.RefreshSandbox(ctx, s.SandboxID, req)
}

func (s *SandboxHandle) SetTimeout(ctx context.Context, req *control.TimeoutRequest) error {
	return s.gateway.control.SetSandboxTimeout(ctx, s.SandboxID, req)
}

func (s *SandboxHandle) Connect(ctx context.Context, req *control.ConnectSandboxRequest) (*ConnectSandboxResponse, error) {
	return s.gateway.connectSandbox(ctx, s.SandboxID, req)
}

// IsRunning reports whether the sandbox is in an active state.
func (s *SandboxHandle) IsRunning() bool {
	return isRunningSandboxState(s.State, s.Status)
}

func bindSandbox(gateway *gatewayServices, sandbox *control.Sandbox) *Sandbox {
	if sandbox == nil {
		return nil
	}
	return &Sandbox{
		Sandbox: sandbox,
		gateway: gateway,
	}
}

func bindSandboxDetail(gateway *gatewayServices, sandbox *control.SandboxDetail) *SandboxDetail {
	if sandbox == nil {
		return nil
	}
	return &SandboxDetail{
		SandboxDetail: sandbox,
		gateway:       gateway,
	}
}

func bindSandboxHandle(gateway *gatewayServices, sandbox *control.ListedSandbox) *SandboxHandle {
	if sandbox == nil {
		return nil
	}
	return &SandboxHandle{
		ListedSandbox: sandbox,
		gateway:       gateway,
	}
}

func isRunningSandboxState(state, status string) bool {
	switch strings.ToLower(strings.TrimSpace(firstNonEmpty(state, status))) {
	case "paused", "stopped", "deleted":
		return false
	default:
		return true
	}
}
