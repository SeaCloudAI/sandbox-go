package sandbox

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

type gatewayServices struct {
	control *control.Service
	build   *build.Service
}

const defaultBaseURL = "https://sandbox-gateway.cloud.seaart.ai"

func newGatewayServices(baseURL, apiKey string, opts ...core.TransportOption) (*gatewayServices, error) {
	baseURL, apiKey = resolveGatewayConfig(baseURL, apiKey)
	defaultOpts := defaultGatewayTransportOptions("", 0)
	controlService, err := control.NewService(baseURL, apiKey, append(defaultOpts, opts...)...)
	if err != nil {
		return nil, err
	}

	buildOps, err := build.NewService(baseURL, apiKey, append(defaultOpts, opts...)...)
	if err != nil {
		return nil, err
	}

	return &gatewayServices{
		control: controlService,
		build:   buildOps,
	}, nil
}

func newGatewayServicesFromEnv(opts ...core.TransportOption) (*gatewayServices, error) {
	return newGatewayServices("", "", opts...)
}

func NewCMD(baseURL, accessToken string) (*cmd.Service, error) {
	return cmd.NewService(baseURL, accessToken)
}

func NewRuntime(baseURL, accessToken string) (*Runtime, error) {
	service, err := cmd.NewService(baseURL, accessToken)
	if err != nil {
		return nil, err
	}
	return &Runtime{Service: service}, nil
}

func (g *gatewayServices) create(ctx context.Context, templateID string, opts *CreateOptions) (*Sandbox, error) {
	req := &control.NewSandboxRequest{}
	if value := strings.TrimSpace(templateID); value != "" {
		req.TemplateID = value
	}
	if opts != nil {
		if value := strings.TrimSpace(firstNonEmpty(templateID, opts.TemplateID)); value != "" {
			req.TemplateID = value
		}
		req.Timeout = opts.Timeout
		req.Metadata = opts.Metadata
		req.EnvVars = opts.EnvVars
		req.WaitReady = opts.WaitReady
	}
	return g.createSandbox(ctx, req)
}

func (g *gatewayServices) connect(ctx context.Context, sandboxID string, opts *ConnectOptions) (*Sandbox, error) {
	timeout := int32(300)
	if opts != nil {
		timeout = opts.Timeout
	}
	resp, err := g.connectSandbox(ctx, sandboxID, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		return nil, err
	}
	return resp.Sandbox, nil
}

func (g *gatewayServices) list(ctx context.Context, opts *ListOptions) ([]*SandboxHandle, error) {
	if opts == nil {
		return g.listSandboxes(ctx, nil)
	}
	return g.listSandboxes(ctx, &control.ListSandboxesParams{
		Metadata:  opts.Metadata,
		State:     opts.State,
		Limit:     opts.Limit,
		NextToken: opts.NextToken,
	})
}

func (g *gatewayServices) get(ctx context.Context, sandboxID string) (*SandboxDetail, error) {
	return g.getSandbox(ctx, sandboxID)
}

func (g *gatewayServices) buildTemplate(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions) (*TemplateBuildInfo, error) {
	return buildTemplateWithService(ctx, g.build, template, name, opts)
}

func (g *gatewayServices) buildTemplateInBackground(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions) (*TemplateBuildInfo, error) {
	cloned := cloneTemplateBuildOptions(opts)
	wait := false
	cloned.Wait = &wait
	return buildTemplateWithService(ctx, g.build, template, name, cloned)
}

func (g *gatewayServices) listTemplates(ctx context.Context, opts *TemplateListOptions) ([]build.ListedTemplate, error) {
	return listTemplatesWithService(ctx, g.build, opts)
}

func (g *gatewayServices) getTemplate(ctx context.Context, ref string, opts *TemplateGetOptions) (*build.TemplateResponse, error) {
	return getTemplateWithService(ctx, g.build, ref, opts)
}

func (g *gatewayServices) deleteTemplate(ctx context.Context, ref string) error {
	return deleteTemplateWithService(ctx, g.build, ref)
}

func (g *gatewayServices) templateExists(ctx context.Context, ref string) (bool, error) {
	return templateExistsWithService(ctx, g.build, ref)
}

