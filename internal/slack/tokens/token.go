// Package tokens holds the tokens that authenticate the Slack client,
// and decides which one each call carries.
//
// Slack's app configuration token lasts 12 hours. A refresh token is traded
// for a new pair by tooling.tokens.rotate, and works once. A Source hands
// out the token for each call, rotates for a new one when it must, and,
// with a Store, keeps the newest pair for the next process.
//
// The package knows nothing of HTTP: the caller gives a Source the function
// that rotates.
package tokens

import "encoding/json"

// tokenRedacted is what a token prints as. The tokens can change every app
// in the workspace, so they must never reach a log or an error by accident.
const tokenRedacted = "[redacted]"

// AppConfigurationToken authenticates calls to the apps.manifest.* methods.
// It lasts 12 hours. It prints as [redacted]; string(t) gives the value.
type AppConfigurationToken string

func (t AppConfigurationToken) String() string { return tokenRedacted }

func (t AppConfigurationToken) GoString() string { return tokenRedacted }

func (t AppConfigurationToken) MarshalJSON() ([]byte, error) { return json.Marshal(tokenRedacted) }

// RefreshToken is traded by tooling.tokens.rotate for a new app
// configuration token and a new refresh token. It does not expire, but
// works once. It prints as [redacted]; string(t) gives the value.
type RefreshToken string

func (t RefreshToken) String() string { return tokenRedacted }

func (t RefreshToken) GoString() string { return tokenRedacted }

func (t RefreshToken) MarshalJSON() ([]byte, error) { return json.Marshal(tokenRedacted) }
