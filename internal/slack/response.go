package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type Response interface {
	IsOk() bool
}

func readJSONResponse[T Response](ctx context.Context, httpResponse *http.Response) (*T, error) {
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

		tflog.Debug(ctx, fmt.Sprintf("Read JSON error response: %+v", errorResponse))

		return nil, &errorResponse
	}

	// Do not log the response body: some responses (e.g. apps.manifest.create,
	// tooling.tokens.rotate) contain secrets such as client_secret, signing_secret,
	// and tokens. tflog does not honor the schema "Sensitive" flag.
	tflog.Debug(ctx, "Read JSON response successfully.")

	return &response, nil
}
