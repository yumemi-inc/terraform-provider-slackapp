package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type Client struct {
	appConfigurationToken *string
	refreshToken          *string
	httpClient            *http.Client
}

func NewClient(appConfigurationToken string) *Client {
	return &Client{
		appConfigurationToken: &appConfigurationToken,
		httpClient:            http.DefaultClient,
	}
}

func NewClientFromRefreshToken(refreshToken string) *Client {
	return &Client{
		refreshToken: &refreshToken,
		httpClient:   http.DefaultClient,
	}
}

func (c *Client) createURL(methodName string) string {
	return "https://slack.com/api/" + methodName
}

func (c *Client) createRequest(
	ctx context.Context,
	httpMethod string,
	methodName string,
	requestBody io.Reader,
) (*http.Request, error) {
	if c.appConfigurationToken == nil {
		if err := c.refreshAppConfigurationToken(ctx); err != nil {
			return nil, err
		}
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		httpMethod,
		c.createURL(methodName),
		requestBody,
	)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Authorization", "Bearer "+*c.appConfigurationToken)
	httpRequest.Header.Set("Content-Type", "application/json")
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

	return c.createRequest(ctx, httpMethod, methodName, bytes.NewBuffer(requestBody))
}

func (c *Client) createFormRequest(
	ctx context.Context,
	httpMethod string, //nolint:unparam
	methodName string,
	request url.Values,
) (*http.Request, error) {
	return c.createRequest(ctx, httpMethod, methodName, bytes.NewBufferString(request.Encode()))
}

func (c *Client) refreshAppConfigurationToken(ctx context.Context) error {
	if c.refreshToken == nil {
		return nil
	}

	response, err := c.ToolingTokensRotate(ctx, *c.refreshToken)
	if err != nil {
		return err
	}

	*c.appConfigurationToken = response.Token
	*c.refreshToken = response.RefreshToken

	return nil
}
