package tests

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/sandbox-go/cmd"
)

func TestCMDListDirSetsConnectHeadersAndBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/filesystem.Filesystem/ListDir" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Connect-Protocol-Version"); got != "1" {
			t.Fatalf("Connect-Protocol-Version = %q", got)
		}
		if got := r.Header.Get("X-Access-Token"); got != "unit-runtime-auth" {
			t.Fatalf("X-Access-Token = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Basic "+base64.StdEncoding.EncodeToString([]byte("sandbox:")) {
			t.Fatalf("Authorization = %q", got)
		}

		var req cmd.ListDirRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Path != "/tmp" {
			t.Fatalf("path = %q", req.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"entries":[{"name":"a.txt","type":"FILE_TYPE_FILE","path":"/tmp/a.txt","size":1,"mode":33188,"permissions":"-rw-r--r--","owner":"u","group":"g","modifiedTime":"2026-04-22T00:00:00Z"}]}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.ListDir(context.Background(), &cmd.ListDirRequest{Path: "/tmp"}, &cmd.RequestOptions{Username: "sandbox"})
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(resp.Entries) != 1 || resp.Entries[0].Path != "/tmp/a.txt" {
		t.Fatalf("entries = %#v", resp.Entries)
	}
}

func TestCMDDownloadUsesQueryUsernameAndRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("path"); got != "~/hello.txt" {
			t.Fatalf("path query = %q", got)
		}
		if got := r.URL.Query().Get("username"); got != "sandbox" {
			t.Fatalf("username query = %q", got)
		}
		if got := r.Header.Get("Range"); got != "bytes=0-3" {
			t.Fatalf("Range = %q", got)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("hell"))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Download(context.Background(), &cmd.DownloadRequest{Path: "~/hello.txt"}, &cmd.RequestOptions{
		Username: "sandbox",
		Range:    "bytes=0-3",
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "hell" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestCMDPortsAcceptsStatusObject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ports" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Ports(context.Background())
	if err != nil {
		t.Fatalf("Ports: %v", err)
	}
	if len(resp) != 0 {
		t.Fatalf("ports = %#v", resp)
	}
}

func TestCMDPreservesBaseURLPathPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sandbox/sb-1/run" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stdout":"ok","stderr":"","exit_code":0,"duration_ms":1}`))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL+"/sandbox/sb-1", "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Run(context.Background(), &cmd.AgentRunRequest{Cmd: "echo"}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Stdout != "ok" {
		t.Fatalf("stdout = %q", resp.Stdout)
	}
}

func TestCMDProxyPassesThroughNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy/8080/health" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream failed"))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.Proxy(context.Background(), &cmd.ProxyRequest{
		Method: http.MethodGet,
		Port:   8080,
		Path:   "/health",
	})
	if err != nil {
		t.Fatalf("Proxy: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "upstream failed" {
		t.Fatalf("body = %q", string(body))
	}
}

func TestCMDProcessStreamParsesConnectFrames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/Start" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
			t.Fatalf("Content-Type = %q", got)
		}
		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameJSON(t, map[string]any{
			"event": map[string]any{
				"start": map[string]any{"pid": 1234, "cmdId": "cmd-1"},
			},
		}))
		_, _ = w.Write(connectFrameJSON(t, map[string]any{
			"event": map[string]any{
				"data": map[string]any{"stdout": base64.StdEncoding.EncodeToString([]byte("hello\n"))},
			},
		}))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	stream, err := service.Start(context.Background(), &cmd.ProcessStartRequest{
		Process: &cmd.ProcessConfig{Cmd: "echo", Args: []string{"hello"}},
	}, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer stream.Close()

	first, err := stream.Next()
	if err != nil {
		t.Fatalf("first Next: %v", err)
	}
	if first.Event.Start == nil || first.Event.Start.CmdID != "cmd-1" {
		t.Fatalf("start event = %#v", first.Event.Start)
	}

	second, err := stream.Next()
	if err != nil {
		t.Fatalf("second Next: %v", err)
	}
	if second.Event.Data == nil || second.Event.Data.Stdout == "" {
		t.Fatalf("data event = %#v", second.Event.Data)
	}
}

func TestCMDStreamInputEncodesConnectFrames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/process.Process/StreamInput" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Content-Type"); got != "application/connect+json" {
			t.Fatalf("Content-Type = %q", got)
		}

		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		frames := decodeFrames(t, data)
		if len(frames) != 2 {
			t.Fatalf("frames len = %d", len(frames))
		}
		if !strings.Contains(string(frames[0].Payload), `"pid":42`) {
			t.Fatalf("start payload = %s", string(frames[0].Payload))
		}
		if !strings.Contains(string(frames[1].Payload), `"stdin":"aGVsbG8="`) {
			t.Fatalf("data payload = %s", string(frames[1].Payload))
		}

		w.Header().Set("Content-Type", "application/connect+json")
		_, _ = w.Write(connectFrameJSON(t, map[string]any{}))
	}))
	defer server.Close()

	service, err := cmd.NewService(server.URL, "")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	frame, err := service.StreamInput(context.Background(), []cmd.StreamInputFrame{
		{Start: &cmd.StreamInputStart{Process: cmd.ProcessSelector{PID: 42}}},
		{Data: &cmd.StreamInputData{Input: cmd.ProcessInput{Stdin: base64.StdEncoding.EncodeToString([]byte("hello"))}}},
	}, nil)
	if err != nil {
		t.Fatalf("StreamInput: %v", err)
	}
	if frame == nil {
		t.Fatal("frame is nil")
	}
}

func connectFrameJSON(t *testing.T, payload any) []byte {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	out := make([]byte, 5+len(data))
	out[0] = 0
	binary.BigEndian.PutUint32(out[1:5], uint32(len(data)))
	copy(out[5:], data)
	return out
}

func decodeFrames(t *testing.T, data []byte) []cmd.ConnectFrame {
	t.Helper()

	var frames []cmd.ConnectFrame
	for len(data) > 0 {
		if len(data) < 5 {
			t.Fatalf("short frame header: %d", len(data))
		}
		size := binary.BigEndian.Uint32(data[1:5])
		if len(data) < int(5+size) {
			t.Fatalf("short frame payload: %d < %d", len(data), 5+size)
		}
		payload := append([]byte(nil), data[5:5+size]...)
		frames = append(frames, cmd.ConnectFrame{
			Flags:   data[0],
			Payload: payload,
		})
		data = data[5+size:]
	}
	return frames
}
