package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

//declscope:shared // methods.go calls every method through it
type response interface {
	IsOk() bool
	// logFields returns the fields of a reply that are safe to log. Replies
	// carry tokens and app credentials, so readJSONResponse never logs a
	// whole reply; each type names what may be logged instead.
	//
	//declscope:private
	logFields() map[string]any
}

//declscope:shared
func readJSONResponse[T response](ctx context.Context, methodName string, httpResponse *http.Response) (*T, error) {
	defer func() { _ = httpResponse.Body.Close() }()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, err
	}

	var response T
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}

	if !response.IsOk() {
		var errorResponse ErrorResponse
		if err := json.Unmarshal(responseBody, &errorResponse); err != nil {
			return nil, err
		}

		tflog.Debug(ctx, "Slack API method returned an error", map[string]any{
			"method": methodName,
			"error":  errorResponse.Error_,
			"errors": errorResponse.Errors,
		})

		return nil, &errorResponse
	}

	fields := response.logFields()
	if fields == nil {
		fields = map[string]any{}
	}
	fields["method"] = methodName
	tflog.Debug(ctx, "Slack API method succeeded", fields)

	return &response, nil
}
