package slack

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
