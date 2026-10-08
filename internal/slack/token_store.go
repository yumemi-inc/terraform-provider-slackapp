package slack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// TokenStore keeps the tokens a rotation returns, so that the next process
// starts from them instead of from a refresh token Slack has voided. It
// holds bytes the client writes and reads back, and need not know what they
// mean.
type TokenStore interface {
	// Lock keeps other processes from rotating until unlock is called, when
	// the store can do that.
	Lock(ctx context.Context) (unlock func(), err error)
	// Load returns what Save last stored, or nil when nothing is stored.
	Load(ctx context.Context) ([]byte, error)
	Save(ctx context.Context, data []byte) error
}

// tokenStoreVersion is the version of tokenStoreRecord written now.
const tokenStoreVersion = 1

// tokenStoreSaveTimeout bounds the save of a rotation's result. It runs on
// past a cancelled context: the result holds the only live refresh token.
const tokenStoreSaveTimeout = 2 * time.Minute

// tokenStoreRefresher refreshes a tokenSource's tokens through its
// TokenStore, keeping the two in step. Under the store's lock, it uses the
// stored tokens while they are good, and otherwise rotates and saves the
// result.
//
//declscope:shared // token_source.go holds one
type tokenStoreRefresher struct {
	// seed is what tokenStoreRecord.SeedSHA256 holds for the configured
	// refresh token.
	seed string
	// rejected is the token Slack last refused as expired or revoked. A
	// record holding it is not used again.
	rejected AppConfigurationToken

	//declscope:private
	store TokenStore
	// read is whether the store was read yet. A token from the
	// configuration is used only after the store, which may hold a newer
	// one: Slack revokes a token once it is rotated.
	//
	//declscope:private
	read bool
	// unsaved is a rotation's result the store failed to take. It is the
	// only copy of the live refresh token, so it is saved before a token is
	// handed out again.
	//
	//declscope:private
	unsaved *tokenStoreRecord
}

//declscope:shared // token_source.go makes one when a store is set
func newTokenStoreRefresher(store TokenStore, configuredRefreshToken RefreshToken) *tokenStoreRefresher {
	return &tokenStoreRefresher{store: store, seed: tokenStoreSeed(configuredRefreshToken)}
}

// settled reports whether the tokens in memory may be used without
// visiting the store: it was read, and nothing waits to be saved.
//
//declscope:shared // token_source.go checks it before each call
func (s *tokenStoreRefresher) settled() bool {
	return s.read && s.unsaved == nil
}

// refresh returns the tokens to use in place of current: the stored ones
// while they are good, current on the first read when the store holds
// none that is, or rotator's result, which it saves. Waiting for the lock
// stops when ctx is cancelled; the rotation and the save do not.
//
// It returns tokens even with an error. When the store fails to take a
// rotation's result, those tokens are the only live ones; they are kept to
// save on the next call.
//
//declscope:shared // token_source.go refreshes through it
func (s *tokenStoreRefresher) refresh(
	ctx context.Context,
	current tokenSet,
	rotator func(ctx context.Context, refreshToken RefreshToken) (tokenSet, error),
) (tokenSet, error) {
	unlock, err := s.store.Lock(ctx)
	if err != nil {
		return current, fmt.Errorf("locking the token store: %w", err)
	}
	defer unlock()

	if s.unsaved != nil {
		// The tokens in memory are newer than anything in the store.
		return current, s.save(ctx, *s.unsaved)
	}

	// Read under the lock: another process may have rotated since this one
	// started.
	record, err := loadTokenStoreRecord(ctx, s.store)
	if err != nil {
		return current, err
	}

	s.read = true

	seed := s.seed
	refreshToken := current.refreshToken

	switch {
	case record == nil:
		tflog.Debug(ctx, "The token store holds no tokens yet.")
	// A record of unknown origin is kept: it may be the only live chain.
	case seed != "" && record.SeedSHA256 != "" && record.SeedSHA256 != seed:
		tflog.Debug(ctx, "The token store holds tokens older than the configured refresh token, ignoring them.")
	default:
		seed = record.SeedSHA256
		stored := record.tokens()

		if stored.usable() && stored.appConfigurationToken != s.rejected {
			tflog.Debug(ctx, "Using the app configuration token from the token store", map[string]any{
				"expires_at": stored.expiry(),
			})

			return stored, nil
		}

		if stored.refreshToken != "" {
			refreshToken = stored.refreshToken
		}

		// The record began from the configured tokens, so a token from the
		// configuration is older than it, and Slack has revoked it.
		current = tokenSet{refreshToken: current.refreshToken}
	}

	if current.usable() {
		tflog.Debug(ctx, "Using the app configuration token from the configuration.")

		return current, nil
	}

	next, err := rotator(ctx, refreshToken)
	if err != nil {
		return current, err
	}

	s.rejected = ""

	return next, s.save(ctx, tokenStoreRecordOf(next, seed))
}

