package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/core"
)

const autoCopyPrefix = "__auto_copy__:"

type CreateOptions struct {
	TemplateID string
	Timeout    *int64
	AutoPause  *bool
	Metadata   map[string]string
	EnvVars    map[string]string
	WaitReady  *bool
}

type ConnectOptions struct {
	Timeout *int64
}

type SandboxURLOptions struct {
	User                   string
	UseSignatureExpiration *int64
}

type ListOptions struct {
	Metadata  map[string]string
	State     []string
	Limit     int
	NextToken string
}

type CommandRunOptions struct {
	Args      []string
	Envs      map[string]string
	CWD       string
	TimeoutMS *int64
	Stdin     *string
	StdinOpen *bool
	OnStdout  func(string)
	OnStderr  func(string)
	User      string
}

type CommandConnectOptions struct {
	OnStdout func(string)
	OnStderr func(string)
}

type PtyCreateOptions struct {
	Args      []string
	Envs      map[string]string
	CWD       string
	TimeoutMS *int64
	Size      *cmd.PtySize
	User      string
}

type PtyConnectOptions struct {
	OnStdout func(string)
	OnStderr func(string)
}

type FilesystemRequestOptions struct {
	User string
}

type FilesystemListOptions struct {
	User  string
	Depth *int
}

type WatchDirOptions struct {
	User      string
	Recursive *bool
	TimeoutMS *int64
	OnExit    func(error)
}

type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMS int64
	Error      string
}

type CommandHandle struct {
	runtime  *Runtime
	stream   *cmd.ProcessStream
	PID      int
	CmdID    string
	PTY      bool
	onStdout func(string)
	onStderr func(string)
}

type CommandWaitResult struct {
	Stdout   string
	Stderr   string
	PTY      string
	ExitCode int
}

type FileType string

const (
	FileTypeFile    FileType = "file"
	FileTypeDir     FileType = "dir"
	FileTypeSymlink FileType = "symlink"
)

type EntryInfo struct {
	Name          string
	Type          FileType
	Path          string
	Size          int64
	Mode          int64
	Permissions   string
	Owner         string
	Group         string
	ModifiedTime  *time.Time
	SymlinkTarget *string
}

type FilesystemEventType string

const (
	FilesystemEventCreate FilesystemEventType = "create"
	FilesystemEventWrite  FilesystemEventType = "write"
	FilesystemEventRemove FilesystemEventType = "remove"
	FilesystemEventRename FilesystemEventType = "rename"
	FilesystemEventChmod  FilesystemEventType = "chmod"
)

type FilesystemEvent struct {
	Name string
	Type FilesystemEventType
}

type WriteInfo struct {
	Name string
	Path string
	Type FileType
}

type ReadOptions struct {
	Format string
}

type WatchHandle struct {
	stop func() error
	once sync.Once
	err  error
}

func (h *WatchHandle) Stop() error {
	if h == nil || h.stop == nil {
		return nil
	}
	h.once.Do(func() {
		h.err = h.stop()
	})
	return h.err
}

type CommandsModule struct {
	runtime *Runtime
}

type FilesystemModule struct {
	runtime *Runtime
}

type PtyModule struct {
	runtime *Runtime
}

type GitModule struct {
	commands *CommandsModule
}

type GitCommandOptions struct {
	Envs      map[string]string
	CWD       string
	TimeoutMS *int64
	User      string
}

type GitCloneOptions struct {
	GitCommandOptions
	Branch string
	Depth  int
}

func (s *Sandbox) Commands() (*CommandsModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &CommandsModule{runtime: runtime}, nil
}

func (s *SandboxDetail) Commands() (*CommandsModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &CommandsModule{runtime: runtime}, nil
}

func (s *Sandbox) Files() (*FilesystemModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &FilesystemModule{runtime: runtime}, nil
}

func (s *SandboxDetail) Files() (*FilesystemModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &FilesystemModule{runtime: runtime}, nil
}

func (s *Sandbox) Pty() (*PtyModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &PtyModule{runtime: runtime}, nil
}

func (s *SandboxDetail) Pty() (*PtyModule, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return &PtyModule{runtime: runtime}, nil
}

func (s *Sandbox) Git() (*GitModule, error) {
	commands, err := s.Commands()
	if err != nil {
		return nil, err
	}
	return &GitModule{commands: commands}, nil
}

func (s *SandboxDetail) Git() (*GitModule, error) {
	commands, err := s.Commands()
	if err != nil {
		return nil, err
	}
	return &GitModule{commands: commands}, nil
}

func (s *Sandbox) GetHost(port int) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeProxyURL(runtime.BaseURL(), port)
}

func (s *SandboxDetail) GetHost(port int) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeProxyURL(runtime.BaseURL(), port)
}

func (s *Sandbox) DownloadURL(path string, opts *SandboxURLOptions) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeFileURL(runtime.BaseURL(), stringValue(s.EnvdAccessToken), path, "read", opts)
}

func (s *SandboxDetail) DownloadURL(path string, opts *SandboxURLOptions) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeFileURL(runtime.BaseURL(), stringValue(s.EnvdAccessToken), path, "read", opts)
}

func (s *Sandbox) UploadURL(path string, opts *SandboxURLOptions) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeFileURL(runtime.BaseURL(), stringValue(s.EnvdAccessToken), path, "write", opts)
}

func (s *SandboxDetail) UploadURL(path string, opts *SandboxURLOptions) (string, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return "", err
	}
	return runtimeFileURL(runtime.BaseURL(), stringValue(s.EnvdAccessToken), path, "write", opts)
}

func (s *Sandbox) Proxy(ctx context.Context, req *cmd.ProxyRequest) (*http.Response, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Proxy(ctx, req)
}

func (s *SandboxDetail) Proxy(ctx context.Context, req *cmd.ProxyRequest) (*http.Response, error) {
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.Proxy(ctx, req)
}

func (m *CommandsModule) Run(ctx context.Context, command string, opts *CommandRunOptions) (*CommandResult, error) {
	if opts == nil {
		opts = &CommandRunOptions{}
	}
	runtimeTimeout, err := resolvePositiveRuntimeTimeoutMilliseconds(opts.TimeoutMS)
	if err != nil {
		return nil, err
	}
	if opts.OnStdout != nil || opts.OnStderr != nil || opts.StdinOpen != nil {
		handle, err := m.Start(ctx, command, opts)
		if err != nil {
			return nil, err
		}
		waited, err := handle.Wait(ctx)
		if err != nil {
			return nil, err
		}
		return &CommandResult{
			Stdout:   waited.Stdout,
			Stderr:   waited.Stderr,
			ExitCode: waited.ExitCode,
		}, nil
	}
	execCommand, execArgs := buildCommandExecution(command, opts.Args, opts.User)
	resp, err := m.runtime.Run(ctx, &cmd.AgentRunRequest{
		Cmd:       execCommand,
		Args:      execArgs,
		CWD:       opts.CWD,
		Env:       opts.Envs,
		TimeoutMS: runtimeTimeout,
		Stdin:     opts.Stdin,
	}, nil)
	if err != nil {
		return nil, err
	}
	return &CommandResult{
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		ExitCode:   resp.ExitCode,
		DurationMS: resp.DurationMS,
		Error:      resp.Error,
	}, nil
}

func (m *CommandsModule) Exec(ctx context.Context, command string, opts *CommandRunOptions) (*CommandResult, error) {
	return m.Run(ctx, command, opts)
}

func (m *CommandsModule) Start(ctx context.Context, command string, opts *CommandRunOptions) (*CommandHandle, error) {
	if opts == nil {
		opts = &CommandRunOptions{}
	}
	runtimeTimeout, err := resolvePositiveRuntimeTimeoutMilliseconds(opts.TimeoutMS)
	if err != nil {
		return nil, err
	}
	stdin := true
	if opts.StdinOpen != nil {
		stdin = *opts.StdinOpen
	}
	execCommand, execArgs := buildCommandExecution(command, opts.Args, opts.User)
	stream, err := m.runtime.Start(ctx, &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{
			Cmd:  execCommand,
			Args: execArgs,
			Envs: opts.Envs,
			CWD:  stringPtr(opts.CWD),
		},
		TimeoutMS: runtimeTimeout,
		Stdin:     &stdin,
	}, nil)
	if err != nil {
		return nil, err
	}
	handle, err := expectStartHandle(stream, m.runtime, false)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	handle.onStdout = opts.OnStdout
	handle.onStderr = opts.OnStderr
	if opts.Stdin != nil {
		if err := handle.SendStdin(ctx, *opts.Stdin); err != nil {
			_ = handle.Close()
			return nil, err
		}
	}
	return handle, nil
}

func (m *CommandsModule) Connect(ctx context.Context, pid int, opts ...*CommandConnectOptions) (*CommandHandle, error) {
	stream, err := m.runtime.Connect(ctx, &cmd.ConnectRequest{
		Process: cmd.ProcessSelector{PID: pid},
	}, nil)
	if err != nil {
		return nil, err
	}
	handle, err := expectStartHandle(stream, m.runtime, false)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	if len(opts) > 0 && opts[0] != nil {
		handle.onStdout = opts[0].OnStdout
		handle.onStderr = opts[0].OnStderr
	}
	return handle, nil
}

func (m *CommandsModule) List(ctx context.Context) ([]cmd.ProcessInfo, error) {
	resp, err := m.runtime.ListProcesses(ctx, nil)
	if err != nil {
		return nil, err
	}
	return resp.Processes, nil
}

func (m *CommandsModule) Kill(ctx context.Context, pid int) (bool, error) {
	err := m.runtime.SendSignal(ctx, &cmd.SendSignalRequest{
		Process: cmd.ProcessSelector{PID: pid},
		Signal:  string(cmd.SignalSIGKILL),
	}, nil)
	if isMissingProcessError(err) {
		return false, nil
	}
	return err == nil, err
}

