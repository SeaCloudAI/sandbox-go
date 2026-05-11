package sandbox

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/core"
)

const codeFileBase = "/root/workspace/.seacloud-code-interpreter"
const codeContextFileBase = "/root/workspace/.seacloud-code-context"
const codeContextPayloadPrefix = "__SEACLOUD_CODE_CONTEXT__"

type CodeOutputChunk struct {
	Error     bool   `json:"error"`
	Line      string `json:"line"`
	Timestamp int64  `json:"timestamp"`
}

type CodeExecutionError struct {
	Name      string `json:"name,omitempty"`
	Message   string `json:"message"`
	Traceback string `json:"traceback,omitempty"`
}

type CodeExecutionResult struct {
	Text  string         `json:"text,omitempty"`
	PNG   string         `json:"png,omitempty"`
	Chart map[string]any `json:"chart,omitempty"`
	JSON  any            `json:"json,omitempty"`
}

type CodeExecutionLogs struct {
	Stdout []string `json:"stdout"`
	Stderr []string `json:"stderr"`
}

type CodeExecution struct {
	Results        []CodeExecutionResult `json:"results"`
	Logs           CodeExecutionLogs     `json:"logs"`
	Error          *CodeExecutionError   `json:"error,omitempty"`
	ExecutionCount int                   `json:"executionCount"`
}

func (e *CodeExecution) Text() string {
	if e == nil {
		return ""
	}
	texts := make([]string, 0, len(e.Results))
	for _, result := range e.Results {
		if strings.TrimSpace(result.Text) != "" {
			texts = append(texts, result.Text)
		}
	}
	if len(texts) > 0 {
		return strings.Join(texts, "\n")
	}
	return strings.Join(append(append([]string{}, e.Logs.Stdout...), e.Logs.Stderr...), "")
}

type CodeContext struct {
	ContextID string `json:"contextId"`
	CWD       string `json:"cwd,omitempty"`
	Language  string `json:"language"`
	Timeout   *int   `json:"timeout,omitempty"`
}

type CodeContextCreateOptions struct {
	CWD      string
	Language string
	Timeout  *int
}

type RunCodeOptions struct {
	Language  string
	CWD       string
	Envs      map[string]string
	Timeout   *int
	Context   *CodeContext
	OnStdout  func(CodeOutputChunk)
	OnStderr  func(CodeOutputChunk)
	OnResult  func(CodeExecutionResult)
	OnResults func(CodeExecutionResult)
	OnError   func(CodeExecutionError)
}

type codeLanguageSpec struct {
	extension   string
	command     string
	args        func(scriptPath string) []string
	buildSource func(code, resultPath string) string
	resultFile  bool
}

type PythonCodeContextSession struct {
	runtime        *Runtime
	context        *CodeContext
	defaultContext bool
	scriptPath     string
	pid            int
	stream         *cmd.ProcessStream
	buffer         string
	closed         bool
}

type contextExecutionPayload struct {
	Results        []CodeExecutionResult `json:"results"`
	Logs           CodeExecutionLogs     `json:"logs"`
	Error          *CodeExecutionError   `json:"error,omitempty"`
	ExecutionCount int                   `json:"executionCount"`
}

