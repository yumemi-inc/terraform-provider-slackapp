package slack

import "time"

// tokenSetExpiryMargin is how long before its expiry an app configuration
// token is rotated, so that a call does not start with a token that
// expires on the way.
const tokenSetExpiryMargin = 5 * time.Minute

// tokenSet is the tokens the client holds at one time. They change
// together: a rotation, or a read of the store, replaces all three.
//
//declscope:shared // token_source.go holds one, and token_store.go stores one
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

// usable reports whether the app configuration token may be sent: there is
// one, and it is not due for rotation.
//
//declscope:shared // token_source.go and token_store.go check tokens with it
func (t tokenSet) usable() bool {
	if t.appConfigurationToken == "" {
		return false
	}

	expiresAt := t.appConfigurationTokenExpiresAt

	return expiresAt.IsZero() || time.Now().Add(tokenSetExpiryMargin).Before(expiresAt)
}

// expiry formats appConfigurationTokenExpiresAt for the log. The log takes
// the expiry, never the tokens.
//
//declscope:shared // token_source.go and token_store.go log it
func (t tokenSet) expiry() string {
	if t.appConfigurationTokenExpiresAt.IsZero() {
		return "unknown"
	}

	return t.appConfigurationTokenExpiresAt.UTC().Format(time.RFC3339)
}
