package slack

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// tokenSource hands out the app configuration token for each call, and
// gets one when it has none.
//
// newTokenSource takes the one thing a source needs: a rotator. The rest
// is optional, and set after: an app configuration token, a refresh token.
// A source with neither has no token to hand out.
//
// Terraform calls the provider from several goroutines, and a refresh
// token works once, so only one of them may rotate it at a time.
//
//declscope:shared // client.go builds one, and methods.go takes tokens from it
type tokenSource struct {
	// rotator trades a refresh token for a new set of tokens. Client gives
	// it, so that the source knows nothing of HTTP.
	//
	//declscope:private
	rotator func(ctx context.Context, refreshToken RefreshToken) (tokenSet, error)

	// mu guards the fields below.
	//
	//declscope:private
	mu sync.Mutex
	//declscope:private
	current tokenSet
}

//declscope:shared // client.go builds the client's with it
func newTokenSource(rotator func(ctx context.Context, refreshToken RefreshToken) (tokenSet, error)) *tokenSource {
	return &tokenSource{rotator: rotator}
}

// setAppConfigurationToken makes the source hand out appConfigurationToken.
// Its expiry is not known.
//
//declscope:shared // client.go configures the source with it
func (s *tokenSource) setAppConfigurationToken(appConfigurationToken AppConfigurationToken) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current.appConfigurationToken = appConfigurationToken
	s.current.appConfigurationTokenExpiresAt = time.Time{}
}

// setRefreshToken lets the source rotate refreshToken.
//
//declscope:shared // client.go configures the source with it
func (s *tokenSource) setRefreshToken(refreshToken RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current.refreshToken = refreshToken
}

// get returns the token to call a method with. It gets one first when the
// source has none.
//
//declscope:shared // methods.go calls every method with it
func (s *tokenSource) get(ctx context.Context) (AppConfigurationToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current.usable() {
		return s.current.appConfigurationToken, nil
	}

	tflog.Debug(ctx, "No app configuration token is usable, refreshing token.")

	next, err := s.rotateFrom(ctx, s.current.refreshToken)
	if err != nil {
		return "", err
	}

	s.current = next

	return s.current.appConfigurationToken, nil
}

// rotateFrom rotates refreshToken. s.mu must be held.
func (s *tokenSource) rotateFrom(ctx context.Context, refreshToken RefreshToken) (tokenSet, error) {
	if refreshToken == "" {
		return tokenSet{}, errors.New("no app configuration token is usable, and there is no refresh token to rotate for one")
	}

	next, err := s.rotator(ctx, refreshToken)
	if err != nil {
		return tokenSet{}, err
	}

	tflog.Debug(ctx, "Rotated the app configuration token", map[string]any{"expires_at": next.expiry()})

	return next, nil
}
