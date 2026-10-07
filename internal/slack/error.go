package slack

import "errors"

type ErrorResponse struct {
	Ok     bool   `json:"ok"`
	Error_ string `json:"error"`
	Errors []struct {
		Message string `json:"message"`
		Pointer string `json:"pointer"`
	} `json:"errors"`
}

func (e *ErrorResponse) IsOk() bool {
	return e.Ok
}

func (e *ErrorResponse) Error() string {
	return e.Error_
}

// IsAppNotFoundError reports whether err, or an error it wraps, is Slack
// answering app_not_found: the app does not exist, or the token cannot see
// it.
func IsAppNotFoundError(err error) bool {
	var slackErr *ErrorResponse

	return errors.As(err, &slackErr) && slackErr.Error_ == "app_not_found"
}
