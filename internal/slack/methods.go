package slack

import (
	"context"
	"net/http"
	"net/url"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
)

type AppsManifestCreateRequest struct {
	Manifest string `json:"manifest"`
}

type AppsManifestCreateResponse struct {
	Ok          bool   `json:"ok"`
	AppID       string `json:"app_id"`
	Credentials struct {
		ClientID          string `json:"client_id"`
		ClientSecret      string `json:"client_secret"`
		VerificationToken string `json:"verification_token"`
		SigningSecret     string `json:"signing_secret"`
	} `json:"credentials"`
	OauthAuthorizeURL string `json:"oauth_authorize_url"`
}

func (r AppsManifestCreateResponse) IsOk() bool {
	return r.Ok
}

func (c *Client) AppsManifestCreate(
	ctx context.Context,
	request AppsManifestCreateRequest,
) (*AppsManifestCreateResponse, error) {
	httpRequest, err := c.createJSONRequest(ctx, http.MethodPost, "apps.manifest.create", &request)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[AppsManifestCreateResponse](httpResponse)
}

type AppsManifestUpdateRequest struct {
	AppID    string `json:"app_id"`
	Manifest string `json:"manifest"`
}

type AppsManifestUpdateResponse struct {
	Ok                 bool   `json:"ok"`
	AppID              string `json:"app_id"`
	PermissionsUpdated bool   `json:"permissions_updated"`
}

func (r AppsManifestUpdateResponse) IsOk() bool {
	return r.Ok
}

func (c *Client) AppsManifestUpdate(
	ctx context.Context,
	request AppsManifestUpdateRequest,
) (*AppsManifestUpdateResponse, error) {
	httpRequest, err := c.createJSONRequest(ctx, http.MethodPost, "apps.manifest.create", &request)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[AppsManifestUpdateResponse](httpResponse)
}

type AppsManifestExportRequest struct {
	AppID string `json:"app_id"`
}

type AppsManifestExportResponse struct {
	Ok       bool          `json:"ok"`
	Manifest *manifest.App `json:"manifest"`
}

func (r AppsManifestExportResponse) IsOk() bool {
	return r.Ok
}

func (c *Client) AppsManifestExport(
	ctx context.Context,
	request AppsManifestExportRequest,
) (*AppsManifestExportResponse, error) {
	httpRequest, err := c.createJSONRequest(ctx, http.MethodPost, "apps.manifest.export", &request)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[AppsManifestExportResponse](httpResponse)
}

type AppsManifestDeleteRequest struct {
	AppID string `json:"app_id"`
}

type AppsManifestDeleteResponse struct {
	Ok bool `json:"ok"`
}

func (r AppsManifestDeleteResponse) IsOk() bool {
	return r.Ok
}

func (c *Client) AppsManifestDelete(
	ctx context.Context,
	request AppsManifestDeleteRequest,
) (*AppsManifestDeleteResponse, error) {
	httpRequest, err := c.createJSONRequest(ctx, http.MethodPost, "apps.manifest.delete", &request)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[AppsManifestDeleteResponse](httpResponse)
}

type ToolingTokensRotateResponse struct {
	Ok           bool          `json:"ok"`
	Token        string        `json:"token"`
	RefreshToken string        `json:"refresh_token"`
	IssuedAt     UnixTimestamp `json:"iat"`
	ExpiresAt    UnixTimestamp `json:"exp"`
}

func (r ToolingTokensRotateResponse) IsOk() bool {
	return r.Ok
}

func (c *Client) ToolingTokensRotate(
	ctx context.Context,
	refreshToken string,
) (*ToolingTokensRotateResponse, error) {
	values := url.Values{}
	values.Set("refresh_token", refreshToken)

	httpRequest, err := c.createFormRequest(ctx, http.MethodPost, "apps.manifest.delete", values)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[ToolingTokensRotateResponse](httpResponse)
}
