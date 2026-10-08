package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const clientDefaultBaseURL = "https://slack.com/api/"

// Client calls Slack's API methods. It builds and sends the requests; its
// tokenSource decides which app configuration token each one carries, and
// when to rotate for a new one.
type Client struct {
	//declscope:shared
	httpClient *http.Client
	//declscope:shared
	tokens *tokenSource

	baseURL string
}

// NewClient returns a client with no token. Give it an app configuration
// token or a refresh token before a call.
func NewClient() *Client {
	c := &Client{
		baseURL:    clientDefaultBaseURL,
		httpClient: http.DefaultClient,
	}
	c.tokens = newTokenSource(c.rotateTokens)

	return c
}

// WithBaseURL makes the client call baseURL instead of Slack's API, as the
// tests do against a fake.
func (c *Client) WithBaseURL(baseURL string) *Client {
	c.baseURL = baseURL

	return c
}

// WithAppConfigurationToken makes the client call methods with token.
func (c *Client) WithAppConfigurationToken(token AppConfigurationToken) *Client {
	c.tokens.setAppConfigurationToken(token)

	return c
}

// WithRefreshToken lets the client rotate refreshToken for an app
// configuration token when it has none.
func (c *Client) WithRefreshToken(refreshToken RefreshToken) *Client {
	c.tokens.setRefreshToken(refreshToken)

	return c
}

// createJSONRequest builds a request for methodName with request as its
// JSON body, sent with token. Every apps.manifest.* method takes one.
//
//declscope:shared
func (c *Client) createJSONRequest(
	ctx context.Context,
	httpMethod string, //nolint:unparam
	methodName string,
	token AppConfigurationToken,
	request any,
) (*http.Request, error) {
	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, err
	}

	httpRequest, err := c.createRequest(ctx, httpMethod, methodName, token, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/json")

	return httpRequest, nil
}

// createFormRequest builds a request for methodName with request as its
// form body, and no token. tooling.tokens.rotate takes one: the refresh
// token in the form is what authenticates it.
//
//declscope:shared
func (c *Client) createFormRequest(
	ctx context.Context,
	httpMethod string, //nolint:unparam
	methodName string,
	request url.Values,
) (*http.Request, error) {
	httpRequest, err := c.createRequest(ctx, httpMethod, methodName, "", bytes.NewBufferString(request.Encode()))
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return httpRequest, nil
}

func (c *Client) createURL(methodName string) string {
	return c.baseURL + methodName
}

// createRequest builds a request for methodName, sent with token unless
// token is "".
func (c *Client) createRequest(
	ctx context.Context,
	httpMethod string,
	methodName string,
	token AppConfigurationToken,
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

	tflog.Debug(ctx, "Calling a Slack API method", map[string]any{"method": methodName})

	if token != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+string(token))
	}

	httpRequest.Header.Set("User-Agent", "ymm-oss/terraform-provider-slackapp")

	return httpRequest, nil
}

// rotateTokens trades refreshToken for a new set of tokens.
func (c *Client) rotateTokens(ctx context.Context, refreshToken RefreshToken) (tokenSet, error) {
	response, err := c.ToolingTokensRotate(ctx, refreshToken)
	if err != nil {
		return tokenSet{}, err
	}

	return tokenSet{
		appConfigurationToken:          response.Token,
		appConfigurationTokenExpiresAt: *response.ExpiresAt.Time(),
		refreshToken:                   response.RefreshToken,
	}, nil
}
