package build

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	buildDirectionForward  = "forward"
	buildDirectionBackward = "backward"
	buildSourceTemporary   = "temporary"
	buildSourcePersistent  = "persistent"
	maxBuildLogsLimit      = 100
	maxBuildStatusLimit    = 100
	maxTemplateListLimit   = 100
	maxTemplateBuildLimit  = 100
)

var (
	dnsLabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	sha256Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

func (c *Service) Metrics(ctx context.Context) (string, error) {
	resp, err := c.DoRequest(ctx, http.MethodGet, "/metrics", nil, nil, nil, http.StatusOK)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *Service) DirectBuild(ctx context.Context, req *DirectBuildRequest) (*DirectBuildResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("sandbox: direct build request is required")
	}

	var resp DirectBuildResponse
	if _, err := c.DoJSON(ctx, http.MethodPost, "/build", nil, nil, req, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) CreateTemplate(ctx context.Context, req *TemplateCreateRequest) (*TemplateCreateResponse, error) {
	var resp TemplateCreateResponse
	if _, err := c.DoJSON(ctx, http.MethodPost, "/api/v1/templates", nil, nil, req, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListTemplates(ctx context.Context, params *ListTemplatesParams) ([]ListedTemplate, error) {
	if err := validateListTemplatesParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp []ListedTemplate
	if _, err := c.DoJSON(ctx, http.MethodGet, "/api/v1/templates", nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) GetTemplateByAlias(ctx context.Context, alias string) (*TemplateAliasResponse, error) {
	if strings.TrimSpace(alias) == "" {
		return nil, ErrAliasEmpty
	}

	var resp TemplateAliasResponse
	path := "/api/v1/templates/aliases/" + url.PathEscape(alias)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetTemplate(ctx context.Context, templateID string, params *GetTemplateParams) (*TemplateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if err := validateGetTemplateParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp TemplateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) UpdateTemplate(ctx context.Context, templateID string, req *TemplateUpdateRequest) (*TemplateUpdateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}

	var resp TemplateUpdateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID)
	if _, err := c.DoJSON(ctx, http.MethodPatch, path, nil, nil, req, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) DeleteTemplate(ctx context.Context, templateID string) error {
	if strings.TrimSpace(templateID) == "" {
		return ErrTemplateEmpty
	}

	path := "/api/v1/templates/" + url.PathEscape(templateID)
	_, err := c.DoRequest(ctx, http.MethodDelete, path, nil, nil, nil, http.StatusNoContent)
	return err
}

func (c *Service) CreateBuild(ctx context.Context, templateID string, req *BuildRequest) (*BuildTriggerResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if err := validateBuildRequest(req); err != nil {
		return nil, err
	}

	var body any
	if !isZeroBuildRequest(req) {
		body = req
	}

	var resp BuildTriggerResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds"
	if _, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, body, &resp, http.StatusAccepted); err != nil {
		return nil, err
	}
	resp.normalize()
	return &resp, nil
}

func (c *Service) GetBuildFile(ctx context.Context, templateID, hash string) (*FilePresenceResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(hash) == "" {
		return nil, ErrHashEmpty
	}
	if !sha256Pattern.MatchString(hash) {
		return nil, fmt.Errorf("sandbox: hash must be a 64-character lowercase hex SHA256")
	}

	var resp FilePresenceResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/files/" + url.PathEscape(hash)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) RollbackTemplate(ctx context.Context, templateID string, req *RollbackRequest) (*TemplateResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if req == nil || strings.TrimSpace(req.BuildID) == "" {
		return nil, ErrBuildEmpty
	}

	var resp TemplateResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/rollback"
	if _, err := c.DoJSON(ctx, http.MethodPost, path, nil, nil, req, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) ListBuilds(ctx context.Context, templateID string) (*BuildHistoryResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}

	var resp BuildHistoryResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuild(ctx context.Context, templateID, buildID string) (*BuildResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}

	var resp BuildResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID)
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, nil, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuildStatus(ctx context.Context, templateID, buildID string, params *BuildStatusParams) (*BuildStatusResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}
	if err := validateBuildStatusParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp BuildStatusResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID) + "/status"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Service) GetBuildLogs(ctx context.Context, templateID, buildID string, params *BuildLogsParams) (*BuildLogsResponse, error) {
	if strings.TrimSpace(templateID) == "" {
		return nil, ErrTemplateEmpty
	}
	if strings.TrimSpace(buildID) == "" {
		return nil, ErrBuildEmpty
	}
	if err := validateBuildLogsParams(params); err != nil {
		return nil, err
	}

	var query url.Values
	if params != nil {
		query = params.encode()
	}

	var resp BuildLogsResponse
	path := "/api/v1/templates/" + url.PathEscape(templateID) + "/builds/" + url.PathEscape(buildID) + "/logs"
	if _, err := c.DoJSON(ctx, http.MethodGet, path, nil, query, nil, &resp, http.StatusOK); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (p *ListTemplatesParams) encode() url.Values {
	values := make(url.Values)
	if visibility := strings.TrimSpace(p.Visibility); visibility != "" {
		values.Set("visibility", visibility)
	}
	if teamID := strings.TrimSpace(p.TeamID); teamID != "" {
		values.Set("teamID", teamID)
	}
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Offset > 0 {
		values.Set("offset", strconv.Itoa(p.Offset))
	}
	return values
}

func (p *GetTemplateParams) encode() url.Values {
	values := make(url.Values)
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if nextToken := strings.TrimSpace(p.NextToken); nextToken != "" {
		values.Set("nextToken", nextToken)
	}
	return values
}

func (p *BuildStatusParams) encode() url.Values {
	values := make(url.Values)
	if p.LogsOffset != nil {
		values.Set("logsOffset", strconv.Itoa(*p.LogsOffset))
	}
	if p.Limit != nil {
		values.Set("limit", strconv.Itoa(*p.Limit))
	}
	if level := strings.TrimSpace(p.Level); level != "" {
		values.Set("level", level)
	}
	return values
}

func (p *BuildLogsParams) encode() url.Values {
	values := make(url.Values)
	if p.Cursor != nil {
		values.Set("cursor", strconv.FormatInt(*p.Cursor, 10))
	}
	if p.Limit != nil {
		values.Set("limit", strconv.Itoa(*p.Limit))
	}
	if direction := strings.TrimSpace(p.Direction); direction != "" {
		values.Set("direction", direction)
	}
	if level := strings.TrimSpace(p.Level); level != "" {
		values.Set("level", level)
	}
	if source := strings.TrimSpace(p.Source); source != "" {
		values.Set("source", source)
	}
	return values
}

func validateListTemplatesParams(params *ListTemplatesParams) error {
	if params == nil {
		return nil
	}
	if params.Limit < 0 || params.Limit > maxTemplateListLimit {
		return fmt.Errorf("sandbox: template list limit must be between 0 and %d", maxTemplateListLimit)
	}
	if params.Offset < 0 {
		return fmt.Errorf("sandbox: template list offset must be non-negative")
	}
	return nil
}

func validateGetTemplateParams(params *GetTemplateParams) error {
	if params == nil {
		return nil
	}
	if params.Limit < 0 || params.Limit > maxTemplateBuildLimit {
		return fmt.Errorf("sandbox: template build history limit must be between 0 and %d", maxTemplateBuildLimit)
	}
	return nil
}

func validateBuildRequest(req *BuildRequest) error {
	if req == nil {
		return nil
	}
	if buildID := strings.TrimSpace(req.BuildID); buildID != "" {
		if len(buildID) > 63 || !dnsLabelPattern.MatchString(buildID) {
			return fmt.Errorf("sandbox: buildID must be a lowercase DNS label up to 63 characters")
		}
	}
	if registry := strings.TrimSpace(req.FromImageRegistry); registry != "" {
		return fmt.Errorf("sandbox: fromImageRegistry is not supported yet")
	}
	if req.Force != nil {
		return fmt.Errorf("sandbox: force rebuild is not supported yet")
	}

	hashes := make(map[string]struct{})
	if hash := strings.TrimSpace(req.FilesHash); hash != "" {
		if !sha256Pattern.MatchString(hash) {
			return fmt.Errorf("sandbox: filesHash must be a 64-character lowercase hex SHA256")
		}
		hashes[hash] = struct{}{}
	}

	for i, step := range req.Steps {
		stepType := strings.TrimSpace(step.Type)
		switch stepType {
		case "files", "context":
		case "":
			return fmt.Errorf("sandbox: steps[%d].type is required", i)
		default:
			return fmt.Errorf("sandbox: steps[%d].type must be files or context", i)
		}

		hash := strings.TrimSpace(step.FilesHash)
		if hash == "" {
			return fmt.Errorf("sandbox: steps[%d].filesHash is required", i)
		}
		if !sha256Pattern.MatchString(hash) {
			return fmt.Errorf("sandbox: steps[%d].filesHash must be a 64-character lowercase hex SHA256", i)
		}
		if len(step.Args) > 0 {
			return fmt.Errorf("sandbox: steps[%d].args is not supported yet", i)
		}
		if step.Force != nil {
			return fmt.Errorf("sandbox: steps[%d].force is not supported yet", i)
		}
		hashes[hash] = struct{}{}
	}

	if len(hashes) > 1 {
		return fmt.Errorf("sandbox: multiple different filesHash values are not supported yet")
	}
	return nil
}

func validateBuildStatusParams(params *BuildStatusParams) error {
	if params == nil {
		return nil
	}
	if params.LogsOffset != nil && *params.LogsOffset < 0 {
		return fmt.Errorf("sandbox: build logsOffset must be non-negative")
	}
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > maxBuildStatusLimit) {
		return fmt.Errorf("sandbox: build status limit must be between 0 and %d", maxBuildStatusLimit)
	}
	return nil
}

