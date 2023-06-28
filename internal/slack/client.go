package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const defaultBaseURL = "https://slack.com/api/"

type Client struct {
	baseURL               string
	appConfigurationToken *string
	refreshToken          *string
	httpClient            *http.Client
}

func NewClient(appConfigurationToken string) *Client {
	return &Client{
		baseURL:               defaultBaseURL,
		appConfigurationToken: &appConfigurationToken,
		httpClient:            http.DefaultClient,
	}
}

func NewClientFromRefreshToken(refreshToken string) *Client {
	return &Client{
		baseURL:      defaultBaseURL,
		refreshToken: &refreshToken,
		httpClient:   http.DefaultClient,
	}
}

func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL

	return c
}

func (c *Client) createURL(methodName string) string {
	return c.baseURL + methodName
}

func (c *Client) createRequest(
	ctx context.Context,
	httpMethod string,
	methodName string,
	requestBody io.Reader,
) (*http.Request, error) {
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		httpMethod,
		c.createURL(methodName),
		requestBody,
	)
	if err != nil {
		return nil, err
	}

	if c.appConfigurationToken != nil {
		httpRequest.Header.Set("Authorization", "Bearer "+*c.appConfigurationToken)
	}

	httpRequest.Header.Set("User-Agent", "yumemi-inc/terraform-provider-slackapp")

	return httpRequest, nil
}

func (c *Client) createJSONRequest(
	ctx context.Context,
	httpMethod string, //nolint:unparam
	methodName string,
	request any,
) (*http.Request, error) {
	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, err
	}

	httpRequest, err := c.createRequest(ctx, httpMethod, methodName, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	return httpRequest, nil
}

func (c *Client) createFormRequest(
	ctx context.Context,
	httpMethod string, //nolint:unparam
	methodName string,
	request url.Values,
) (*http.Request, error) {
	httpRequest, err := c.createRequest(ctx, httpMethod, methodName, bytes.NewBufferString(request.Encode()))
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return httpRequest, nil
}

func (c *Client) refreshAppConfigurationToken(ctx context.Context) error {
	if c.refreshToken == nil {
		return nil
	}

	response, err := c.ToolingTokensRotate(ctx, *c.refreshToken)
	if err != nil {
		return err
	}

	tflog.Debug(ctx, fmt.Sprintf("%+v", response))

	c.appConfigurationToken = &response.Token
	c.refreshToken = &response.RefreshToken

	return nil
}

func (c *Client) ensureAppConfigurationToken(ctx context.Context) error {
	if c.appConfigurationToken == nil {
		tflog.Debug(ctx, "No app configuration token is available, refreshing token.")

		if err := c.refreshAppConfigurationToken(ctx); err != nil {
			return err
		}

		return nil
	}

	tflog.Debug(ctx, "App configuration token is already provided, continuing.")

	return nil
}
