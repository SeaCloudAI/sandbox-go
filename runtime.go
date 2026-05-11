package sandbox

import (
	"strings"

	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

type Runtime struct {
	*cmd.Service
}

func RuntimeFromSandbox(sandbox *control.Sandbox) (*Runtime, error) {
	if sandbox == nil || sandbox.EnvdURL == nil || strings.TrimSpace(*sandbox.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}

	accessToken := ""
	if sandbox.EnvdAccessToken != nil {
		accessToken = *sandbox.EnvdAccessToken
	}
	return NewRuntime(*sandbox.EnvdURL, accessToken)
}

func RuntimeFromDetail(sandbox *control.SandboxDetail) (*Runtime, error) {
	if sandbox == nil || sandbox.EnvdURL == nil || strings.TrimSpace(*sandbox.EnvdURL) == "" {
		return nil, core.ErrBaseURLEmpty
	}

	accessToken := ""
	if sandbox.EnvdAccessToken != nil {
		accessToken = *sandbox.EnvdAccessToken
	}
	return NewRuntime(*sandbox.EnvdURL, accessToken)
}