func (m *CommandsModule) SendStdin(ctx context.Context, pid int, data string) error {
	return m.runtime.SendInput(ctx, &cmd.SendInputRequest{
		Process: cmd.ProcessSelector{PID: pid},
		Input:   cmd.ProcessInput{Stdin: encodeStreamData(data)},
	}, nil)
}

func (m *FilesystemModule) Exists(ctx context.Context, path string) (bool, error) {
	return m.ExistsWithOptions(ctx, path, nil)
}

func (m *FilesystemModule) ExistsWithOptions(ctx context.Context, path string, opts *FilesystemRequestOptions) (bool, error) {
	_, err := m.runtime.Stat(ctx, &cmd.StatRequest{Path: path}, filesystemRequestOptions(opts))
	if apiErr, ok := err.(*core.APIError); ok && apiErr.Kind == core.APIErrorKindNotFound {
		return false, nil
	}
	return err == nil, err
}

func (m *FilesystemModule) GetInfo(ctx context.Context, path string) (*EntryInfo, error) {
	return m.GetInfoWithOptions(ctx, path, nil)
}

func (m *FilesystemModule) GetInfoWithOptions(ctx context.Context, path string, opts *FilesystemRequestOptions) (*EntryInfo, error) {
	resp, err := m.runtime.Stat(ctx, &cmd.StatRequest{Path: path}, filesystemRequestOptions(opts))
	if err != nil {
		return nil, err
	}
	return normalizeEntryInfo(resp.Entry), nil
}

func (m *FilesystemModule) List(ctx context.Context, path string, depth *int) ([]EntryInfo, error) {
	return m.ListWithOptions(ctx, path, &FilesystemListOptions{Depth: depth})
}

func (m *FilesystemModule) ListWithOptions(ctx context.Context, path string, opts *FilesystemListOptions) ([]EntryInfo, error) {
	var depth *int
	if opts != nil {
		depth = opts.Depth
	}
	resp, err := m.runtime.ListDir(ctx, &cmd.ListDirRequest{Path: path, Depth: depth}, filesystemListOptions(opts))
	if err != nil {
		return nil, err
	}
	out := make([]EntryInfo, 0, len(resp.Entries))
	for _, entry := range resp.Entries {
		out = append(out, *normalizeEntryInfo(entry))
	}
	return out, nil
}

func (m *FilesystemModule) MakeDir(ctx context.Context, path string) (bool, error) {
	return m.MakeDirWithOptions(ctx, path, nil)
}

func (m *FilesystemModule) MakeDirWithOptions(ctx context.Context, path string, opts *FilesystemRequestOptions) (bool, error) {
	exists, err := m.ExistsWithOptions(ctx, path, opts)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	_, err = m.runtime.MakeDir(ctx, &cmd.MakeDirRequest{Path: path}, filesystemRequestOptions(opts))
	return err == nil, err
}

func (m *FilesystemModule) Read(ctx context.Context, path string, opts *ReadOptions) (any, error) {
	return m.ReadWithOptions(ctx, path, opts, nil)
}

func (m *FilesystemModule) ReadWithOptions(ctx context.Context, path string, opts *ReadOptions, reqOpts *FilesystemRequestOptions) (any, error) {
	resp, err := m.runtime.ReadFile(ctx, &cmd.FileRequest{Path: path}, filesystemRequestOptions(reqOpts))
	if err != nil {
		return nil, err
	}
	format := "text"
	if opts != nil && strings.TrimSpace(opts.Format) != "" {
		format = strings.TrimSpace(opts.Format)
	}
	if format == "stream" {
		return resp.Body, nil
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if format == "bytes" || format == "blob" {
		return body, nil
	}
	return string(body), nil
}

func (m *FilesystemModule) Write(ctx context.Context, path string, data any) (*WriteInfo, error) {
	return m.WriteWithOptions(ctx, path, data, nil)
}

func (m *FilesystemModule) WriteWithOptions(ctx context.Context, path string, data any, opts *FilesystemRequestOptions) (*WriteInfo, error) {
	raw, err := normalizeWriteData(data)
	if err != nil {
		return nil, err
	}
	if err := m.runtime.WriteFile(ctx, &cmd.UploadBytesRequest{Path: path, Data: raw}, filesystemRequestOptions(opts)); err != nil {
		return nil, err
	}
	return normalizeWriteInfo(path), nil
}

func (m *FilesystemModule) WriteFiles(ctx context.Context, files []cmd.WriteFileEntry) ([]WriteInfo, error) {
	return m.WriteFilesWithOptions(ctx, files, nil)
}

func (m *FilesystemModule) WriteFilesWithOptions(ctx context.Context, files []cmd.WriteFileEntry, opts *FilesystemRequestOptions) ([]WriteInfo, error) {
	resp, err := m.runtime.WriteBatch(ctx, &cmd.WriteFilesRequest{Files: files}, filesystemRequestOptions(opts))
	if err != nil {
		return nil, err
	}
	out := make([]WriteInfo, 0, len(resp.Files))
	for _, file := range resp.Files {
		out = append(out, *normalizeWriteInfo(file.Path))
	}
	return out, nil
}

func (m *FilesystemModule) Remove(ctx context.Context, path string) error {
	return m.RemoveWithOptions(ctx, path, nil)
}

func (m *FilesystemModule) RemoveWithOptions(ctx context.Context, path string, opts *FilesystemRequestOptions) error {
	return m.runtime.Remove(ctx, &cmd.RemoveRequest{Path: path}, filesystemRequestOptions(opts))
}

func (m *FilesystemModule) Rename(ctx context.Context, oldPath, newPath string) (*EntryInfo, error) {
	return m.RenameWithOptions(ctx, oldPath, newPath, nil)
}

func (m *FilesystemModule) RenameWithOptions(ctx context.Context, oldPath, newPath string, opts *FilesystemRequestOptions) (*EntryInfo, error) {
	resp, err := m.runtime.Move(ctx, &cmd.MoveRequest{Source: oldPath, Destination: newPath}, filesystemRequestOptions(opts))
	if err != nil {
		return nil, err
	}
	return normalizeEntryInfo(resp.Entry), nil
}

func (m *FilesystemModule) WatchDir(ctx context.Context, path string, onEvent func(FilesystemEvent) error, recursive *bool) (*WatchHandle, error) {
	return m.WatchDirWithOptions(ctx, path, onEvent, &WatchDirOptions{Recursive: recursive})
}

func (m *FilesystemModule) WatchDirWithOptions(ctx context.Context, path string, onEvent func(FilesystemEvent) error, opts *WatchDirOptions) (*WatchHandle, error) {
	var recursive *bool
	if opts != nil {
		recursive = opts.Recursive
	}
	stream, err := m.runtime.WatchDir(ctx, &cmd.WatchDirRequest{Path: path, Recursive: recursive}, watchDirRequestOptions(opts))
	if err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	stopRequested := make(chan struct{})
	var timer *time.Timer
	if opts != nil && opts.TimeoutMS != nil {
		if *opts.TimeoutMS < 0 {
			_ = stream.Close()
			return nil, fmt.Errorf("sandbox: timeoutMs must be non-negative")
		}
		if *opts.TimeoutMS > 0 {
			timer = time.AfterFunc(time.Duration(*opts.TimeoutMS)*time.Millisecond, func() {
				_ = stream.Close()
			})
		}
	}
	go func() {
		defer close(done)
		var exitErr error
		defer func() {
			if timer != nil {
				timer.Stop()
			}
			select {
			case <-stopRequested:
			default:
				if opts != nil && opts.OnExit != nil {
					opts.OnExit(exitErr)
				}
			}
		}()
		for {
			frame, err := stream.Next()
			if err != nil {
				if errors.Is(err, io.EOF) {
					done <- nil
					return
				}
				select {
				case <-stopRequested:
					done <- nil
					return
				default:
				}
				exitErr = err
				done <- err
				return
			}
			if frame == nil || frame.Filesystem == nil {
				done <- nil
				return
			}
			if onEvent != nil {
				if err := onEvent(*normalizeFilesystemEvent(*frame.Filesystem)); err != nil {
					exitErr = err
					done <- err
					return
				}
			}
		}
	}()
	return &WatchHandle{
		stop: func() error {
			close(stopRequested)
			closeErr := stream.Close()
			doneErr := <-done
			if closeErr != nil {
				return closeErr
			}
			return doneErr
		},
	}, nil
}

func (m *PtyModule) Kill(ctx context.Context, pid int) (bool, error) {
	err := m.runtime.SendSignal(ctx, &cmd.SendSignalRequest{
		Process: cmd.ProcessSelector{PID: pid},
		Signal:  string(cmd.SignalSIGKILL),
	}, nil)
	if isMissingProcessError(err) {
		return false, nil
	}
	return err == nil, err
}

func (m *PtyModule) SendStdin(ctx context.Context, pid int, data string) error {
	return m.runtime.SendInput(ctx, &cmd.SendInputRequest{
		Process: cmd.ProcessSelector{PID: pid},
		Input:   cmd.ProcessInput{PTY: encodeStreamData(data)},
	}, nil)
}

func (m *PtyModule) SendInput(ctx context.Context, pid int, data string) error {
	return m.SendStdin(ctx, pid, data)
}

func (m *PtyModule) Resize(ctx context.Context, pid int, size cmd.PtySize) error {
	return m.runtime.Update(ctx, &cmd.UpdateRequest{
		Process: cmd.ProcessSelector{PID: pid},
		PTY:     &cmd.PtyConfig{Size: size},
	}, nil)
}

func (m *PtyModule) Create(ctx context.Context, command string, opts *PtyCreateOptions) (*CommandHandle, error) {
	if opts == nil {
		opts = &PtyCreateOptions{}
	}
	runtimeTimeout, err := resolvePositiveRuntimeTimeoutMilliseconds(opts.TimeoutMS)
	if err != nil {
		return nil, err
	}
	stdin := true
	size := opts.Size
	if size == nil {
		size = &cmd.PtySize{Cols: 80, Rows: 24}
	}
	execCommand, execArgs := buildCommandExecution(command, opts.Args, opts.User)
	stream, err := m.runtime.Start(ctx, &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{
			Cmd:  execCommand,
			Args: execArgs,
			Envs: opts.Envs,
			CWD:  stringPtr(opts.CWD),
		},
		TimeoutMS: runtimeTimeout,
		Stdin:     &stdin,
		PTY:       &cmd.PtyConfig{Size: *size},
	}, nil)
	if err != nil {
		return nil, err
	}
	handle, err := expectStartHandle(stream, m.runtime, true)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	return handle, nil
}