// save keeps record in the store, to the end even when ctx is cancelled.
// When the store fails, it keeps record to save on the next call. The
// store's lock must be held.
func (s *tokenStoreRefresher) save(ctx context.Context, record tokenStoreRecord) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tokenStoreSaveTimeout)
	defer cancel()

	if err := saveTokenStoreRecord(ctx, s.store, record); err != nil {
		s.unsaved = &record

		return fmt.Errorf("rotated the app configuration token, but could not keep the new tokens. "+
			"Slack has voided the refresh token used, so the next call tries to keep them again: %w", err)
	}

	s.unsaved = nil

	return nil
}

// tokenStoreRecord is what the client keeps in a TokenStore.
type tokenStoreRecord struct {
	Version               int    `json:"version"`
	AppConfigurationToken string `json:"app_configuration_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresAt             int64  `json:"expires_at"`
	// SeedSHA256 is the SHA-256 of the refresh token the provider was
	// configured with when the chain of rotations began. A new refresh
	// token in the configuration means the user generated a new pair, so
	// the record is older than it and is not used. The token is random and
	// long, so its hash reveals nothing; a hash of anything guessable would.
	// It is "" when no refresh token was configured.
	SeedSHA256 string `json:"seed_sha256"`
}

func tokenStoreRecordOf(tokens tokenSet, seed string) tokenStoreRecord {
	record := tokenStoreRecord{
		Version:               tokenStoreVersion,
		AppConfigurationToken: string(tokens.appConfigurationToken),
		RefreshToken:          string(tokens.refreshToken),
		SeedSHA256:            seed,
	}
	if !tokens.appConfigurationTokenExpiresAt.IsZero() {
		record.ExpiresAt = tokens.appConfigurationTokenExpiresAt.Unix()
	}

	return record
}

func (r *tokenStoreRecord) tokens() tokenSet {
	tokens := tokenSet{
		appConfigurationToken: AppConfigurationToken(r.AppConfigurationToken),
		refreshToken:          RefreshToken(r.RefreshToken),
	}
	if r.ExpiresAt != 0 {
		tokens.appConfigurationTokenExpiresAt = time.Unix(r.ExpiresAt, 0)
	}

	return tokens
}

// tokenStoreSeed returns what tokenStoreRecord.SeedSHA256 holds for the
// configured refresh token, or "" when none is configured.
//
//declscope:shared // token_source.go sets the seed when the refresh token changes
func tokenStoreSeed(configuredRefreshToken RefreshToken) string {
	if configuredRefreshToken == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(configuredRefreshToken))

	return hex.EncodeToString(sum[:])
}

// loadTokenStoreRecord returns the record in store, or nil when it holds
// none.
func loadTokenStoreRecord(ctx context.Context, store TokenStore) (*tokenStoreRecord, error) {
	data, err := store.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading the token store: %w", err)
	}
	// An emptied file reads as an empty store, as a helper printing nothing
	// does.
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var record tokenStoreRecord
	// The error says nothing of the content: it holds the tokens.
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("the token store holds something that is not a token record")
	}
	if record.Version != tokenStoreVersion {
		return nil, fmt.Errorf("the token store holds a record of version %d, want %d", record.Version, tokenStoreVersion)
	}

	return &record, nil
}

func saveTokenStoreRecord(ctx context.Context, store TokenStore, record tokenStoreRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	if err := store.Save(ctx, data); err != nil {
		return fmt.Errorf("writing the token store: %w", err)
	}

	return nil
}
