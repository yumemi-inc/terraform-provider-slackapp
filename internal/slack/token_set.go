package slack

import "time"

// tokenSet is the tokens the client holds at one time. They change
// together: a rotation replaces all three.
//
//declscope:shared // token_source.go holds one, and client.go makes one
type tokenSet struct {
	// appConfigurationToken is "" when there is none.
	appConfigurationToken AppConfigurationToken
	// appConfigurationTokenExpiresAt is zero when the expiry of
	// appConfigurationToken is not known, as for a token from the
	// configuration.
	appConfigurationTokenExpiresAt time.Time
	// refreshToken is "" when there is none. It does not expire, but works
	// once.
	refreshToken RefreshToken
}

// usable reports whether there is an app configuration token to send.
//
//declscope:shared // token_source.go checks tokens with it
func (t tokenSet) usable() bool {
	return t.appConfigurationToken != ""
}

// expiry formats appConfigurationTokenExpiresAt for the log. The log takes
// the expiry, never the tokens.
//
//declscope:shared // token_source.go logs it
func (t tokenSet) expiry() string {
	if t.appConfigurationTokenExpiresAt.IsZero() {
		return "unknown"
	}

	return t.appConfigurationTokenExpiresAt.UTC().Format(time.RFC3339)
}