func (m *PtyModule) Connect(ctx context.Context, pid int, opts ...*PtyConnectOptions) (*CommandHandle, error) {
	stream, err := m.runtime.Connect(ctx, &cmd.ConnectRequest{
		Process: cmd.ProcessSelector{PID: pid},
	}, nil)
	if err != nil {
		return nil, err
	}
	handle, err := expectStartHandle(stream, m.runtime, true)
	if err != nil {
		_ = stream.Close()
		return nil, err
	}
	if len(opts) > 0 && opts[0] != nil {
		handle.onStdout = opts[0].OnStdout
		handle.onStderr = opts[0].OnStderr
	}
	return handle, nil
}

func (m *GitModule) Clone(ctx context.Context, repoURL, path string, opts *GitCloneOptions) (*CommandResult, error) {
	args := make([]string, 0, 6)
	if opts != nil && strings.TrimSpace(opts.Branch) != "" {
		args = append(args, "--branch", strings.TrimSpace(opts.Branch))
	}
	if opts != nil && opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, repoURL)
	if strings.TrimSpace(path) != "" {
		args = append(args, path)
	}
	return m.run(ctx, "clone", args, gitCommandOptions(opts))
}

func (m *GitModule) Pull(ctx context.Context, path string, opts *GitCommandOptions) (*CommandResult, error) {
	cloned := cloneGitCommandOptions(opts)
	if strings.TrimSpace(path) != "" {
		cloned.CWD = path
	}
	return m.run(ctx, "pull", nil, &cloned)
}

func (m *GitModule) Checkout(ctx context.Context, ref, path string, opts *GitCommandOptions) (*CommandResult, error) {
	cloned := cloneGitCommandOptions(opts)
	if strings.TrimSpace(path) != "" {
		cloned.CWD = path
	}
	return m.run(ctx, "checkout", []string{ref}, &cloned)
}

func (m *GitModule) Status(ctx context.Context, path string, opts *GitCommandOptions) (*CommandResult, error) {
	cloned := cloneGitCommandOptions(opts)
	if strings.TrimSpace(path) != "" {
		cloned.CWD = path
	}
	return m.run(ctx, "status", nil, &cloned)
}

func (m *GitModule) run(ctx context.Context, subcommand string, args []string, opts *GitCommandOptions) (*CommandResult, error) {
	command, commandArgs := buildGitExecution(subcommand, args, strings.TrimSpace(opts.User))
	return m.commands.Run(ctx, command, &CommandRunOptions{
		Args:      commandArgs,
		Envs:      cloneStringMap(opts.Envs),
		CWD:       opts.CWD,
		TimeoutMS: opts.TimeoutMS,
	})
}

type ReadyCmd struct {
	cmd string
}

func (r ReadyCmd) Command() string {
	return r.cmd
}

type Template struct {
	builder    *build.TemplateBuildBuilder
	autoCopies map[string]templateAutoCopy
	skipCache  bool
}

type templateAutoCopy struct {
	Source       string
	ForceUpload  bool
	Mode         *int
	ResolveLinks bool
}

// TemplateCommandOptions configures template RUN helper commands.
type TemplateCommandOptions struct {
	User  string
	Force *bool
}

// TemplateCopyOptions configures local sources copied into a template build.
type TemplateCopyOptions struct {
	FilesHash       string
	ForceUpload     bool
	Mode            *int
	ResolveSymlinks bool
	User            string
}

// TemplateCopyItem describes one COPY helper entry with per-item options.
type TemplateCopyItem struct {
	Src             string
	Srcs            []string
	Dest            string
	FilesHash       string
	ForceUpload     bool
	Mode            *int
	ResolveSymlinks bool
	User            string
}

type TemplatePathOptions struct {
	User  string
	Force *bool
}

type TemplateRemoveOptions struct {
	TemplatePathOptions
	Recursive bool
}

type TemplateRenameOptions struct {
	TemplatePathOptions
}

type TemplateAptInstallOptions struct {
	NoInstallRecommends bool
	Force               *bool
}

type TemplateGitCloneOptions struct {
	Branch string
	Depth  int
	User   string
	Force  *bool
}

type TemplateMakeDirOptions struct {
	TemplatePathOptions
	Mode *int
}

type TemplateMakeSymlinkOptions struct {
	TemplatePathOptions
}

type TemplateNpmInstallOptions struct {
	Dev   bool
	G     bool
	Force *bool
}

type TemplatePipInstallOptions struct {
	G     *bool
	Force *bool
}

type TemplateBunInstallOptions struct {
	Dev   bool
	G     bool
	Force *bool
}

type TemplateBuildOptions struct {
	Tags           []string
	BaseTemplateID string
	CPUCount       *int32
	MemoryMB       *int32
	Wait           *bool
	PollInterval   time.Duration
	OnBuildLog     func(LogEntry)
}

type TemplateBuildInfo struct {
	TemplateID string
	BuildID    string
	Name       string
	Tags       []string
	Alias      string
	Status     string
	Template   *build.TemplateResponse
	Build      *build.BuildResponse
}

type TemplateListOptions struct {
	Visibility string
	Limit      int
	Offset     int
}

type TemplateGetOptions struct {
	Limit     int
	NextToken string
}

type TemplateBuildStatusOptions struct {
	LogsOffset *int
	Limit      *int
	Level      string
}

type TemplateTag struct {
	BuildID   string
	CreatedAt time.Time
	Tag       string
}

type TemplateTagInfo struct {
	BuildID string
	Tags    []string
}

type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
}

func (l LogEntry) String() string {
	return "[" + l.Timestamp.UTC().Format(time.RFC3339) + "] " + strings.ToUpper(l.Level) + " " + l.Message
}

// NewTemplate creates a high-level template builder with E2B-style helpers.
func NewTemplate() *Template {
	return &Template{
		builder:    build.NewTemplateBuildBuilder(),
		autoCopies: map[string]templateAutoCopy{},
	}
}

func (t *Template) FromImage(image string, registryConfig ...map[string]any) *Template {
	t.builder.FromImage(image)
	if len(registryConfig) > 0 && registryConfig[0] != nil {
		t.builder.FromImageRegistry(registryConfig[0])
	}
	return t
}

func (t *Template) FromTemplate(templateID string) *Template {
	t.builder.FromTemplate(templateID)
	return t
}

// FromDockerfile parses a supported Dockerfile subset into template build steps.
func (t *Template) FromDockerfile(dockerfileContentOrPath string) (*Template, error) {
	if t == nil {
		return nil, fmt.Errorf("sandbox: template is required")
	}
	content, contextDir, err := resolveDockerfileInput(dockerfileContentOrPath)
	if err != nil {
		return nil, err
	}
	seenFrom := false
	for _, instruction := range parseDockerfileInstructions(content) {
		switch instruction.Name {
		case "FROM":
			if seenFrom {
				return nil, fmt.Errorf("sandbox: Dockerfile multi-stage builds are not supported")
			}
			tokens, err := tokenizeShellLike(instruction.Value)
			if err != nil {
				return nil, err
			}
			if len(tokens) != 1 {
				return nil, fmt.Errorf("sandbox: FROM only supports a single base image")
			}
			t.FromImage(tokens[0])
			seenFrom = true
		case "RUN":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			command, err := requireDockerfileValue(instruction.Name, instruction.Value)
			if err != nil {
				return nil, err
			}
			t.RunCmd(command, nil)
		case "ENV":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			pairs, err := parseDockerfileEnv(instruction.Value)
			if err != nil {
				return nil, err
			}
			for _, pair := range pairs {
				t.builder.Env(pair[0], pair[1])
			}
		case "WORKDIR":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			workdir, err := requireDockerfileValue(instruction.Name, instruction.Value)
			if err != nil {
				return nil, err
			}
			t.SetWorkdir(workdir)
		case "USER":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			user, err := requireDockerfileValue(instruction.Name, instruction.Value)
			if err != nil {
				return nil, err
			}
			t.SetUser(user)
		case "COPY":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			sources, dest, err := parseDockerfileCopy(instruction.Value)
			if err != nil {
				return nil, err
			}
			for _, source := range sources {
				t.Copy(resolveDockerfileCopyPath(source, contextDir), dest, nil)
			}
		case "CMD":
			if err := ensureDockerfileBaseImage(seenFrom); err != nil {
				return nil, err
			}
			command, err := parseDockerfileCmd(instruction.Value)
			if err != nil {
				return nil, err
			}
			t.builder.StartCmd(command)
		default:
			return nil, fmt.Errorf("sandbox: unsupported Dockerfile instruction: %s", instruction.Name)
		}
	}
	if !seenFrom {
		return nil, fmt.Errorf("sandbox: Dockerfile must include a FROM instruction")
	}
	return t, nil
}

func (t *Template) FromBaseImage() *Template {
	return t.FromImage("e2bdev/base:latest")
}

func (t *Template) FromNodeImage(variant string) *Template {
	if strings.TrimSpace(variant) == "" {
		variant = "lts"
	}
	return t.FromImage("node:" + strings.TrimSpace(variant))
}

func (t *Template) FromPythonImage(version string) *Template {
	if strings.TrimSpace(version) == "" {
		version = "3"
	}
	return t.FromImage("python:" + strings.TrimSpace(version))
}

func (t *Template) FromBunImage(variant string) *Template {
	if strings.TrimSpace(variant) == "" {
		variant = "latest"
	}
	return t.FromImage("oven/bun:" + strings.TrimSpace(variant))
}

func (t *Template) FromUbuntuImage(variant string) *Template {
	if strings.TrimSpace(variant) == "" {
		variant = "latest"
	}
	return t.FromImage("ubuntu:" + strings.TrimSpace(variant))
}