func (g *gatewayServices) getTemplateBuildStatus(ctx context.Context, templateID, buildID string, opts *TemplateBuildStatusOptions) (*build.BuildStatusResponse, error) {
	return getTemplateBuildStatusWithService(ctx, g.build, templateID, buildID, opts)
}

func cloneTemplateBuildOptions(opts *TemplateBuildOptions) *TemplateBuildOptions {
	if opts == nil {
		return &TemplateBuildOptions{}
	}
	cloned := *opts
	return &cloned
}

func resolveGatewayConfig(baseURL, apiKey string) (string, string) {
	resolvedBaseURL := strings.TrimSpace(baseURL)
	if resolvedBaseURL == "" {
		resolvedBaseURL = normalizeDomain(strings.TrimSpace(os.Getenv("E2B_DOMAIN")))
	}
	if resolvedBaseURL == "" {
		resolvedBaseURL = defaultBaseURL
	}

	resolvedAPIKey := strings.TrimSpace(apiKey)
	if resolvedAPIKey == "" {
		resolvedAPIKey = strings.TrimSpace(os.Getenv("E2B_API_KEY"))
	}
	return resolvedBaseURL, resolvedAPIKey
}

func normalizeDomain(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}

func defaultGatewayTransportOptions(projectID string, timeout time.Duration) []core.TransportOption {
	resolvedProjectID := strings.TrimSpace(projectID)
	if resolvedProjectID == "" {
		resolvedProjectID = strings.TrimSpace(os.Getenv("SEACLOUD_PROJECT_ID"))
	}

	var opts []core.TransportOption
	if resolvedProjectID != "" {
		opts = append(opts, core.WithProjectID(resolvedProjectID))
	}
	if timeout > 0 {
		opts = append(opts, core.WithTimeout(timeout))
	}
	return opts
}

func Create(ctx context.Context, templateID string, opts *CreateOptions, transportOpts ...core.TransportOption) (*Sandbox, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.create(ctx, templateID, opts)
}

func Connect(ctx context.Context, sandboxID string, opts *ConnectOptions, transportOpts ...core.TransportOption) (*Sandbox, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.connect(ctx, sandboxID, opts)
}

func List(ctx context.Context, opts *ListOptions, transportOpts ...core.TransportOption) ([]*SandboxHandle, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.list(ctx, opts)
}

func Get(ctx context.Context, sandboxID string, transportOpts ...core.TransportOption) (*SandboxDetail, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.get(ctx, sandboxID)
}

func BuildTemplate(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions, transportOpts ...core.TransportOption) (*TemplateBuildInfo, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.buildTemplate(ctx, template, name, opts)
}

func BuildTemplateInBackground(ctx context.Context, template *Template, name string, opts *TemplateBuildOptions, transportOpts ...core.TransportOption) (*TemplateBuildInfo, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.buildTemplateInBackground(ctx, template, name, opts)
}

func ListTemplates(ctx context.Context, opts *TemplateListOptions, transportOpts ...core.TransportOption) ([]build.ListedTemplate, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.listTemplates(ctx, opts)
}

func GetTemplate(ctx context.Context, ref string, opts *TemplateGetOptions, transportOpts ...core.TransportOption) (*build.TemplateResponse, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.getTemplate(ctx, ref, opts)
}

func DeleteTemplate(ctx context.Context, ref string, transportOpts ...core.TransportOption) error {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return err
	}
	return gateway.deleteTemplate(ctx, ref)
}

func TemplateExists(ctx context.Context, ref string, transportOpts ...core.TransportOption) (bool, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return false, err
	}
	return gateway.templateExists(ctx, ref)
}

func GetTemplateBuildStatus(ctx context.Context, templateID, buildID string, opts *TemplateBuildStatusOptions, transportOpts ...core.TransportOption) (*build.BuildStatusResponse, error) {
	gateway, err := newGatewayServicesFromEnv(transportOpts...)
	if err != nil {
		return nil, err
	}
	return gateway.getTemplateBuildStatus(ctx, templateID, buildID, opts)
}
