//declscope:core // the slack package's API: Slack Web API methods and the types they share

package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
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

// logFields leaves out Credentials: they are the app's secrets.
func (r AppsManifestCreateResponse) logFields() map[string]any {
	return map[string]any{"app_id": r.AppID}
}

func (c *Client) AppsManifestCreate(
	ctx context.Context,
	request AppsManifestCreateRequest,
) (*AppsManifestCreateResponse, error) {
	return callMethod[AppsManifestCreateResponse](ctx, c, "apps.manifest.create", &request)
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

func (r AppsManifestUpdateResponse) logFields() map[string]any {
	return map[string]any{"app_id": r.AppID, "permissions_updated": r.PermissionsUpdated}
}

func (c *Client) AppsManifestUpdate(
	ctx context.Context,
	request AppsManifestUpdateRequest,
) (*AppsManifestUpdateResponse, error) {
	return callMethod[AppsManifestUpdateResponse](ctx, c, "apps.manifest.update", &request)
}

type AppsManifestExportRequest struct {
	AppID string `json:"app_id"`
}

type AppsManifestExportResponse struct {
	Ok bool `json:"ok"`
	// Manifest is kept as the raw JSON Slack returned. Decoding it into
	// a struct would drop every field that struct does not model.
	Manifest json.RawMessage `json:"manifest"`
}

func (r AppsManifestExportResponse) IsOk() bool {
	return r.Ok
}

func (r AppsManifestExportResponse) logFields() map[string]any {
	return nil
}

func (c *Client) AppsManifestExport(
	ctx context.Context,
	request AppsManifestExportRequest,
) (*AppsManifestExportResponse, error) {
	return callMethod[AppsManifestExportResponse](ctx, c, "apps.manifest.export", &request)
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

func (r AppsManifestDeleteResponse) logFields() map[string]any {
	return nil
}

func (c *Client) AppsManifestDelete(
	ctx context.Context,
	request AppsManifestDeleteRequest,
) (*AppsManifestDeleteResponse, error) {
	return callMethod[AppsManifestDeleteResponse](ctx, c, "apps.manifest.delete", &request)
}

type ToolingTokensRotateResponse struct {
	Ok           bool                  `json:"ok"`
	Token        AppConfigurationToken `json:"token"`
	RefreshToken RefreshToken          `json:"refresh_token"`
	IssuedAt     UnixTimestamp         `json:"iat"`
	ExpiresAt    UnixTimestamp         `json:"exp"`
}

func (r ToolingTokensRotateResponse) IsOk() bool {
	return r.Ok
}

// logFields leaves out Token and RefreshToken. The token can change every
// app in the workspace, and the refresh token can mint new tokens.
func (r ToolingTokensRotateResponse) logFields() map[string]any {
	return map[string]any{
		"issued_at":  r.IssuedAt.Time().UTC().Format(time.RFC3339),
		"expires_at": r.ExpiresAt.Time().UTC().Format(time.RFC3339),
	}
}

func (c *Client) ToolingTokensRotate(
	ctx context.Context,
	refreshToken RefreshToken,
) (*ToolingTokensRotateResponse, error) {
	values := url.Values{}
	values.Set("refresh_token", string(refreshToken))

	httpRequest, err := c.createFormRequest(ctx, http.MethodPost, "tooling.tokens.rotate", values)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[ToolingTokensRotateResponse](ctx, "tooling.tokens.rotate", httpResponse)
}

// callMethod calls methodName with the client's app configuration token.
func callMethod[T response](ctx context.Context, c *Client, methodName string, request any) (*T, error) {
	token, err := c.tokens.get(ctx)
	if err != nil {
		return nil, err
	}

	return callMethodWithToken[T](ctx, c, methodName, token, request)
}

func callMethodWithToken[T response](
	ctx context.Context,
	c *Client,
	methodName string,
	token AppConfigurationToken,
	request any,
) (*T, error) {
	httpRequest, err := c.createJSONRequest(ctx, http.MethodPost, methodName, token, request)
	if err != nil {
		return nil, err
	}

	httpResponse, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}

	return readJSONResponse[T](ctx, methodName, httpResponse)
}