func (t *Template) FromDebianImage(variant string) *Template {
	if strings.TrimSpace(variant) == "" {
		variant = "stable"
	}
	return t.FromImage("debian:" + strings.TrimSpace(variant))
}

func (t *Template) FromAWSRegistry(image, accessKeyID, secretAccessKey, region string) *Template {
	return t.FromImage(image, map[string]any{
		"type":               "aws",
		"awsAccessKeyId":     accessKeyID,
		"awsSecretAccessKey": secretAccessKey,
		"awsRegion":          region,
	})
}

func (t *Template) FromGCPRegistry(image string, serviceAccountJSON any) *Template {
	serviceAccount := ""
	switch value := serviceAccountJSON.(type) {
	case string:
		serviceAccount = value
	default:
		encoded, _ := json.Marshal(value)
		serviceAccount = string(encoded)
	}
	return t.FromImage(image, map[string]any{
		"type":               "gcp",
		"serviceAccountJson": serviceAccount,
	})
}

// Copy adds one local source to the template build context.
func (t *Template) Copy(src, dest string, opts *TemplateCopyOptions) *Template {
	if opts == nil {
		opts = &TemplateCopyOptions{}
	}
	filesHash := strings.TrimSpace(opts.FilesHash)
	if filesHash == "" {
		filesHash = t.registerAutoCopy(src, opts)
	}
	t.builder.Copy(src, dest, filesHash, &build.CopyStepOptions{Force: t.stepForce(nil)})
	if strings.TrimSpace(opts.User) != "" {
		t.builder.Run(buildCopyOwnershipCommand(dest, opts.User), &build.CommandStepOptions{Force: t.stepForce(nil)})
	}
	return t
}

// CopyItems adds multiple local sources with per-item copy options.
func (t *Template) CopyItems(items []TemplateCopyItem) *Template {
	for _, item := range items {
		sources := item.Srcs
		if len(sources) == 0 && strings.TrimSpace(item.Src) != "" {
			sources = []string{item.Src}
		}
		for _, source := range normalizeTemplateItems(sources) {
			t.Copy(source, item.Dest, &TemplateCopyOptions{
				FilesHash:       item.FilesHash,
				ForceUpload:     item.ForceUpload,
				Mode:            item.Mode,
				ResolveSymlinks: item.ResolveSymlinks,
				User:            item.User,
			})
		}
	}
	return t
}

// RunCmd adds one RUN step and optionally wraps it to execute as a specific user.
func (t *Template) RunCmd(command string, opts *TemplateCommandOptions) *Template {
	command = maybeRunAsUser(command, templateCommandUser(opts))
	t.builder.Run(command, &build.CommandStepOptions{Force: templateCommandForce(t, opts)})
	return t
}

// RunCmds adds multiple RUN steps using the same options.
func (t *Template) RunCmds(commands []string, opts *TemplateCommandOptions) *Template {
	for _, command := range commands {
		t.RunCmd(command, opts)
	}
	return t
}

func (t *Template) AptInstall(packages []string, opts *TemplateAptInstallOptions) *Template {
	command := buildAptInstallCommand(packages, opts)
	t.builder.Run(command, &build.CommandStepOptions{Force: templateForce(t, opts)})
	return t
}

func (t *Template) GitClone(repoURL, path string, opts *TemplateGitCloneOptions) *Template {
	command := buildTemplateGitCloneCommand(repoURL, path, opts)
	t.builder.Run(command, &build.CommandStepOptions{Force: templateGitCloneForce(t, opts)})
	return t
}

func (t *Template) MakeDir(paths []string, opts *TemplateMakeDirOptions) *Template {
	for _, path := range normalizeTemplateItems(paths) {
		t.builder.Run(buildMakeDirCommand(path, opts), &build.CommandStepOptions{Force: templateMakeDirForce(t, opts)})
	}
	return t
}

func (t *Template) MakeSymlink(src, dest string, opts *TemplateMakeSymlinkOptions) *Template {
	t.builder.Run(buildMakeSymlinkCommand(src, dest, opts), &build.CommandStepOptions{Force: templateMakeSymlinkForce(t, opts)})
	return t
}

func (t *Template) NpmInstall(packages []string, opts *TemplateNpmInstallOptions) *Template {
	t.builder.Run(buildNpmInstallCommand(packages, opts), &build.CommandStepOptions{Force: templateNpmInstallForce(t, opts)})
	return t
}

func (t *Template) PipInstall(packages []string, opts *TemplatePipInstallOptions) *Template {
	t.builder.Run(buildPipInstallCommand(packages, opts), &build.CommandStepOptions{Force: templatePipInstallForce(t, opts)})
	return t
}

func (t *Template) BunInstall(packages []string, opts *TemplateBunInstallOptions) *Template {
	t.builder.Run(buildBunInstallCommand(packages, opts), &build.CommandStepOptions{Force: templateBunInstallForce(t, opts)})
	return t
}

func (t *Template) SetEnvs(envs map[string]string) *Template {
	t.builder.EnvMap(envs)
	return t
}

func (t *Template) SetWorkdir(path string) *Template {
	t.builder.Workdir(path, &build.CommandStepOptions{Force: t.stepForce(nil)})
	return t
}

func (t *Template) SetUser(user string) *Template {
	t.builder.User(user, &build.CommandStepOptions{Force: t.stepForce(nil)})
	return t
}

func (t *Template) Remove(paths []string, opts *TemplateRemoveOptions) *Template {
	for _, item := range normalizeTemplateItems(paths) {
		t.builder.Run(buildRemoveCommand(item, opts), &build.CommandStepOptions{Force: templateRemoveForce(t, opts)})
	}
	return t
}

func (t *Template) Rename(src, dest string, opts *TemplateRenameOptions) *Template {
	t.builder.Run(buildRenameCommand(src, dest, opts), &build.CommandStepOptions{Force: templateRenameForce(t, opts)})
	return t
}

// SkipCache forces subsequent helper-generated steps to bypass build cache.
func (t *Template) SkipCache() *Template {
	t.skipCache = true
	return t
}

// SetStartCmd records the start command and ready command for the built template.
func (t *Template) SetStartCmd(startCommand string, readyCommand ReadyCmd) *Template {
	t.builder.StartCmd(startCommand)
	t.builder.ReadyCmd(readyCommand.Command())
	return t
}

func (t *Template) registerAutoCopy(source string, opts *TemplateCopyOptions) string {
	token := autoCopyPrefix + strconv.Itoa(len(t.autoCopies)+1)
	var mode *int
	if opts != nil && opts.Mode != nil {
		value := *opts.Mode
		mode = &value
	}
	t.autoCopies[token] = templateAutoCopy{
		Source:       source,
		ForceUpload:  opts != nil && opts.ForceUpload,
		Mode:         mode,
		ResolveLinks: opts != nil && opts.ResolveSymlinks,
	}
	return token
}

func (t *Template) stepForce(force *bool) *bool {
	if force != nil {
		value := *force
		return &value
	}
	if t != nil && t.skipCache {
		value := true
		return &value
	}
	return nil
}

func templateCommandForce(t *Template, opts *TemplateCommandOptions) *bool {
	if opts == nil {
		return t.stepForce(nil)
	}
	return t.stepForce(opts.Force)
}

func templateCommandUser(opts *TemplateCommandOptions) string {
	if opts == nil {
		return ""
	}
	return strings.TrimSpace(opts.User)
}

func (t *Template) SetReadyCmd(readyCommand ReadyCmd) *Template {
	t.builder.ReadyCmd(readyCommand.Command())
	return t
}

func (t *Template) Request() *build.BuildRequest {
	req := t.builder.Request()
	for _, step := range req.Steps {
		if strings.EqualFold(step.Type, "COPY") && strings.HasPrefix(strings.TrimSpace(step.FilesHash), autoCopyPrefix) {
			panic(fmt.Errorf("sandbox: copy steps without FilesHash require BuildTemplate"))
		}
	}
	return req
}

func buildTemplateWithService(
	ctx context.Context,
	buildService *build.Service,
	template *Template,
	name string,
	opts *TemplateBuildOptions,
) (*TemplateBuildInfo, error) {
	if template == nil {
		return nil, core.ErrTemplateEmpty
	}
	if opts == nil {
		opts = &TemplateBuildOptions{}
	}
	templateName, parsedTags, err := parseTemplateName(name)
	if err != nil {
		return nil, err
	}
	tags := dedupeStrings(append(parsedTags, opts.Tags...))
	var extensions *build.PublicTemplateExtensions
	if strings.TrimSpace(opts.BaseTemplateID) != "" {
		extensions = &build.PublicTemplateExtensions{
			BaseTemplateID: strings.TrimSpace(opts.BaseTemplateID),
		}
	}
	created, err := buildService.CreateTemplate(ctx, &build.TemplateCreateRequest{
		Name:       templateName,
		Tags:       tags,
		CPUCount:   opts.CPUCount,
		MemoryMB:   opts.MemoryMB,
		Extensions: extensions,
	})
	if err != nil {
		return nil, err
	}
	buildID := "build-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 16)
	if opts.OnBuildLog != nil {
		opts.OnBuildLog(LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: "Starting build " + buildID})
	}
	request, err := resolveTemplateRequest(ctx, template.builder.Request(), template.autoCopies, created.TemplateID, buildService)
	if err != nil {
		return nil, err
	}
	if _, err := buildService.CreateBuild(ctx, created.TemplateID, buildID, request); err != nil {
		return nil, err
	}
	wait := true
	if opts.Wait != nil {
		wait = *opts.Wait
	}
	if !wait {
		templateResp, err := buildService.GetTemplate(ctx, created.TemplateID, nil)
		if err != nil {
			return nil, err
		}
		return &TemplateBuildInfo{
			TemplateID: created.TemplateID,
			BuildID:    buildID,
			Name:       templateName,
			Tags:       tags,
			Alias:      templateName,
			Status:     "building",
			Template:   templateResp,
		}, nil
	}

	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	logsOffset := 0
	var status *build.BuildStatusResponse
	for {
		status, err = buildService.GetBuildStatus(ctx, created.TemplateID, buildID, &build.BuildStatusParams{
			LogsOffset: &logsOffset,
			Limit:      intPtr(100),
		})
		if err != nil {
			return nil, err
		}
		logsOffset += len(status.LogEntries)
		if opts.OnBuildLog != nil {
			for _, entry := range status.LogEntries {
				opts.OnBuildLog(LogEntry{
					Timestamp: entry.Timestamp,
					Level:     normalizeLogLevel(entry.Level),
					Message:   entry.Step + ": " + entry.Message,
				})
			}
		}
		if isTerminalBuildStatus(status.Status) {
			break
		}
		time.Sleep(pollInterval)
	}
	if opts.OnBuildLog != nil {
		opts.OnBuildLog(LogEntry{Timestamp: time.Now().UTC(), Level: "info", Message: "Build " + buildID + " finished with status " + status.Status})
	}
	templateResp, err := buildService.GetTemplate(ctx, created.TemplateID, nil)
	if err != nil {
		return nil, err
	}
	buildResp, err := buildService.GetBuild(ctx, created.TemplateID, buildID)
	if err != nil {
		return nil, err
	}
	return &TemplateBuildInfo{
		TemplateID: created.TemplateID,
		BuildID:    buildID,
		Name:       templateName,
		Tags:       tags,
		Alias:      templateName,
		Status:     status.Status,
		Template:   templateResp,
		Build:      buildResp,
	}, nil
}

