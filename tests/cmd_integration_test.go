package tests

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
)

func TestIntegrationCMD(t *testing.T) {
	baseURL, apiKey, templateID := integrationConfig(t)
	workspaceRoot := os.Getenv("SANDBOX_TEST_SANDBOX_ROOT")
	if workspaceRoot == "" {
		workspaceRoot = "/root/workspace"
	}

	if templateID == "" {
		t.Skip("SANDBOX_TEST_TEMPLATE_ID is not set")
	}

	client, err := sandbox.NewClient(baseURL, apiKey)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx := context.Background()
	workspaceID := "go-cmd-sdk-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	timeout := int32(1800)
	waitReady := true

	created, err := client.CreateSandbox(ctx, &control.NewSandboxRequest{
		TemplateID:  templateID,
		WorkspaceID: workspaceID,
		Timeout:     &timeout,
		WaitReady:   &waitReady,
	})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}

	sandboxID := created.SandboxID
	if sandboxID == "" {
		t.Fatal("created sandbox id is empty")
	}
	defer func() {
		if err := client.DeleteSandbox(ctx, sandboxID); err != nil && !isNotFound(err) {
			t.Fatalf("DeleteSandbox: %v", err)
		}
	}()

	if created.EnvdURL == nil || strings.TrimSpace(*created.EnvdURL) == "" {
		t.Skip("sandbox did not return envdUrl")
	}

	accessToken := ""
	if created.EnvdAccessToken != nil {
		accessToken = *created.EnvdAccessToken
	}

	command, err := client.Runtime(*created.EnvdURL, accessToken)
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}

	t.Run("rest files", func(t *testing.T) {
		path := strings.TrimRight(workspaceRoot, "/") + "/go-cmd-sdk.txt"
		resp, err := command.UploadBytes(ctx, &cmd.UploadBytesRequest{
			Path: path,
			Data: []byte("go-cmd"),
		}, nil)
		if err != nil {
			t.Fatalf("UploadBytes: %v", err)
		}
		if len(resp) == 0 {
			t.Fatal("upload response is empty")
		}

		download, err := command.Download(ctx, &cmd.DownloadRequest{Path: path}, nil)
		if err != nil {
			t.Fatalf("Download: %v", err)
		}
		defer download.Body.Close()

		body, err := io.ReadAll(download.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(body) != "go-cmd" {
			t.Fatalf("body = %q", string(body))
		}

		content, err := command.FilesContent(ctx, &cmd.FilesContentRequest{Path: path}, nil)
		if err != nil {
			t.Fatalf("FilesContent: %v", err)
		}
		if content.Type != "text" {
			t.Fatalf("content type = %q", content.Type)
		}
		if content.Content != "go-cmd" {
			t.Fatalf("content = %q", content.Content)
		}
	})

	t.Run("list dir", func(t *testing.T) {
		depth := 1
		resp, err := command.ListDir(ctx, &cmd.ListDirRequest{
			Path:  workspaceRoot,
			Depth: &depth,
		}, nil)
		if err != nil {
			t.Fatalf("ListDir: %v", err)
		}
		if resp.Entries == nil {
			t.Fatal("entries is nil")
		}
	})

	t.Run("process stream", func(t *testing.T) {
		stream, err := command.Start(ctx, &cmd.ProcessStartRequest{
			Process: &cmd.ProcessConfig{Cmd: "cat"},
			Tag:     "go-cmd-test",
		}, nil)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		defer func() {
			_ = stream.Close()
		}()

		startFrame, err := stream.Next()
		if err != nil {
			t.Fatalf("stream.Next start: %v", err)
		}
		if startFrame.Event.Start == nil {
			t.Fatalf("expected start event, got %#v", startFrame.Event)
		}

		pid := startFrame.Event.Start.PID
		if pid == 0 {
			t.Fatal("start event pid is empty")
		}

		if err := command.SendInput(ctx, &cmd.SendInputRequest{
			Process: cmd.ProcessSelector{PID: pid},
			Input: cmd.ProcessInput{
				Stdin: base64.StdEncoding.EncodeToString([]byte("ping\n")),
			},
		}, nil); err != nil {
			t.Fatalf("SendInput: %v", err)
		}

		if err := command.CloseStdin(ctx, &cmd.CloseStdinRequest{
			Process: cmd.ProcessSelector{PID: pid},
		}, nil); err != nil {
			t.Fatalf("CloseStdin: %v", err)
		}

		sawOutput := false
		sawEnd := false
		for i := 0; i < 10; i++ {
			frame, err := stream.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("stream.Next: %v", err)
			}
			if frame.Event.Data != nil && frame.Event.Data.Stdout != "" {
				data, err := base64.StdEncoding.DecodeString(frame.Event.Data.Stdout)
				if err != nil {
					t.Fatalf("DecodeString: %v", err)
				}
				if strings.Contains(string(data), "ping") {
					sawOutput = true
				}
			}
			if frame.Event.End != nil {
				sawEnd = true
				break
			}
		}

		if !sawOutput {
			t.Fatal("did not observe expected stdout")
		}
		if !sawEnd {
			t.Fatal("did not observe process end event")
		}
	})
}
