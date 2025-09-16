package slack

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

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

    // Optional: log raw JSON response for debugging provider API interactions
    if os.Getenv("SLACKAPP_DEBUG_RAW_JSON") == "1" {
        tflog.Debug(ctx, string(responseBody))
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

	tflog.Debug(ctx, fmt.Sprintf("Read JSON response: %+v", response))

	return &response, nil
}