func listTemplatesWithService(
	ctx context.Context,
	buildService *build.Service,
	opts *TemplateListOptions,
) ([]build.ListedTemplate, error) {
	if opts == nil {
		opts = &TemplateListOptions{}
	}
	return buildService.ListTemplates(ctx, &build.ListTemplatesParams{
		Visibility: opts.Visibility,
		Limit:      opts.Limit,
		Offset:     opts.Offset,
	})
}

func getTemplateWithService(
	ctx context.Context,
	buildService *build.Service,
	ref string,
	opts *TemplateGetOptions,
) (*build.TemplateResponse, error) {
	if opts == nil {
		opts = &TemplateGetOptions{}
	}
	templateID := strings.TrimSpace(ref)
	if !strings.HasPrefix(templateID, "tpl-") {
		resolved, err := buildService.ResolveTemplateRef(ctx, templateID)
		if err != nil {
			return nil, err
		}
		templateID = resolved.TemplateID
	}
	return buildService.GetTemplate(ctx, templateID, &build.GetTemplateParams{
		Limit:     opts.Limit,
		NextToken: opts.NextToken,
	})
}

func deleteTemplateWithService(ctx context.Context, buildService *build.Service, ref string) error {
	templateID := strings.TrimSpace(ref)
	if !strings.HasPrefix(templateID, "tpl-") {
		resolved, err := buildService.ResolveTemplateRef(ctx, templateID)
		if err != nil {
			return err
		}
		templateID = resolved.TemplateID
	}
	return buildService.DeleteTemplate(ctx, templateID)
}

func assignTemplateTagsWithService(ctx context.Context, buildService *build.Service, targetName string, tags []string) (*TemplateTagInfo, error) {
	resp, err := buildService.AssignTemplateTags(ctx, &build.AssignTemplateTagsRequest{
		Target: targetName,
		Tags:   tags,
	})
	if err != nil {
		return nil, err
	}
	return &TemplateTagInfo{
		BuildID: strings.TrimSpace(resp.BuildID),
		Tags:    append([]string(nil), resp.Tags...),
	}, nil
}

func getTemplateTagsWithService(ctx context.Context, buildService *build.Service, ref string) ([]TemplateTag, error) {
	templateID := strings.TrimSpace(ref)
	if !strings.HasPrefix(templateID, "tpl-") {
		resolved, err := buildService.ResolveTemplateRef(ctx, templateID)
		if err != nil {
			return nil, err
		}
		templateID = resolved.TemplateID
	}
	tags, err := buildService.ListTemplateTags(ctx, templateID)
	if err != nil {
		return nil, err
	}
	out := make([]TemplateTag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, TemplateTag{
			BuildID:   strings.TrimSpace(tag.BuildID),
			CreatedAt: tag.CreatedAt,
			Tag:       tag.Tag,
		})
	}
	return out, nil
}

func removeTemplateTagsWithService(ctx context.Context, buildService *build.Service, ref string, tags []string) error {
	return buildService.DeleteTemplateTags(ctx, &build.DeleteTemplateTagsRequest{
		Name: ref,
		Tags: tags,
	})
}

func templateExistsWithService(ctx context.Context, buildService *build.Service, ref string) (bool, error) {
	_, err := getTemplateWithService(ctx, buildService, ref, &TemplateGetOptions{})
	if err == nil {
		return true, nil
	}
	var apiErr *core.APIError
	if errors.As(err, &apiErr) && apiErr.Kind == core.APIErrorKindNotFound {
		return false, nil
	}
	return false, err
}

func getTemplateBuildStatusWithService(
	ctx context.Context,
	buildService *build.Service,
	templateID, buildID string,
	opts *TemplateBuildStatusOptions,
) (*build.BuildStatusResponse, error) {
	if opts == nil {
		opts = &TemplateBuildStatusOptions{}
	}
	return buildService.GetBuildStatus(ctx, strings.TrimSpace(templateID), strings.TrimSpace(buildID), &build.BuildStatusParams{
		LogsOffset: opts.LogsOffset,
		Limit:      opts.Limit,
		Level:      firstNonEmpty(opts.Level),
	})
}

func getTemplateForTagMutationWithService(ctx context.Context, buildService *build.Service, ref string) (*build.TemplateResponse, error) {
	templateResp, err := getTemplateWithService(ctx, buildService, ref, nil)
	if err == nil {
		return templateResp, nil
	}
	var apiErr *core.APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != core.APIErrorKindNotFound {
		return nil, err
	}
	name, _, parseErr := parseTemplateName(ref)
	if parseErr != nil || strings.TrimSpace(name) == strings.TrimSpace(ref) {
		return nil, err
	}
	return getTemplateWithService(ctx, buildService, name, nil)
}

// TemplateToJSON serializes the currently supported template subset into build-request JSON.
func TemplateToJSON(template *Template, computeHashes bool) (string, error) {
	request, err := serializeTemplateRequestForJSON(template, computeHashes)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TemplateToDockerfile converts the currently supported template subset into a Dockerfile string.
func TemplateToDockerfile(template *Template) (string, error) {
	if template == nil {
		return "", core.ErrTemplateEmpty
	}
	request := template.builder.Request()
	if strings.TrimSpace(request.FromTemplate) != "" {
		return "", fmt.Errorf("sandbox: templates based on other templates cannot be converted to Dockerfile")
	}
	if strings.TrimSpace(request.FromImage) == "" {
		return "", fmt.Errorf("sandbox: template must define a base image to convert to Dockerfile")
	}
	lines := []string{"FROM " + request.FromImage}
	for _, step := range request.Steps {
		switch step.Type {
		case "COPY":
			if len(step.Args) >= 2 {
				lines = append(lines, "COPY "+step.Args[0]+" "+step.Args[1])
			}
		case "RUN":
			if len(step.Args) >= 1 {
				lines = append(lines, "RUN "+step.Args[0])
			}
		case "ENV":
			lines = append(lines, dockerfileEnvLines(step.Args)...)
		case "WORKDIR":
			if len(step.Args) >= 1 {
				lines = append(lines, "WORKDIR "+step.Args[0])
			}
		case "USER":
			if len(step.Args) >= 1 {
				lines = append(lines, "USER "+step.Args[0])
			}
		}
	}
	if strings.TrimSpace(request.StartCmd) != "" {
		encoded, _ := json.Marshal(request.StartCmd)
		lines = append(lines, `CMD ["sh", "-lc", `+string(encoded)+`]`)
	}
	if strings.TrimSpace(request.ReadyCmd) != "" {
		lines = append(lines, "# Ready command: "+request.ReadyCmd)
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func WaitForFile(filename string) ReadyCmd {
	return ReadyCmd{cmd: "test -f " + shellQuote(filename)}
}

func WaitForPort(port int) ReadyCmd {
	return ReadyCmd{cmd: "sh -lc \"ss -ltn | grep -q ':" + strconv.Itoa(port) + " '\""}
}

func WaitForProcess(processName string) ReadyCmd {
	return ReadyCmd{cmd: "pgrep -f " + shellQuote(processName) + " >/dev/null"}
}

func WaitForTimeout(timeout time.Duration) ReadyCmd {
	seconds := int(timeout.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return ReadyCmd{cmd: "sleep " + strconv.Itoa(seconds)}
}

func WaitForURL(rawURL string, statusCode int) ReadyCmd {
	if statusCode == 0 {
		statusCode = 200
	}
	return ReadyCmd{cmd: "test \"$(curl -o /dev/null -s -w '%{http_code}' " + shellQuote(rawURL) + ")\" = \"" + strconv.Itoa(statusCode) + "\""}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseTemplateName(name string) (string, []string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", nil, core.ErrTemplateEmpty
	}
	lastColon := strings.LastIndex(trimmed, ":")
	if lastColon < 0 {
		return trimmed, nil, nil
	}
	baseName := strings.TrimSpace(trimmed[:lastColon])
	tag := strings.TrimSpace(trimmed[lastColon+1:])
	if baseName == "" || tag == "" {
		return "", nil, core.ErrTemplateEmpty
	}
	return baseName, []string{tag}, nil
}

func resolveTemplateRefID(ctx context.Context, buildService *build.Service, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", core.ErrTemplateEmpty
	}
	if strings.HasPrefix(trimmed, "tpl-") {
		return trimmed, nil
	}
	resolved, err := buildService.ResolveTemplateRef(ctx, trimmed)
	if err != nil {
		return "", err
	}
	return resolved.TemplateID, nil
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeTemplateTagInput(values []string) ([]string, error) {
	tags := dedupeStrings(values)
	if len(tags) == 0 {
		return nil, fmt.Errorf("sandbox: tags are required")
	}
	return tags, nil
}

func normalizeWriteData(data any) ([]byte, error) {
	switch value := data.(type) {
	case nil:
		return nil, fmt.Errorf("sandbox: data is required")
	case []byte:
		return value, nil
	case string:
		return []byte(value), nil
	case io.Reader:
		return io.ReadAll(value)
	default:
		return nil, fmt.Errorf("sandbox: write data must be a string, []byte, or io.Reader")
	}
}

func normalizeWriteInfo(filePath string) *WriteInfo {
	return &WriteInfo{
		Name: path.Base(filePath),
		Path: filePath,
		Type: FileTypeFile,
	}
}

func normalizeEntryInfo(entry cmd.EntryInfo) *EntryInfo {
	var modifiedTime *time.Time
	if strings.TrimSpace(entry.ModifiedTime) != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, entry.ModifiedTime); err == nil {
			modifiedTime = &parsed
		}
	}
	return &EntryInfo{
		Name:          entry.Name,
		Type:          normalizeFileType(entry.Type),
		Path:          entry.Path,
		Size:          entry.Size,
		Mode:          entry.Mode,
		Permissions:   entry.Permissions,
		Owner:         entry.Owner,
		Group:         entry.Group,
		ModifiedTime:  modifiedTime,
		SymlinkTarget: entry.SymlinkTarget,
	}
}

func normalizeFilesystemEvent(event cmd.FilesystemEvent) *FilesystemEvent {
	return &FilesystemEvent{
		Name: event.Name,
		Type: normalizeFilesystemEventType(event.Type),
	}
}

func normalizeFileType(value cmd.FileType) FileType {
	switch value {
	case cmd.FileTypeDirectory:
		return FileTypeDir
	case cmd.FileTypeSymlink:
		return FileTypeSymlink
	default:
		return FileTypeFile
	}
}

func normalizeFilesystemEventType(value cmd.EventType) FilesystemEventType {
	switch value {
	case cmd.EventTypeCreate:
		return FilesystemEventCreate
	case cmd.EventTypeRemove:
		return FilesystemEventRemove
	case cmd.EventTypeRename:
		return FilesystemEventRename
	case cmd.EventTypeChmod:
		return FilesystemEventChmod
	default:
		return FilesystemEventWrite
	}
}

func isTerminalBuildStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ready", "failed", "error", "cancelled":
		return true
	default:
		return false
	}
}

func normalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "warn", "error":
		return strings.ToLower(strings.TrimSpace(level))
	default:
		return "info"
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func intPtr(value int) *int {
	return &value
}

func gitCommandOptions(opts *GitCloneOptions) *GitCommandOptions {
	if opts == nil {
		return &GitCommandOptions{}
	}
	return &GitCommandOptions{
		Envs:      cloneStringMap(opts.Envs),
		CWD:       opts.CWD,
		TimeoutMS: opts.TimeoutMS,
		User:      opts.User,
	}
}

func cloneGitCommandOptions(opts *GitCommandOptions) GitCommandOptions {
	if opts == nil {
		return GitCommandOptions{}
	}
	return GitCommandOptions{
		Envs:      cloneStringMap(opts.Envs),
		CWD:       opts.CWD,
		TimeoutMS: opts.TimeoutMS,
		User:      opts.User,
	}
}

func resolvePositiveRuntimeTimeoutMilliseconds(timeoutMS *int64) (*int64, error) {
	if timeoutMS == nil {
		return nil, nil
	}
	value := *timeoutMS
	if value <= 0 {
		return nil, fmt.Errorf("sandbox: timeoutMs must be a positive number")
	}
	return &value, nil
}

func buildGitExecution(subcommand string, args []string, user string) (string, []string) {
	gitArgs := append([]string{subcommand}, args...)
	if user == "" {
		return "git", gitArgs
	}
	return "sh", []string{
		"-lc",
		"su -s /bin/sh " + shellQuote(user) + " -c " + shellQuote(shellJoin(append([]string{"git"}, gitArgs...))),
	}
}

func buildCommandExecution(command string, args []string, user string) (string, []string) {
	trimmedUser := strings.TrimSpace(user)
	if trimmedUser == "" {
		return command, args
	}
	return "sh", []string{
		"-lc",
		"su -s /bin/sh " + shellQuote(trimmedUser) + " -c " + shellQuote(shellJoin(append([]string{command}, args...))),
	}
}

func buildAptInstallCommand(packages []string, opts *TemplateAptInstallOptions) string {
	names := normalizeTemplateItems(packages)
	installArgs := []string{"apt-get", "install", "-y"}
	if opts != nil && opts.NoInstallRecommends {
		installArgs = append(installArgs, "--no-install-recommends")
	}
	installArgs = append(installArgs, names...)
	return shellJoin([]string{"apt-get", "update"}) + " && DEBIAN_FRONTEND=noninteractive " + shellJoin(installArgs)
}

func buildTemplateGitCloneCommand(repoURL, path string, opts *TemplateGitCloneOptions) string {
	trimmedURL := strings.TrimSpace(repoURL)
	args := []string{"git", "clone"}
	if opts != nil && strings.TrimSpace(opts.Branch) != "" {
		args = append(args, "--branch", strings.TrimSpace(opts.Branch))
	}
	if opts != nil && opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, trimmedURL)
	if strings.TrimSpace(path) != "" {
		args = append(args, strings.TrimSpace(path))
	}
	command := shellJoin(args)
	if opts == nil || strings.TrimSpace(opts.User) == "" {
		return command
	}
	return "su -s /bin/sh " + shellQuote(strings.TrimSpace(opts.User)) + " -c " + shellQuote(command)
}

func buildMakeDirCommand(path string, opts *TemplateMakeDirOptions) string {
	args := []string{"mkdir", "-p"}
	if opts != nil && opts.Mode != nil {
		args = append(args, "-m", strconv.FormatInt(int64(*opts.Mode), 8))
	}
	args = append(args, path)
	return maybeRunAsUser(shellJoin(args), templateMakeDirUser(opts))
}

func buildCopyOwnershipCommand(path, user string) string {
	return shellJoin([]string{"chown", "-R", strings.TrimSpace(user), strings.TrimSpace(path)})
}

func buildMakeSymlinkCommand(src, dest string, opts *TemplateMakeSymlinkOptions) string {
	args := []string{"ln", "-s"}
	if opts != nil && opts.Force != nil && *opts.Force {
		args = append(args, "-f")
	}
	args = append(args, src, dest)
	return maybeRunAsUser(shellJoin(args), templateMakeSymlinkUser(opts))
}

func buildRemoveCommand(target string, opts *TemplateRemoveOptions) string {
	args := []string{"rm"}
	if opts != nil && opts.Recursive {
		args = append(args, "-r")
	}
	if opts != nil && opts.Force != nil && *opts.Force {
		args = append(args, "-f")
	}
	args = append(args, target)
	user := ""
	if opts != nil {
		user = opts.User
	}
	return maybeRunAsUser(shellJoin(args), user)
}

func buildRenameCommand(src, dest string, opts *TemplateRenameOptions) string {
	args := []string{"mv"}
	if opts != nil && opts.Force != nil && *opts.Force {
		args = append(args, "-f")
	}
	args = append(args, src, dest)
	user := ""
	if opts != nil {
		user = opts.User
	}
	return maybeRunAsUser(shellJoin(args), user)
}

func dockerfileEnvLines(args []string) []string {
	lines := make([]string, 0, len(args)/2)
	for index := 0; index < len(args); index += 2 {
		name := args[index]
		value := ""
		if index+1 < len(args) {
			value = args[index+1]
		}
		if name == "" {
			continue
		}
		encoded, _ := json.Marshal(value)
		lines = append(lines, "ENV "+name+"="+string(encoded))
	}
	return lines
}

func buildNpmInstallCommand(packages []string, opts *TemplateNpmInstallOptions) string {
	args := []string{"npm", "install"}
	if opts != nil && opts.Dev {
		args = append(args, "--save-dev")
	}
	if opts != nil && opts.G {
		args = append(args, "-g")
	}
	args = append(args, normalizeTemplateItems(packages)...)
	return shellJoin(args)
}

func buildPipInstallCommand(packages []string, opts *TemplatePipInstallOptions) string {
	args := []string{"pip", "install"}
	global := true
	if opts != nil && opts.G != nil {
		global = *opts.G
	}
	if !global {
		args = append(args, "--user")
	}
	names := normalizeTemplateItems(packages)
	if len(names) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, names...)
	}
	return shellJoin(args)
}

func buildBunInstallCommand(packages []string, opts *TemplateBunInstallOptions) string {
	args := []string{"bun", "install"}
	if opts != nil && opts.Dev {
		args = append(args, "--dev")
	}
	if opts != nil && opts.G {
		args = append(args, "-g")
	}
	args = append(args, normalizeTemplateItems(packages)...)
	return shellJoin(args)
}

type dockerfileInstruction struct {
	Name  string
	Value string
}

func resolveDockerfileInput(dockerfileContentOrPath string) (string, string, error) {
	raw := dockerfileContentOrPath
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", fmt.Errorf("sandbox: dockerfile content or path is required")
	}
	resolvedPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", "", err
	}
	if !strings.Contains(trimmed, "\n") {
		if info, statErr := os.Stat(resolvedPath); statErr == nil {
			if info.IsDir() {
				return "", "", fmt.Errorf("sandbox: dockerfile path must point to a file")
			}
			data, readErr := os.ReadFile(resolvedPath)
			if readErr != nil {
				return "", "", readErr
			}
			return string(data), filepath.Dir(resolvedPath), nil
		}
	}
	return raw, "", nil
}

func parseDockerfileInstructions(content string) []dockerfileInstruction {
	lines := joinDockerfileLines(content)
	out := make([]dockerfileInstruction, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name := trimmed
		value := ""
		if index := strings.IndexFunc(trimmed, func(r rune) bool { return r == ' ' || r == '\t' }); index >= 0 {
			name = trimmed[:index]
			value = strings.TrimSpace(trimmed[index+1:])
		}
		out = append(out, dockerfileInstruction{
			Name:  strings.ToUpper(name),
			Value: value,
		})
	}
	return out
}

