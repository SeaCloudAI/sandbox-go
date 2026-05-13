package tests

import (
	"context"
	"testing"

	sandbox "github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

type sdkClient struct {
	t            *testing.T
	transportOps []core.TransportOption
}

func newSDKClient(t *testing.T, baseURL string, transportOpts ...core.TransportOption) *sdkClient {
	t.Helper()
	t.Setenv("SEACLOUD_BASE_URL", baseURL)
	t.Setenv("SEACLOUD_API_KEY", "unit-auth-value")
	return &sdkClient{t: t, transportOps: transportOpts}
}

func (c *sdkClient) Create(ctx context.Context, templateID string, opts *sandbox.CreateOptions) (*sandbox.Sandbox, error) {
	return sandbox.Create(ctx, templateID, opts, c.transportOps...)
}

func (c *sdkClient) Connect(ctx context.Context, sandboxID string, opts *sandbox.ConnectOptions) (*sandbox.Sandbox, error) {
	return sandbox.Connect(ctx, sandboxID, opts, c.transportOps...)
}

func (c *sdkClient) List(ctx context.Context, opts *sandbox.ListOptions) (*sandbox.SandboxPaginator, error) {
	return sandbox.List(ctx, opts, c.transportOps...)
}

func (c *sdkClient) Get(ctx context.Context, sandboxID string) (*sandbox.SandboxDetail, error) {
	return sandbox.Get(ctx, sandboxID, c.transportOps...)
}

func (c *sdkClient) BuildTemplate(ctx context.Context, template *sandbox.Template, name string, opts *sandbox.TemplateBuildOptions) (*sandbox.TemplateBuildInfo, error) {
	return sandbox.BuildTemplate(ctx, template, name, opts, c.transportOps...)
}

func (c *sdkClient) BuildTemplateInBackground(ctx context.Context, template *sandbox.Template, name string, opts *sandbox.TemplateBuildOptions) (*sandbox.TemplateBuildInfo, error) {
	return sandbox.BuildTemplateInBackground(ctx, template, name, opts, c.transportOps...)
}

func (c *sdkClient) TemplateExists(ctx context.Context, ref string) (bool, error) {
	return sandbox.TemplateExists(ctx, ref, c.transportOps...)
}

func (c *sdkClient) GetTemplateBuildStatus(ctx context.Context, templateID, buildID string, opts *sandbox.TemplateBuildStatusOptions) (*build.BuildStatusResponse, error) {
	return sandbox.GetTemplateBuildStatus(ctx, templateID, buildID, opts, c.transportOps...)
}

func (c *sdkClient) GetTemplate(ctx context.Context, ref string, opts *sandbox.TemplateGetOptions) (*build.TemplateResponse, error) {
	return sandbox.GetTemplate(ctx, ref, opts, c.transportOps...)
}

func (c *sdkClient) ListTemplates(ctx context.Context, opts *sandbox.TemplateListOptions) ([]build.ListedTemplate, error) {
	return sandbox.ListTemplates(ctx, opts, c.transportOps...)
}

func (c *sdkClient) DeleteTemplate(ctx context.Context, ref string) error {
	return sandbox.DeleteTemplate(ctx, ref, c.transportOps...)
}

func (c *sdkClient) AssignTemplateTags(ctx context.Context, targetName string, tags []string) (*sandbox.TemplateTagInfo, error) {
	return sandbox.AssignTemplateTags(ctx, targetName, tags, c.transportOps...)
}

func (c *sdkClient) GetTemplateTags(ctx context.Context, ref string) ([]sandbox.TemplateTag, error) {
	return sandbox.GetTemplateTags(ctx, ref, c.transportOps...)
}

func (c *sdkClient) RemoveTemplateTags(ctx context.Context, ref string, tags []string) error {
	return sandbox.RemoveTemplateTags(ctx, ref, tags, c.transportOps...)
}

func (c *sdkClient) CreateSandbox(ctx context.Context, req *control.NewSandboxRequest) (*sandbox.Sandbox, error) {
	templateID := ""
	opts := &sandbox.CreateOptions{}
	if req != nil {
		templateID = req.TemplateID
		opts.TemplateID = req.TemplateID
		opts.Timeout = req.Timeout
		opts.Metadata = req.Metadata
		opts.EnvVars = req.EnvVars
		opts.WaitReady = req.WaitReady
	}
	return sandbox.Create(ctx, templateID, opts, c.transportOps...)
}

func (c *sdkClient) ListSandboxes(ctx context.Context, opts *control.ListSandboxesParams) ([]*sandbox.SandboxHandle, error) {
	var paginator *sandbox.SandboxPaginator
	var err error
	if opts == nil {
		paginator, err = sandbox.List(ctx, nil, c.transportOps...)
	} else {
		paginator, err = sandbox.List(ctx, &sandbox.ListOptions{
			Metadata:  opts.Metadata,
			State:     opts.State,
			Limit:     opts.Limit,
			NextToken: opts.NextToken,
		}, c.transportOps...)
	}
	if err != nil {
		return nil, err
	}
	return paginator.NextPage(ctx)
}

func (c *sdkClient) Runtime(baseURL, accessToken string) (*sandbox.Runtime, error) {
	return sandbox.NewRuntime(baseURL, accessToken)
}

func (c *sdkClient) RuntimeFromSandbox(s *control.Sandbox) (*sandbox.Runtime, error) {
	return sandbox.RuntimeFromSandbox(s)
}

func (c *sdkClient) DeleteSandbox(ctx context.Context, sandboxID string) error {
	detail, err := sandbox.Get(ctx, sandboxID, c.transportOps...)
	if err == nil {
		return detail.Delete(ctx)
	}
	return err
}
