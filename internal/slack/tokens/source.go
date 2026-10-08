package tokens

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// sourceRotationTimeout bounds a rotation. It runs on past a cancelled
// context: Slack voids the refresh token as soon as it takes it, so a
// rotation stopped halfway loses the only live token.
const sourceRotationTimeout = 2 * time.Minute

// Source hands out the app configuration token for each call, and gets a
// new one when it has none, when it is about to expire, or when Slack
// refuses it.
//
// NewSource takes the one thing a source needs: a rotator. The rest is
// optional, and set after: an app configuration token, a refresh token, a
// store. A source with none of them has no token to hand out.
//
// Terraform calls the provider from several goroutines, and a refresh
// token works once, so only one of them may rotate it at a time.
type Source struct {
	// rotator trades a refresh token for a new set of tokens. The caller
	// gives it, so that the source knows nothing of HTTP.
	rotator func(ctx context.Context, refreshToken RefreshToken) (Set, error)

	// mu guards the fields below.
	mu      sync.Mutex
	current Set
	// store refreshes the tokens through the token store. It is nil when
	// the source has none.
	store *storeRefresher
}

func NewSource(rotator func(ctx context.Context, refreshToken RefreshToken) (Set, error)) *Source {
	return &Source{rotator: rotator}
}

// SetAppConfigurationToken makes the source hand out appConfigurationToken
// until Slack refuses it. Its expiry is not known.
func (s *Source) SetAppConfigurationToken(appConfigurationToken AppConfigurationToken) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current.AppConfigurationToken = appConfigurationToken
	s.current.AppConfigurationTokenExpiresAt = time.Time{}
}

// SetRefreshToken lets the source rotate refreshToken.
func (s *Source) SetRefreshToken(refreshToken RefreshToken) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.current.RefreshToken = refreshToken
	if s.store != nil {
		s.store.seed = storeSeed(refreshToken)
	}
}

// SetStore makes the source keep each rotation's tokens in store, and start
// from what store holds.
func (s *Source) SetStore(store Store) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.store = newStoreRefresher(store, s.current.RefreshToken)
}

// Get returns the token to call a method with. It gets a new one first when
// the source has none that is usable.
func (s *Source) Get(ctx context.Context) (AppConfigurationToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current.usable() && (s.store == nil || s.store.settled()) {
		return s.current.AppConfigurationToken, nil
	}

	tflog.Debug(ctx, "No app configuration token is usable, refreshing token.")

	var err error
	if s.store == nil {
		var next Set
		if next, err = s.rotateFrom(ctx, s.current.RefreshToken); err == nil {
			s.current = next
		}
	} else {
		// The tokens come back even with an error: they may be newer than
		// the store, which failed to take them.
		s.current, err = s.store.refresh(ctx, s.current, s.rotateFrom)
	}

	if err != nil {
		return "", err
	}

	return s.current.AppConfigurationToken, nil
}

// MarkAsRefused drops appConfigurationToken after Slack refused it. It
// reports whether the source may get another, by a refresh token or from
// its store.
func (s *Source) MarkAsRefused(appConfigurationToken AppConfigurationToken) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.current.RefreshToken == "" && s.store == nil {
		return false
	}

	// Another call may have replaced the token already.
	if s.current.AppConfigurationToken == appConfigurationToken {
		s.current = Set{RefreshToken: s.current.RefreshToken}
		if s.store != nil {
			s.store.rejected = appConfigurationToken
		}
	}

	return true
}

// rotateFrom rotates refreshToken, to the end even when ctx is cancelled.
// s.mu must be held.
func (s *Source) rotateFrom(ctx context.Context, refreshToken RefreshToken) (Set, error) {
	if refreshToken == "" && s.store != nil {
		return Set{}, errors.New("no app configuration token is usable, and neither the token store nor the configuration holds a refresh token to rotate for one")
	}
	if refreshToken == "" {
		return Set{}, errors.New("no app configuration token is usable, and there is no refresh token to rotate for one")
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sourceRotationTimeout)
	defer cancel()

	next, err := s.rotator(ctx, refreshToken)
	if err != nil {
		return Set{}, err
	}

	tflog.Debug(ctx, "Rotated the app configuration token", map[string]any{"expires_at": next.expiry()})

	return next, nil
}