func joinDockerfileLines(content string) []string {
	rawLines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	current := ""
	for _, rawLine := range rawLines {
		trimmedRight := strings.TrimRight(rawLine, " \t")
		if strings.HasSuffix(trimmedRight, "\\") {
			current += strings.TrimSuffix(trimmedRight, "\\") + " "
			continue
		}
		current += trimmedRight
		lines = append(lines, current)
		current = ""
	}
	if strings.TrimSpace(current) != "" {
		lines = append(lines, current)
	}
	return lines
}

func ensureDockerfileBaseImage(seenFrom bool) error {
	if !seenFrom {
		return fmt.Errorf("sandbox: Dockerfile instructions must appear after FROM")
	}
	return nil
}

func requireDockerfileValue(instruction, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("sandbox: %s requires a value", instruction)
	}
	return trimmed, nil
}

func parseDockerfileEnv(value string) ([][2]string, error) {
	trimmed, err := requireDockerfileValue("ENV", value)
	if err != nil {
		return nil, err
	}
	tokens, err := tokenizeShellLike(trimmed)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("sandbox: ENV requires at least one variable")
	}
	hasAssignments := false
	for _, token := range tokens {
		if strings.Contains(token, "=") {
			hasAssignments = true
			break
		}
	}
	if hasAssignments {
		pairs := make([][2]string, 0, len(tokens))
		for _, token := range tokens {
			separator := strings.Index(token, "=")
			if separator <= 0 {
				return nil, fmt.Errorf("sandbox: invalid ENV assignment: %s", token)
			}
			pairs = append(pairs, [2]string{token[:separator], token[separator+1:]})
		}
		return pairs, nil
	}
	if len(tokens) < 2 {
		return nil, fmt.Errorf("sandbox: ENV requires a key and value")
	}
	valueIndex := strings.Index(trimmed, tokens[1])
	return [][2]string{{tokens[0], stripMatchingQuotes(trimmed[valueIndex:])}}, nil
}

func parseDockerfileCopy(value string) ([]string, string, error) {
	trimmed, err := requireDockerfileValue("COPY", value)
	if err != nil {
		return nil, "", err
	}
	if strings.HasPrefix(trimmed, "--") {
		return nil, "", fmt.Errorf("sandbox: COPY flags are not supported")
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []string
		if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
			return nil, "", fmt.Errorf("sandbox: invalid COPY JSON array: %w", err)
		}
		if len(items) < 2 {
			return nil, "", fmt.Errorf("sandbox: COPY JSON array must contain at least one source and one destination")
		}
		values := make([]string, 0, len(items))
		for _, item := range items {
			value := strings.TrimSpace(item)
			if value == "" {
				return nil, "", fmt.Errorf("sandbox: COPY JSON array entries must be non-empty")
			}
			values = append(values, value)
		}
		return values[:len(values)-1], values[len(values)-1], nil
	}
	tokens, err := tokenizeShellLike(trimmed)
	if err != nil {
		return nil, "", err
	}
	if len(tokens) < 2 {
		return nil, "", fmt.Errorf("sandbox: COPY requires at least one source and one destination")
	}
	for _, token := range tokens {
		if strings.HasPrefix(token, "--") {
			return nil, "", fmt.Errorf("sandbox: COPY flags are not supported")
		}
	}
	return tokens[:len(tokens)-1], tokens[len(tokens)-1], nil
}

func parseDockerfileCmd(value string) (string, error) {
	trimmed, err := requireDockerfileValue("CMD", value)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(trimmed, "[") {
		return trimmed, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return "", fmt.Errorf("sandbox: invalid CMD JSON array: %w", err)
	}
	if len(items) == 0 {
		return "", fmt.Errorf("sandbox: CMD JSON array must contain one or more strings")
	}
	return shellJoin(items), nil
}

func resolveDockerfileCopyPath(source, contextDir string) string {
	if contextDir == "" || filepath.IsAbs(source) {
		return source
	}
	return filepath.Join(contextDir, source)
}

func tokenizeShellLike(value string) ([]string, error) {
	tokens := make([]string, 0, 4)
	var current strings.Builder
	quote := byte(0)
	escaping := false

	flush := func() {
		if current.Len() > 0 {
			tokens = append(tokens, current.String())
			current.Reset()
		}
	}

	for index := 0; index < len(value); index++ {
		char := value[index]
		if escaping {
			current.WriteByte(char)
			escaping = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaping = true
			continue
		}
		if char == '\'' || char == '"' {
			if quote == 0 {
				quote = char
				continue
			}
			if quote == char {
				quote = 0
				continue
			}
		}
		if quote == 0 && (char == ' ' || char == '\t') {
			flush()
			continue
		}
		current.WriteByte(char)
	}
	if escaping || quote != 0 {
		return nil, fmt.Errorf("sandbox: unterminated Dockerfile quoted value")
	}
	flush()
	return tokens, nil
}

func stripMatchingQuotes(value string) string {
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func resolveTemplateRequest(
	ctx context.Context,
	request *build.BuildRequest,
	autoCopies map[string]templateAutoCopy,
	templateID string,
	buildService *build.Service,
) (*build.BuildRequest, error) {
	if request == nil {
		return nil, nil
	}
	resolved := *request
	if len(request.Steps) > 0 {
		resolved.Steps = make([]build.BuildStep, 0, len(request.Steps))
	}
	uploaded := map[string]struct{}{}
	for _, step := range request.Steps {
		cloned := cloneFacadeBuildStep(step)
		if !strings.EqualFold(strings.TrimSpace(cloned.Type), "COPY") || !strings.HasPrefix(strings.TrimSpace(cloned.FilesHash), autoCopyPrefix) {
			resolved.Steps = append(resolved.Steps, cloned)
			continue
		}
		copySpec, ok := autoCopies[strings.TrimSpace(cloned.FilesHash)]
		if !ok {
			return nil, fmt.Errorf("sandbox: unknown copy token %s", cloned.FilesHash)
		}
		archivePath, err := normalizeArchiveSource(copySpec.Source)
		if err != nil {
			return nil, err
		}
		if len(cloned.Args) > 0 {
			cloned.Args[0] = archivePath
		}
		tarBytes, err := packTemplateSource(copySpec.Source, archivePath, copySpec)
		if err != nil {
			return nil, err
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(tarBytes))
		if _, ok := uploaded[hash]; !ok {
			resp, err := buildService.GetBuildFile(ctx, templateID, hash)
			if err != nil {
				return nil, err
			}
			if !resp.Present || copySpec.ForceUpload {
				if strings.TrimSpace(resp.URL) == "" {
					return nil, fmt.Errorf("sandbox: build file upload URL is missing for hash %s", hash)
				}
				if err := uploadBuildFile(ctx, buildService, resp.URL, tarBytes); err != nil {
					return nil, err
				}
			}
			uploaded[hash] = struct{}{}
		}
		cloned.FilesHash = hash
		resolved.Steps = append(resolved.Steps, cloned)
	}
	return &resolved, nil
}

func serializeTemplateRequestForJSON(template *Template, computeHashes bool) (*build.BuildRequest, error) {
	if template == nil {
		return nil, core.ErrTemplateEmpty
	}
	request := template.builder.Request()
	if !computeHashes {
		return request, nil
	}
	resolved := *request
	if len(request.Steps) > 0 {
		resolved.Steps = make([]build.BuildStep, 0, len(request.Steps))
	}
	for _, step := range request.Steps {
		cloned := cloneFacadeBuildStep(step)
		if !strings.EqualFold(strings.TrimSpace(cloned.Type), "COPY") || !strings.HasPrefix(strings.TrimSpace(cloned.FilesHash), autoCopyPrefix) {
			resolved.Steps = append(resolved.Steps, cloned)
			continue
		}
		copySpec, ok := template.autoCopies[strings.TrimSpace(cloned.FilesHash)]
		if !ok {
			return nil, fmt.Errorf("sandbox: unknown copy token %s", cloned.FilesHash)
		}
		archivePath, err := normalizeArchiveSource(copySpec.Source)
		if err != nil {
			return nil, err
		}
		tarBytes, err := packTemplateSource(copySpec.Source, archivePath, copySpec)
		if err != nil {
			return nil, err
		}
		cloned.FilesHash = fmt.Sprintf("%x", sha256.Sum256(tarBytes))
		resolved.Steps = append(resolved.Steps, cloned)
	}
	return &resolved, nil
}

func cloneFacadeBuildStep(step build.BuildStep) build.BuildStep {
	cloned := step
	if len(step.Args) > 0 {
		cloned.Args = append([]string(nil), step.Args...)
	}
	if step.Force != nil {
		force := *step.Force
		cloned.Force = &force
	}
	return cloned
}

func packTemplateSource(source, archivePath string, opts templateAutoCopy) ([]byte, error) {
	absoluteSource, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(gzipWriter)
	if err := appendTarEntry(writer, absoluteSource, archivePath, opts); err != nil {
		_ = writer.Close()
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func appendTarEntry(writer *tar.Writer, diskPath, archivePath string, opts templateAutoCopy) error {
	info, err := os.Lstat(diskPath)
	if opts.ResolveLinks {
		info, err = os.Stat(diskPath)
	}
	if err != nil {
		return err
	}
	normalizedArchivePath := strings.TrimLeft(filepath.ToSlash(archivePath), "/")
	if normalizedArchivePath == "" {
		return fmt.Errorf("sandbox: copy source path must not resolve to an empty archive path")
	}
	entryMode := info.Mode().Perm()
	if opts.Mode != nil {
		entryMode = os.FileMode(*opts.Mode)
	}
	switch mode := info.Mode(); {
	case mode.IsDir():
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = ensureTrailingSlash(normalizedArchivePath)
		header.ModTime = time.Unix(0, 0).UTC()
		header.AccessTime = time.Unix(0, 0).UTC()
		header.ChangeTime = time.Unix(0, 0).UTC()
		header.Uid = 0
		header.Gid = 0
		header.Uname = "root"
		header.Gname = "root"
		header.Mode = int64(entryMode)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		entries, err := os.ReadDir(diskPath)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := appendTarEntry(writer, filepath.Join(diskPath, entry.Name()), path.Join(normalizedArchivePath, entry.Name()), opts); err != nil {
				return err
			}
		}
		return nil
	case mode&os.ModeSymlink != 0 && !opts.ResolveLinks:
		linkTarget, err := os.Readlink(diskPath)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return err
		}
		header.Name = normalizedArchivePath
		header.ModTime = time.Unix(0, 0).UTC()
		header.AccessTime = time.Unix(0, 0).UTC()
		header.ChangeTime = time.Unix(0, 0).UTC()
		header.Uid = 0
		header.Gid = 0
		header.Uname = "root"
		header.Gname = "root"
		header.Mode = int64(entryMode)
		return writer.WriteHeader(header)
	case mode.IsRegular():
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = normalizedArchivePath
		header.ModTime = time.Unix(0, 0).UTC()
		header.AccessTime = time.Unix(0, 0).UTC()
		header.ChangeTime = time.Unix(0, 0).UTC()
		header.Uid = 0
		header.Gid = 0
		header.Uname = "root"
		header.Gname = "root"
		header.Mode = int64(entryMode)
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(diskPath)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	default:
		return fmt.Errorf("sandbox: unsupported copy source type for %s", diskPath)
	}
}

func normalizeArchiveSource(source string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", fmt.Errorf("sandbox: copy source path is required")
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Base(trimmed), nil
	}
	normalized := filepath.ToSlash(trimmed)
	if strings.HasPrefix(normalized, "./") {
		normalized = strings.TrimPrefix(normalized, "./")
	}
	normalized = strings.TrimLeft(normalized, "/")
	if normalized == "" {
		return filepath.Base(trimmed), nil
	}
	return normalized, nil
}

