package listvalidatorx

func appendHint(message, hint string) string {
	if hint != "" {
		message += "\n\nHint: " + message
	}
	return message
}
