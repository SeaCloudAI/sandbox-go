package sandbox

import (
	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

type Client struct {
	*control.Service
	Build *build.Service
}

func NewClient(baseURL, apiKey string, opts ...core.TransportOption) (*Client, error) {
	controlService, err := control.NewService(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	buildOps, err := build.NewService(baseURL, apiKey, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		Service: controlService,
		Build:   buildOps,
	}, nil
}

func (c *Client) NewCMD(baseURL, accessToken string) (*cmd.Service, error) {
	return cmd.NewService(baseURL, accessToken)
}

func (c *Client) Runtime(baseURL, accessToken string) (*Runtime, error) {
	service, err := cmd.NewService(baseURL, accessToken)
	if err != nil {
		return nil, err
	}
	return &Runtime{Service: service}, nil
}
