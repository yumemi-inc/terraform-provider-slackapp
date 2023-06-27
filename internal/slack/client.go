package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
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
	request any,
) (*http.Request, error) {
	requestBody, err := json.Marshal(&request)
	if err != nil {
		return nil, err
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx,
		httpMethod,
		c.createURL(methodName),
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return nil, err
	}

	httpRequest.Header.Set("Authorization", "Bearer "+*c.appConfigurationToken)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("User-Agent", "yumemi-inc/terraform-provider-slackapp")

	return httpRequest, nil
}