func validateBuildLogsParams(params *BuildLogsParams) error {
	if params == nil {
		return nil
	}
	if params.Cursor != nil && *params.Cursor < 0 {
		return fmt.Errorf("sandbox: build logs cursor must be non-negative")
	}
	if params.Limit != nil && (*params.Limit < 0 || *params.Limit > maxBuildLogsLimit) {
		return fmt.Errorf("sandbox: build logs limit must be between 0 and %d", maxBuildLogsLimit)
	}
	if direction := strings.TrimSpace(params.Direction); direction != "" &&
		direction != buildDirectionForward && direction != buildDirectionBackward {
		return fmt.Errorf("sandbox: build logs direction must be %q or %q", buildDirectionForward, buildDirectionBackward)
	}
	if source := strings.TrimSpace(params.Source); source != "" &&
		source != buildSourceTemporary && source != buildSourcePersistent {
		return fmt.Errorf("sandbox: build logs source must be %q or %q", buildSourceTemporary, buildSourcePersistent)
	}
	return nil
}

func isZeroBuildRequest(req *BuildRequest) bool {
	if req == nil {
		return true
	}
	return strings.TrimSpace(req.BuildID) == "" &&
		strings.TrimSpace(req.FromTemplate) == "" &&
		strings.TrimSpace(req.FromImage) == "" &&
		strings.TrimSpace(req.FromImageRegistry) == "" &&
		req.Force == nil &&
		len(req.Steps) == 0 &&
		strings.TrimSpace(req.FilesHash) == "" &&
		strings.TrimSpace(req.StartCmd) == "" &&
		strings.TrimSpace(req.ReadyCmd) == ""
}