func (s *Sandbox) RunCode(ctx context.Context, code string, opts *RunCodeOptions) (*CodeExecution, error) {
	if opts != nil && opts.Context != nil {
		if !isPythonLanguage(opts.Context.Language) {
			runtime, err := s.Runtime()
			if err != nil {
				return nil, err
			}
			return runCodeWithRuntime(ctx, runtime, code, mergeContextRunCodeOptions(opts.Context, opts))
		}
		session, err := s.resolveCodeContext(opts.Context)
		if err != nil {
			return nil, err
		}
		return session.Execute(ctx, code, opts)
	}
	if isPythonLanguage(runLanguage(opts)) {
		session, err := s.defaultPythonCodeContext(opts)
		if err != nil {
			return nil, err
		}
		return session.Execute(ctx, code, opts)
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runCodeWithRuntime(ctx, runtime, code, opts)
}

func (s *SandboxDetail) RunCode(ctx context.Context, code string, opts *RunCodeOptions) (*CodeExecution, error) {
	if opts != nil && opts.Context != nil {
		if !isPythonLanguage(opts.Context.Language) {
			runtime, err := s.Runtime()
			if err != nil {
				return nil, err
			}
			return runCodeWithRuntime(ctx, runtime, code, mergeContextRunCodeOptions(opts.Context, opts))
		}
		session, err := s.resolveCodeContext(opts.Context)
		if err != nil {
			return nil, err
		}
		return session.Execute(ctx, code, opts)
	}
	if isPythonLanguage(runLanguage(opts)) {
		session, err := s.defaultPythonCodeContext(opts)
		if err != nil {
			return nil, err
		}
		return session.Execute(ctx, code, opts)
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	return runCodeWithRuntime(ctx, runtime, code, opts)
}

func (s *Sandbox) CreateCodeContext(ctx context.Context, opts *CodeContextCreateOptions) (*CodeContext, error) {
	contextDef := newCodeContext(opts)
	if !isPythonLanguage(contextDef.Language) {
		s.codeContextMu.Lock()
		defer s.codeContextMu.Unlock()
		if s.statelessCodeContexts == nil {
			s.statelessCodeContexts = map[string]*CodeContext{}
		}
		s.statelessCodeContexts[contextDef.ContextID] = cloneCodeContext(contextDef)
		return cloneCodeContext(contextDef), nil
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	session := &PythonCodeContextSession{runtime: runtime, context: contextDef}
	if err := session.ensureStarted(ctx); err != nil {
		return nil, err
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.codeContexts == nil {
		s.codeContexts = map[string]*PythonCodeContextSession{}
	}
	s.codeContexts[contextDef.ContextID] = session
	return cloneCodeContext(contextDef), nil
}

func (s *SandboxDetail) CreateCodeContext(ctx context.Context, opts *CodeContextCreateOptions) (*CodeContext, error) {
	contextDef := newCodeContext(opts)
	if !isPythonLanguage(contextDef.Language) {
		s.codeContextMu.Lock()
		defer s.codeContextMu.Unlock()
		if s.statelessCodeContexts == nil {
			s.statelessCodeContexts = map[string]*CodeContext{}
		}
		s.statelessCodeContexts[contextDef.ContextID] = cloneCodeContext(contextDef)
		return cloneCodeContext(contextDef), nil
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	session := &PythonCodeContextSession{runtime: runtime, context: contextDef}
	if err := session.ensureStarted(ctx); err != nil {
		return nil, err
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.codeContexts == nil {
		s.codeContexts = map[string]*PythonCodeContextSession{}
	}
	s.codeContexts[contextDef.ContextID] = session
	return cloneCodeContext(contextDef), nil
}

func (s *Sandbox) ListCodeContexts() []*CodeContext {
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	out := make([]*CodeContext, 0, len(s.codeContexts)+len(s.statelessCodeContexts))
	for _, context := range s.statelessCodeContexts {
		out = append(out, cloneCodeContext(context))
	}
	for _, session := range s.codeContexts {
		out = append(out, cloneCodeContext(session.context))
	}
	return out
}

func (s *SandboxDetail) ListCodeContexts() []*CodeContext {
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	out := make([]*CodeContext, 0, len(s.codeContexts)+len(s.statelessCodeContexts))
	for _, context := range s.statelessCodeContexts {
		out = append(out, cloneCodeContext(context))
	}
	for _, session := range s.codeContexts {
		out = append(out, cloneCodeContext(session.context))
	}
	return out
}

func (s *Sandbox) RestartCodeContext(ctx context.Context, contextRef any) (*CodeContext, error) {
	if stateless := s.resolveStatelessCodeContext(contextRef); stateless != nil {
		return cloneCodeContext(stateless), nil
	}
	session, err := s.resolveCodeContext(contextRef)
	if err != nil {
		return nil, err
	}
	if err := session.Restart(ctx); err != nil {
		return nil, err
	}
	return cloneCodeContext(session.context), nil
}

func (s *SandboxDetail) RestartCodeContext(ctx context.Context, contextRef any) (*CodeContext, error) {
	if stateless := s.resolveStatelessCodeContext(contextRef); stateless != nil {
		return cloneCodeContext(stateless), nil
	}
	session, err := s.resolveCodeContext(contextRef)
	if err != nil {
		return nil, err
	}
	if err := session.Restart(ctx); err != nil {
		return nil, err
	}
	return cloneCodeContext(session.context), nil
}

func (s *Sandbox) RemoveCodeContext(ctx context.Context, contextRef any) error {
	if s.removeStatelessCodeContext(contextRef) {
		return nil
	}
	session, err := s.resolveCodeContext(contextRef)
	if err != nil {
		return err
	}
	s.codeContextMu.Lock()
	delete(s.codeContexts, session.context.ContextID)
	s.codeContextMu.Unlock()
	return session.Close(ctx)
}

func (s *SandboxDetail) RemoveCodeContext(ctx context.Context, contextRef any) error {
	if s.removeStatelessCodeContext(contextRef) {
		return nil
	}
	session, err := s.resolveCodeContext(contextRef)
	if err != nil {
		return err
	}
	s.codeContextMu.Lock()
	delete(s.codeContexts, session.context.ContextID)
	s.codeContextMu.Unlock()
	return session.Close(ctx)
}

func runCodeWithRuntime(ctx context.Context, runtime *Runtime, code string, opts *RunCodeOptions) (*CodeExecution, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("sandbox: code is required")
	}
	if opts == nil {
		opts = &RunCodeOptions{}
	}

	spec, err := codeLanguageSpecFor(opts.Language)
	if err != nil {
		return nil, err
	}

	requestID := fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
	scriptPath := codeFileBase + "-" + requestID + spec.extension
	resultPath := codeFileBase + "-" + requestID + ".result.json"

	if err := runtime.WriteFile(ctx, &cmd.UploadBytesRequest{
		Path: scriptPath,
		Data: []byte(spec.buildSource(code, resultPath)),
	}, nil); err != nil {
		return nil, err
	}

	stream, err := runtime.Start(ctx, &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{
			Cmd:  spec.command,
			Args: spec.args(scriptPath),
			Envs: opts.Envs,
			CWD:  stringPtr(opts.CWD),
		},
		Timeout: opts.Timeout,
	}, nil)
	if err != nil {
		return nil, err
	}

	cmdID := ""
	streamedStdout := make([]string, 0, 8)
	streamedStderr := make([]string, 0, 8)
	var endEvent *cmd.ProcessEndEvent

	defer func() {
		_ = runtime.Remove(ctx, &cmd.RemoveRequest{Path: scriptPath}, nil)
		_ = runtime.Remove(ctx, &cmd.RemoveRequest{Path: resultPath}, nil)
		_ = stream.Close()
	}()

	for {
		frame, err := stream.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if frame.Event.Start != nil {
			cmdID = frame.Event.Start.CmdID
			continue
		}
		if frame.Event.Data != nil {
			now := time.Now().UnixMicro()
			stdoutChunk := decodeStreamData(frame.Event.Data.Stdout)
			stderrChunk := decodeStreamData(frame.Event.Data.Stderr)
			if stdoutChunk != "" {
				streamedStdout = append(streamedStdout, stdoutChunk)
				if opts.OnStdout != nil {
					opts.OnStdout(CodeOutputChunk{Error: false, Line: stdoutChunk, Timestamp: now})
				}
			}
			if stderrChunk != "" {
				streamedStderr = append(streamedStderr, stderrChunk)
				if opts.OnStderr != nil {
					opts.OnStderr(CodeOutputChunk{Error: true, Line: stderrChunk, Timestamp: now})
				}
			}
		}
		if frame.Event.End != nil {
			endEvent = frame.Event.End
			break
		}
	}

	var result struct {
		ExitCode int
		Stdout   string
		Stderr   string
		Error    string
	}
	if strings.TrimSpace(cmdID) != "" && spec.resultFile {
		resp, err := getResultWithRetry(ctx, runtime, cmdID)
		if err != nil {
			return nil, err
		}
		result.ExitCode = resp.ExitCode
		result.Stdout = resp.Stdout
		result.Stderr = resp.Stderr
	} else {
		result.Stdout = strings.Join(streamedStdout, "")
		result.Stderr = strings.Join(streamedStderr, "")
		result.ExitCode = exitCodeFromEndEvent(endEvent)
		if endEvent != nil && endEvent.Error != nil {
			result.Error = *endEvent.Error
		}
	}

	payload := struct {
		Results []CodeExecutionResult `json:"results"`
		Error   *CodeExecutionError   `json:"error,omitempty"`
	}{
		Results: []CodeExecutionResult{},
	}
	if spec.resultFile {
		payload = readResultPayload(ctx, runtime, resultPath)
	}

	executionError := payload.Error
	if executionError == nil {
		executionError = buildExecutionError(result.ExitCode, result.Stderr, result.Error)
	}
	if executionError != nil && opts.OnError != nil {
		opts.OnError(*executionError)
	}
	resultCallback := opts.OnResult
	if resultCallback == nil {
		resultCallback = opts.OnResults
	}
	for _, item := range payload.Results {
		if resultCallback != nil {
			resultCallback(item)
		}
	}

	return &CodeExecution{
		Results: payload.Results,
		Logs: CodeExecutionLogs{
			Stdout: splitLogLines(firstNonEmptyString(result.Stdout, strings.Join(streamedStdout, ""))),
			Stderr: splitLogLines(firstNonEmptyString(result.Stderr, strings.Join(streamedStderr, ""))),
		},
		Error:          executionError,
		ExecutionCount: 1,
	}, nil
}

func getResultWithRetry(ctx context.Context, runtime *Runtime, cmdID string) (*cmd.GetResultResponse, error) {
	var lastErr error
	for attempt := 0; attempt < 40; attempt++ {
		resp, err := runtime.GetResult(ctx, &cmd.GetResultRequest{CmdID: cmdID}, nil)
		if err == nil {
			return resp, nil
		}
		var apiErr *core.APIError
		if !errors.As(err, &apiErr) || apiErr.Kind != core.APIErrorKindNotFound {
			return nil, err
		}
		message := strings.ToLower(apiErr.Error())
		if !strings.Contains(message, "process not found") && !strings.Contains(message, "not finished") {
			return nil, err
		}
		lastErr = err
		if attempt == 39 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil, lastErr
}

func exitCodeFromEndEvent(endEvent *cmd.ProcessEndEvent) int {
	if endEvent == nil {
		return 0
	}
	re := regexp.MustCompile(`(?i)exit status (\d+)`)
	if matches := re.FindStringSubmatch(endEvent.Status); len(matches) == 2 {
		if value, err := strconv.Atoi(matches[1]); err == nil {
			return value
		}
	}
	if endEvent.Error != nil && strings.TrimSpace(*endEvent.Error) != "" {
		return 1
	}
	if !endEvent.Exited {
		return 1
	}
	return 0
}

func (s *Sandbox) defaultPythonCodeContext(opts *RunCodeOptions) (*PythonCodeContextSession, error) {
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.defaultCodeContext != nil {
		return s.defaultCodeContext, nil
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	s.defaultCodeContext = &PythonCodeContextSession{
		runtime:        runtime,
		context:        newDefaultPythonCodeContext(opts),
		defaultContext: true,
	}
	return s.defaultCodeContext, nil
}

func (s *SandboxDetail) defaultPythonCodeContext(opts *RunCodeOptions) (*PythonCodeContextSession, error) {
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.defaultCodeContext != nil {
		return s.defaultCodeContext, nil
	}
	runtime, err := s.Runtime()
	if err != nil {
		return nil, err
	}
	s.defaultCodeContext = &PythonCodeContextSession{
		runtime:        runtime,
		context:        newDefaultPythonCodeContext(opts),
		defaultContext: true,
	}
	return s.defaultCodeContext, nil
}

func (s *Sandbox) resolveStatelessCodeContext(contextRef any) *CodeContext {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return nil
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	return cloneCodeContext(s.statelessCodeContexts[contextID])
}

func (s *SandboxDetail) resolveStatelessCodeContext(contextRef any) *CodeContext {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return nil
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	return cloneCodeContext(s.statelessCodeContexts[contextID])
}

func (s *Sandbox) removeStatelessCodeContext(contextRef any) bool {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return false
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.statelessCodeContexts == nil {
		return false
	}
	if _, ok := s.statelessCodeContexts[contextID]; !ok {
		return false
	}
	delete(s.statelessCodeContexts, contextID)
	return true
}

func (s *SandboxDetail) removeStatelessCodeContext(contextRef any) bool {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return false
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	if s.statelessCodeContexts == nil {
		return false
	}
	if _, ok := s.statelessCodeContexts[contextID]; !ok {
		return false
	}
	delete(s.statelessCodeContexts, contextID)
	return true
}

func (s *Sandbox) closeAllCodeContexts(ctx context.Context) {
	s.codeContextMu.Lock()
	sessions := make([]*PythonCodeContextSession, 0, len(s.codeContexts))
	for _, session := range s.codeContexts {
		sessions = append(sessions, session)
	}
	s.codeContexts = nil
	s.statelessCodeContexts = nil
	defaultSession := s.defaultCodeContext
	s.defaultCodeContext = nil
	s.codeContextMu.Unlock()

	for _, session := range sessions {
		_ = session.Close(ctx)
	}
	if defaultSession != nil {
		_ = defaultSession.Close(ctx)
	}
}

func (s *SandboxDetail) closeAllCodeContexts(ctx context.Context) {
	s.codeContextMu.Lock()
	sessions := make([]*PythonCodeContextSession, 0, len(s.codeContexts))
	for _, session := range s.codeContexts {
		sessions = append(sessions, session)
	}
	s.codeContexts = nil
	s.statelessCodeContexts = nil
	defaultSession := s.defaultCodeContext
	s.defaultCodeContext = nil
	s.codeContextMu.Unlock()

	for _, session := range sessions {
		_ = session.Close(ctx)
	}
	if defaultSession != nil {
		_ = defaultSession.Close(ctx)
	}
}

func (s *Sandbox) resolveCodeContext(contextRef any) (*PythonCodeContextSession, error) {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return nil, err
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	session := s.codeContexts[contextID]
	if session == nil {
		return nil, fmt.Errorf("sandbox: code context not found: %s", contextID)
	}
	return session, nil
}

func (s *SandboxDetail) resolveCodeContext(contextRef any) (*PythonCodeContextSession, error) {
	contextID, err := contextIDFromRef(contextRef)
	if err != nil {
		return nil, err
	}
	s.codeContextMu.Lock()
	defer s.codeContextMu.Unlock()
	session := s.codeContexts[contextID]
	if session == nil {
		return nil, fmt.Errorf("sandbox: code context not found: %s", contextID)
	}
	return session, nil
}

func contextIDFromRef(ref any) (string, error) {
	switch value := ref.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("sandbox: code context id is required")
		}
		return strings.TrimSpace(value), nil
	case *CodeContext:
		if value == nil || strings.TrimSpace(value.ContextID) == "" {
			return "", fmt.Errorf("sandbox: code context id is required")
		}
		return strings.TrimSpace(value.ContextID), nil
	default:
		return "", fmt.Errorf("sandbox: unsupported code context reference %T", ref)
	}
}

func newCodeContext(opts *CodeContextCreateOptions) *CodeContext {
	context := &CodeContext{
		ContextID: fmt.Sprintf("ctx-%d-%d", time.Now().UnixNano(), rand.Int63()),
		Language:  "python",
	}
	if opts != nil {
		if strings.TrimSpace(opts.CWD) != "" {
			context.CWD = strings.TrimSpace(opts.CWD)
		}
		if strings.TrimSpace(opts.Language) != "" {
			context.Language = normalizeLanguage(opts.Language)
		}
		if opts.Timeout != nil {
			value := *opts.Timeout
			context.Timeout = &value
		}
	}
	return context
}

func newDefaultPythonCodeContext(opts *RunCodeOptions) *CodeContext {
	context := &CodeContext{
		ContextID: "default",
		Language:  "python",
	}
	if opts != nil {
		if strings.TrimSpace(opts.CWD) != "" {
			context.CWD = strings.TrimSpace(opts.CWD)
		}
		if opts.Timeout != nil {
			value := *opts.Timeout
			context.Timeout = &value
		}
	}
	return context
}

func cloneCodeContext(context *CodeContext) *CodeContext {
	if context == nil {
		return nil
	}
	cloned := *context
	if context.Timeout != nil {
		value := *context.Timeout
		cloned.Timeout = &value
	}
	return &cloned
}

func (s *PythonCodeContextSession) ensureStarted(ctx context.Context) error {
	if s.stream != nil {
		return nil
	}
	return s.start(ctx)
}

func (s *PythonCodeContextSession) start(ctx context.Context) error {
	contextName := s.context.ContextID
	if s.defaultContext {
		contextName = "default"
	}
	s.scriptPath = codeContextFileBase + "-" + contextName + ".py"
	if err := s.runtime.WriteFile(ctx, &cmd.UploadBytesRequest{
		Path: s.scriptPath,
		Data: []byte(buildPythonContextServerSource()),
	}, nil); err != nil {
		return err
	}
	stream, err := s.runtime.Start(ctx, &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{
			Cmd:  "python3",
			Args: []string{"-u", s.scriptPath},
			CWD:  stringPtr(s.context.CWD),
		},
		Stdin:   boolPtr(true),
		Timeout: s.context.Timeout,
	}, nil)
	if err != nil {
		return err
	}
	s.stream = stream
	for {
		frame, err := stream.Next()
		if err != nil {
			return err
		}
		if frame == nil {
			return fmt.Errorf("sandbox: code context stream ended before start frame")
		}
		if frame.Event.Start != nil {
			s.pid = frame.Event.Start.PID
			return nil
		}
	}
}

func (s *PythonCodeContextSession) Execute(ctx context.Context, code string, opts *RunCodeOptions) (*CodeExecution, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("sandbox: code is required")
	}
	if opts == nil {
		opts = &RunCodeOptions{}
	}
	if err := s.ensureStarted(ctx); err != nil {
		return nil, err
	}
	if !isPythonLanguage(firstNonEmptyCI(opts.Language, s.context.Language)) {
		return nil, fmt.Errorf("sandbox: code contexts currently support python only")
	}
	request := map[string]any{
		"code": base64.StdEncoding.EncodeToString([]byte(code)),
		"cwd":  firstNonEmptyCI(opts.CWD, s.context.CWD),
		"timeout": func() any {
			if opts.Timeout != nil {
				return *opts.Timeout
			}
			if s.context.Timeout != nil {
				return *s.context.Timeout
			}
			return nil
		}(),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if err := s.runtime.SendInput(ctx, &cmd.SendInputRequest{
		Process: cmd.ProcessSelector{PID: s.pid},
		Input:   cmd.ProcessInput{Stdin: encodeStreamData(string(payload) + "\n")},
	}, nil); err != nil {
		return nil, err
	}
	result, err := s.readExecutionPayload()
	if err != nil {
		return nil, err
	}
	execution := &CodeExecution{
		Results:        result.Results,
		Logs:           result.Logs,
		Error:          result.Error,
		ExecutionCount: result.ExecutionCount,
	}
	emitContextCallbacks(execution, opts)
	return execution, nil
}

func (s *PythonCodeContextSession) readExecutionPayload() (*contextExecutionPayload, error) {
	for {
		if newline := strings.IndexByte(s.buffer, '\n'); newline >= 0 {
			line := s.buffer[:newline]
			s.buffer = s.buffer[newline+1:]
			if !strings.HasPrefix(line, codeContextPayloadPrefix) {
				continue
			}
			var payload contextExecutionPayload
			if err := json.Unmarshal([]byte(line[len(codeContextPayloadPrefix):]), &payload); err != nil {
				return nil, err
			}
			if payload.Results == nil {
				payload.Results = []CodeExecutionResult{}
			}
			if payload.Logs.Stdout == nil {
				payload.Logs.Stdout = []string{}
			}
			if payload.Logs.Stderr == nil {
				payload.Logs.Stderr = []string{}
			}
			return &payload, nil
		}
		frame, err := s.stream.Next()
		if err != nil {
			return nil, err
		}
		if frame == nil {
			return nil, fmt.Errorf("sandbox: code context stream closed")
		}
		if frame.Event.Data != nil {
			s.buffer += decodeStreamData(frame.Event.Data.Stdout)
			stderr := decodeStreamData(frame.Event.Data.Stderr)
			if strings.TrimSpace(stderr) != "" {
				return nil, fmt.Errorf("sandbox: %s", firstNonEmptyLine(stderr))
			}
		}
		if frame.Event.End != nil {
			return nil, fmt.Errorf("sandbox: code context stream closed")
		}
	}
}

func (s *PythonCodeContextSession) Restart(ctx context.Context) error {
	if err := s.Close(ctx); err != nil {
		return err
	}
	s.closed = false
	return s.start(ctx)
}

func (s *PythonCodeContextSession) Close(ctx context.Context) error {
	if s.closed {
		return nil
	}
	s.closed = true
	if s.pid != 0 {
		err := s.runtime.SendSignal(ctx, &cmd.SendSignalRequest{
			Process: cmd.ProcessSelector{PID: s.pid},
			Signal:  string(cmd.SignalSIGKILL),
		}, nil)
		if err != nil && !isMissingProcessError(err) {
			return err
		}
	}
	if s.scriptPath != "" {
		_ = s.runtime.Remove(ctx, &cmd.RemoveRequest{Path: s.scriptPath}, nil)
	}
	if s.stream != nil {
		_ = s.stream.Close()
	}
	s.pid = 0
	s.scriptPath = ""
	s.stream = nil
	s.buffer = ""
	return nil
}

func emitContextCallbacks(execution *CodeExecution, opts *RunCodeOptions) {
	if execution == nil || opts == nil {
		return
	}
	timestamp := time.Now().UnixMicro()
	for _, line := range execution.Logs.Stdout {
		if opts.OnStdout != nil {
			opts.OnStdout(CodeOutputChunk{Error: false, Line: line, Timestamp: timestamp})
		}
		timestamp++
	}
	for _, line := range execution.Logs.Stderr {
		if opts.OnStderr != nil {
			opts.OnStderr(CodeOutputChunk{Error: true, Line: line, Timestamp: timestamp})
		}
		timestamp++
	}
	resultCallback := opts.OnResult
	if resultCallback == nil {
		resultCallback = opts.OnResults
	}
	for _, result := range execution.Results {
		if resultCallback != nil {
			resultCallback(result)
		}
	}
	if execution.Error != nil && opts.OnError != nil {
		opts.OnError(*execution.Error)
	}
}

func runLanguage(opts *RunCodeOptions) string {
	if opts == nil {
		return "python"
	}
	return firstNonEmptyCI(opts.Language, "python")
}

func mergeContextRunCodeOptions(contextDef *CodeContext, opts *RunCodeOptions) *RunCodeOptions {
	if contextDef == nil {
		return opts
	}
	merged := &RunCodeOptions{Context: contextDef}
	if opts != nil {
		*merged = *opts
	}
	merged.Context = contextDef
	if strings.TrimSpace(merged.Language) == "" {
		merged.Language = contextDef.Language
	}
	if strings.TrimSpace(merged.CWD) == "" {
		merged.CWD = contextDef.CWD
	}
	if merged.Timeout == nil && contextDef.Timeout != nil {
		value := *contextDef.Timeout
		merged.Timeout = &value
	}
	return merged
}

func codeLanguageSpecFor(language string) (*codeLanguageSpec, error) {
	normalized := normalizeLanguage(language)
	switch normalized {
	case "python", "py":
		return &codeLanguageSpec{
			extension: ".py",
			command:   "python3",
			args: func(scriptPath string) []string {
				return []string{"-u", scriptPath}
			},
			buildSource: buildPythonWrapperSource,
			resultFile:  true,
		}, nil
	case "javascript", "js":
		return &codeLanguageSpec{
			extension: ".mjs",
			command:   "node",
			args:      func(scriptPath string) []string { return []string{scriptPath} },
			buildSource: func(code, _ string) string {
				return code
			},
		}, nil
	case "typescript", "ts":
		return &codeLanguageSpec{
			extension: ".ts",
			command:   "tsx",
			args:      func(scriptPath string) []string { return []string{scriptPath} },
			buildSource: func(code, _ string) string {
				return code
			},
		}, nil
	case "bash", "sh":
		return &codeLanguageSpec{
			extension: ".sh",
			command:   "bash",
			args:      func(scriptPath string) []string { return []string{scriptPath} },
			buildSource: func(code, _ string) string {
				return code
			},
		}, nil
	case "r":
		return &codeLanguageSpec{
			extension: ".R",
			command:   "Rscript",
			args:      func(scriptPath string) []string { return []string{scriptPath} },
			buildSource: func(code, _ string) string {
				return code
			},
		}, nil
	case "java":
		return &codeLanguageSpec{
			extension: ".jsh",
			command:   "jshell",
			args:      func(scriptPath string) []string { return []string{"--execution", "local", scriptPath} },
			buildSource: func(code, _ string) string {
				return code
			},
		}, nil
	default:
		return nil, fmt.Errorf("sandbox: unsupported code language: %s", language)
	}
}

func normalizeLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if normalized == "" {
		return "python"
	}
	return normalized
}

func isPythonLanguage(language string) bool {
	normalized := normalizeLanguage(language)
	return normalized == "python" || normalized == "py"
}

func buildExecutionError(exitCode int, stderr, runtimeError string) *CodeExecutionError {
	if exitCode == 0 && strings.TrimSpace(runtimeError) == "" {
		return nil
	}
	message := firstNonEmptyLine(firstNonEmptyString(runtimeError, stderr))
	if message == "" {
		message = fmt.Sprintf("code execution failed with exit code %d", maxInt(exitCode, 1))
	}
	return &CodeExecutionError{Message: message}
}

func readResultPayload(ctx context.Context, runtime *Runtime, path string) struct {
	Results []CodeExecutionResult `json:"results"`
	Error   *CodeExecutionError   `json:"error,omitempty"`
} {
	payload := struct {
		Results []CodeExecutionResult `json:"results"`
		Error   *CodeExecutionError   `json:"error,omitempty"`
	}{
		Results: []CodeExecutionResult{},
	}
	resp, err := runtime.ReadFile(ctx, &cmd.FileRequest{Path: path}, nil)
	if err != nil {
		return payload
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil || strings.TrimSpace(string(body)) == "" {
		return payload
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return struct {
			Results []CodeExecutionResult `json:"results"`
			Error   *CodeExecutionError   `json:"error,omitempty"`
		}{Results: []CodeExecutionResult{}}
	}
	if payload.Results == nil {
		payload.Results = []CodeExecutionResult{}
	}
	return payload
}

func splitLogLines(value string) []string {
	if value == "" {
		return []string{}
	}
	lines := strings.SplitAfter(value, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []string{value}
	}
	return lines
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyCI(values ...string) string {
	return firstNonEmptyString(values...)
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func boolPtr(value bool) *bool {
	return &value
}

func buildPythonWrapperSource(code, resultPath string) string {
	encodedCode := base64.StdEncoding.EncodeToString([]byte(code))
	return fmt.Sprintf(`import ast
import base64
import io
import json
import os
import traceback

os.environ.setdefault("MPLBACKEND", "Agg")

RESULT_PATH = %q
USER_CODE = base64.b64decode(%q).decode("utf-8")
payload = {"results": [], "error": None}


def _write_payload():
    with open(RESULT_PATH, "w", encoding="utf-8") as handle:
        json.dump(payload, handle)


%s

namespace = {"__name__": "__main__", "display": display, "_emit_result": _emit_result}
_install_matplotlib_hook()

try:
    tree = ast.parse(USER_CODE, filename="<seacloud-code>", mode="exec")
    if tree.body and isinstance(tree.body[len(tree.body) - 1], ast.Expr):
        last_value = tree.body[len(tree.body) - 1].value
        tree.body[len(tree.body) - 1] = ast.Assign(
            targets=[ast.Name(id="__seacloud_last_result", ctx=ast.Store())],
            value=last_value,
        )
        tree.body.append(
            ast.Expr(
                value=ast.Call(
                    func=ast.Name(id="_emit_result", ctx=ast.Load()),
                    args=[ast.Name(id="__seacloud_last_result", ctx=ast.Load())],
                    keywords=[],
                )
            )
        )
    ast.fix_missing_locations(tree)
    exec(compile(tree, "<seacloud-code>", "exec"), namespace, namespace)
except Exception as exc:
    payload["error"] = {
        "name": exc.__class__.__name__,
        "message": str(exc),
        "traceback": traceback.format_exc(),
    }
    _write_payload()
    raise
else:
    _write_payload()
`, resultPath, encodedCode, pythonResultHelpers())
}

func buildPythonContextServerSource() string {
	return fmt.Sprintf(`import ast
import base64
import contextlib
import io
import json
import os
import sys
import traceback

os.environ.setdefault("MPLBACKEND", "Agg")

SENTINEL = %q
namespace = {"__name__": "__main__"}
execution_count = 0


def _split_log_lines(value):
    if not value:
        return []
    return value.splitlines(True) or [value]


%s

while True:
    line = sys.stdin.readline()
    if not line:
        break
    request = json.loads(line)
    user_code = base64.b64decode(request["code"]).decode("utf-8")
    cwd = request.get("cwd")
    payload = {
        "results": [],
        "logs": {"stdout": [], "stderr": []},
        "error": None,
        "executionCount": execution_count + 1,
    }
    namespace["display"] = display
    namespace["_emit_result"] = _emit_result
    stdout_buffer = io.StringIO()
    stderr_buffer = io.StringIO()
    previous_cwd = os.getcwd()
    try:
        if cwd:
            os.chdir(cwd)
        globals()["payload"] = payload
        globals()["_install_matplotlib_hook"]()
        with contextlib.redirect_stdout(stdout_buffer), contextlib.redirect_stderr(stderr_buffer):
            tree = ast.parse(user_code, filename="<seacloud-context>", mode="exec")
            if tree.body and isinstance(tree.body[len(tree.body) - 1], ast.Expr):
                last_value = tree.body[len(tree.body) - 1].value
                tree.body[len(tree.body) - 1] = ast.Assign(
                    targets=[ast.Name(id="__seacloud_last_result", ctx=ast.Store())],
                    value=last_value,
                )
                tree.body.append(
                    ast.Expr(
                        value=ast.Call(
                            func=ast.Name(id="_emit_result", ctx=ast.Load()),
                            args=[ast.Name(id="__seacloud_last_result", ctx=ast.Load())],
                            keywords=[],
                        )
                    )
                )
            ast.fix_missing_locations(tree)
            execution_count += 1
            payload["executionCount"] = execution_count
            exec(compile(tree, "<seacloud-context>", "exec"), namespace, namespace)
    except Exception as exc:
        payload["error"] = {
            "name": exc.__class__.__name__,
            "message": str(exc),
            "traceback": traceback.format_exc(),
        }
    finally:
        if cwd:
            os.chdir(previous_cwd)
    payload["logs"]["stdout"] = _split_log_lines(stdout_buffer.getvalue())
    payload["logs"]["stderr"] = _split_log_lines(stderr_buffer.getvalue())
    sys.stdout.write(SENTINEL + json.dumps(payload) + "\n")
    sys.stdout.flush()
`, codeContextPayloadPrefix, pythonResultHelpers())
}

func pythonResultHelpers() string {
	return `def _chart_payload(figure):
    chart = {
        "type": "unknown",
        "title": None,
        "x_label": None,
        "y_label": None,
        "x_unit": None,
        "y_unit": None,
        "elements": [],
    }
    axes = figure.axes[0] if getattr(figure, "axes", None) else None
    if axes is None:
        return chart
    chart["title"] = axes.get_title() or None
    chart["x_label"] = axes.get_xlabel() or None
    chart["y_label"] = axes.get_ylabel() or None

    if getattr(axes, "containers", None):
        chart["type"] = "bar"
        for container in axes.containers:
            label = container.get_label()
            for patch in getattr(container, "patches", []):
                chart["elements"].append({
                    "label": str(getattr(patch, "get_x", lambda: 0)() + getattr(patch, "get_width", lambda: 0)() / 2),
                    "group": None if label == "_nolegend_" else label,
                    "value": float(getattr(patch, "get_height", lambda: 0)()),
                })
        tick_labels = [tick.get_text() for tick in axes.get_xticklabels()]
        for index, tick in enumerate(tick_labels):
            if index < len(chart["elements"]) and tick:
                chart["elements"][index]["label"] = tick
        return chart

    if getattr(axes, "lines", None):
        chart["type"] = "line"
        for line in axes.lines:
            group = line.get_label()
            x_values = list(line.get_xdata())
            y_values = list(line.get_ydata())
            for x_value, y_value in zip(x_values, y_values):
                chart["elements"].append({
                    "label": str(x_value),
                    "group": None if group == "_nolegend_" else group,
                    "value": float(y_value),
                })
        return chart

    if getattr(axes, "collections", None):
        chart["type"] = "scatter"
        for collection in axes.collections:
            offsets = getattr(collection, "get_offsets", lambda: [])()
            for point in offsets:
                try:
                    x_value = float(point[0])
                    y_value = float(point[1])
                except Exception:
                    continue
                chart["elements"].append({
                    "label": str(x_value),
                    "group": None,
                    "value": y_value,
                })
        return chart

    return chart


def _emit_result(value):
    if value is None:
        return

    try:
        import matplotlib.figure

        if isinstance(value, matplotlib.figure.Figure):
            buffer = io.BytesIO()
            value.savefig(buffer, format="png", bbox_inches="tight")
            payload["results"].append({
                "png": base64.b64encode(buffer.getvalue()).decode("ascii"),
                "chart": _chart_payload(value),
            })
            return
    except Exception:
        pass

    try:
        from PIL import Image

        if isinstance(value, Image.Image):
            buffer = io.BytesIO()
            value.save(buffer, format="PNG")
            payload["results"].append({
                "png": base64.b64encode(buffer.getvalue()).decode("ascii"),
            })
            return
    except Exception:
        pass

    try:
        import pandas as pd

        if isinstance(value, pd.DataFrame):
            payload["results"].append({
                "text": value.to_string(),
                "json": value.to_dict(orient="records"),
            })
            return
    except Exception:
        pass

    if isinstance(value, (str, int, float, bool, list, dict)):
        payload["results"].append({
            "text": value if isinstance(value, str) else repr(value),
            "json": value,
        })
        return

    payload["results"].append({"text": repr(value)})


def display(*values):
    for value in values:
        _emit_result(value)


def _install_matplotlib_hook():
    try:
        import matplotlib._pylab_helpers
        import matplotlib.pyplot as plt

        def _patched_show(*args, **kwargs):
            managers = matplotlib._pylab_helpers.Gcf.get_all_fig_managers()
            for manager in managers:
                figure = getattr(getattr(manager, "canvas", None), "figure", None)
                if figure is not None:
                    _emit_result(figure)
            plt.close("all")
            return None

        plt.show = _patched_show
    except Exception:
        pass`
}
