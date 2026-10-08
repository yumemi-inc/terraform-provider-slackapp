package tokens

import "time"

// setExpiryMargin is how long before its expiry an app configuration token
// is rotated, so that a call does not start with a token that expires on
// the way.
const setExpiryMargin = 5 * time.Minute

// Set is the tokens held at one time. They change together: a rotation, or
// a read of the store, replaces all three.
type Set struct {
	// AppConfigurationToken is "" when there is none.
	AppConfigurationToken AppConfigurationToken
	// AppConfigurationTokenExpiresAt is zero when the expiry of
	// AppConfigurationToken is not known, as for a token from the
	// configuration.
	AppConfigurationTokenExpiresAt time.Time
	// RefreshToken is "" when there is none. It does not expire, but works
	// once.
	RefreshToken RefreshToken
}

// usable reports whether the app configuration token may be sent: there is
// one, and it is not due for rotation.
//
//declscope:shared // source.go and store.go check tokens with it
func (t Set) usable() bool {
	if t.AppConfigurationToken == "" {
		return false
	}

	expiresAt := t.AppConfigurationTokenExpiresAt

	return expiresAt.IsZero() || time.Now().Add(setExpiryMargin).Before(expiresAt)
}

// expiry formats AppConfigurationTokenExpiresAt for the log. The log takes
// the expiry, never the tokens.
//
//declscope:shared // source.go and store.go log it
func (t Set) expiry() string {
	if t.AppConfigurationTokenExpiresAt.IsZero() {
		return "unknown"
	}

	return t.AppConfigurationTokenExpiresAt.UTC().Format(time.RFC3339)
}