func ensureTrailingSlash(value string) string {
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

func uploadBuildFile(ctx context.Context, buildService *build.Service, rawURL string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, rawURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := buildService.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("sandbox: build file upload failed with status %d", resp.StatusCode)
	}
	return nil
}

func normalizeTemplateItems(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func templateForce(t *Template, opts *TemplateAptInstallOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateGitCloneForce(t *Template, opts *TemplateGitCloneOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateMakeDirForce(t *Template, opts *TemplateMakeDirOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateMakeDirUser(opts *TemplateMakeDirOptions) string {
	if opts == nil {
		return ""
	}
	return opts.User
}

func templateMakeSymlinkForce(t *Template, opts *TemplateMakeSymlinkOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateMakeSymlinkUser(opts *TemplateMakeSymlinkOptions) string {
	if opts == nil {
		return ""
	}
	return opts.User
}

func templateNpmInstallForce(t *Template, opts *TemplateNpmInstallOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templatePipInstallForce(t *Template, opts *TemplatePipInstallOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateBunInstallForce(t *Template, opts *TemplateBunInstallOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateRemoveForce(t *Template, opts *TemplateRemoveOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func templateRenameForce(t *Template, opts *TemplateRenameOptions) *bool {
	if opts != nil && opts.Force != nil {
		return t.stepForce(opts.Force)
	}
	return t.stepForce(nil)
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func shellJoin(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func maybeRunAsUser(command, user string) string {
	if strings.TrimSpace(user) == "" {
		return command
	}
	return "su -s /bin/sh " + shellQuote(strings.TrimSpace(user)) + " -c " + shellQuote(command)
}

func filesystemRequestOptions(opts *FilesystemRequestOptions) *cmd.RequestOptions {
	if opts == nil || strings.TrimSpace(opts.User) == "" {
		return nil
	}
	return &cmd.RequestOptions{Username: strings.TrimSpace(opts.User)}
}

func filesystemListOptions(opts *FilesystemListOptions) *cmd.RequestOptions {
	if opts == nil || strings.TrimSpace(opts.User) == "" {
		return nil
	}
	return &cmd.RequestOptions{Username: strings.TrimSpace(opts.User)}
}

func watchDirRequestOptions(opts *WatchDirOptions) *cmd.RequestOptions {
	if opts == nil || strings.TrimSpace(opts.User) == "" {
		return nil
	}
	return &cmd.RequestOptions{Username: strings.TrimSpace(opts.User)}
}

func (h *CommandHandle) Close() error {
	if h == nil || h.stream == nil {
		return nil
	}
	return h.stream.Close()
}

func (h *CommandHandle) SendStdin(ctx context.Context, data string) error {
	if h == nil {
		return fmt.Errorf("sandbox: command handle is nil")
	}
	input := cmd.ProcessInput{Stdin: encodeStreamData(data)}
	if h.PTY {
		input = cmd.ProcessInput{PTY: encodeStreamData(data)}
	}
	return h.runtime.SendInput(ctx, &cmd.SendInputRequest{
		Process: cmd.ProcessSelector{PID: h.PID},
		Input:   input,
	}, nil)
}

func (h *CommandHandle) SendInput(ctx context.Context, data string) error {
	return h.SendStdin(ctx, data)
}

func (h *CommandHandle) CloseStdin(ctx context.Context) error {
	if h == nil {
		return fmt.Errorf("sandbox: command handle is nil")
	}
	return h.runtime.CloseStdin(ctx, &cmd.CloseStdinRequest{
		Process: cmd.ProcessSelector{PID: h.PID},
	}, nil)
}

func (h *CommandHandle) Kill(ctx context.Context) (bool, error) {
	if h == nil {
		return false, fmt.Errorf("sandbox: command handle is nil")
	}
	err := h.runtime.SendSignal(ctx, &cmd.SendSignalRequest{
		Process: cmd.ProcessSelector{PID: h.PID},
		Signal:  string(cmd.SignalSIGKILL),
	}, nil)
	if isMissingProcessError(err) {
		return false, nil
	}
	return err == nil, err
}

func (h *CommandHandle) Wait(ctx context.Context) (*CommandWaitResult, error) {
	if h == nil || h.stream == nil {
		return nil, fmt.Errorf("sandbox: command handle is nil")
	}
	result := &CommandWaitResult{}
	for {
		frame, err := h.stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if frame.Event.Data != nil {
			stdoutChunk := decodeStreamData(frame.Event.Data.Stdout)
			stderrChunk := decodeStreamData(frame.Event.Data.Stderr)
			ptyChunk := decodeStreamData(frame.Event.Data.PTY)
			result.Stdout += stdoutChunk
			result.Stderr += stderrChunk
			result.PTY += ptyChunk
			if stdoutChunk != "" && h.onStdout != nil {
				h.onStdout(stdoutChunk)
			}
			if stderrChunk != "" && h.onStderr != nil {
				h.onStderr(stderrChunk)
			}
			// Some runtimes stream PTY reconnect output through stdout/stderr instead of PTY.
			if h.PTY && ptyChunk == "" {
				result.PTY += stdoutChunk + stderrChunk
			}
		}
		if frame.Event.End != nil {
			break
		}
	}
	if h.CmdID != "" {
		cmdResult, err := getResultWithRetry(ctx, h.runtime, h.CmdID)
		if err != nil {
			return nil, err
		}
		result.Stdout = cmdResult.Stdout
		result.Stderr = cmdResult.Stderr
		result.ExitCode = cmdResult.ExitCode
	}
	return result, nil
}

func expectStartHandle(stream *cmd.ProcessStream, runtime *Runtime, pty bool) (*CommandHandle, error) {
	for {
		frame, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("sandbox: process stream ended before start frame")
			}
			return nil, err
		}
		if frame.Event.Start != nil {
			return &CommandHandle{
				runtime: runtime,
				stream:  stream,
				PID:     frame.Event.Start.PID,
				CmdID:   frame.Event.Start.CmdID,
				PTY:     pty,
			}, nil
		}
	}
}

func encodeStreamData(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

func decodeStreamData(data string) string {
	if strings.TrimSpace(data) == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return data
	}
	return string(decoded)
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func runtimeProxyURL(baseURL string, port int) (string, error) {
	if port <= 0 {
		return "", fmt.Errorf("sandbox: port must be a positive integer")
	}
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/proxy/" + strconv.Itoa(port) + "/"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func runtimeFileURL(baseURL, accessToken, filePath, operation string, opts *SandboxURLOptions) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/files"
	parsed.Fragment = ""
	query := parsed.Query()
	if strings.TrimSpace(filePath) != "" {
		query.Set("path", strings.TrimSpace(filePath))
	}
	username := ""
	if opts != nil {
		username = strings.TrimSpace(opts.User)
		if username != "" {
			query.Set("username", username)
		}
	}
	secret := strings.TrimSpace(accessToken)
	if secret != "" {
		var expiration *int64
		if opts != nil && opts.UseSignatureExpiration != nil {
			if *opts.UseSignatureExpiration <= 0 {
				return "", fmt.Errorf("sandbox: UseSignatureExpiration must be a positive integer")
			}
			expiration = opts.UseSignatureExpiration
			query.Set("signature_expiration", strconv.FormatInt(*expiration, 10))
		}
		query.Set("signature", signFileURL(strings.TrimSpace(filePath), operation, username, secret, expiration))
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func signFileURL(filePath, operation, username, secret string, expiration *int64) string {
	raw := filePath + ":" + operation + ":" + username + ":" + secret
	if expiration != nil {
		raw += ":" + strconv.FormatInt(*expiration, 10)
	}
	sum := sha256.Sum256([]byte(raw))
	return "v1_" + strings.TrimRight(base64.StdEncoding.EncodeToString(sum[:]), "=")
}

func isMissingProcessError(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *core.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Kind == core.APIErrorKindNotFound {
		return true
	}
	message := strings.ToLower(apiErr.Error() + " " + string(apiErr.Body))
	return strings.Contains(message, "no such process") || strings.Contains(message, "esrch")
}
