package slack

import (
	"encoding/json"
	"io"
	"net/http"
)

type Response interface {
	IsOk() bool
}

func readJSONResponse[T Response](httpResponse *http.Response) (*T, error) {
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

		return nil, &errorResponse
	}

	return &response, nil
}
