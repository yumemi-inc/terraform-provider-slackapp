package tokens_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/tokens"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/tokens/storage"
)

// storeMemory is a tokens.Store that keeps the record in memory, and
// fails where a test tells it to. Its Save fails on a cancelled context, as
// a real store's would.
type storeMemory struct {
	mu      sync.Mutex
	data    []byte
	saves   int
	lockErr error
	loadErr error
	saveErr error
}

func (m *storeMemory) Lock(context.Context) (func(), error) {
	if m.lockErr != nil {
		return nil, m.lockErr
	}

	return func() {}, nil
}

func (m *storeMemory) Load(context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.data, m.loadErr
}

func (m *storeMemory) Save(ctx context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if m.saveErr != nil {
		return m.saveErr
	}
	m.data = data
	m.saves++

	return nil
}

// storeRecord is the record as a Source writes it.
type storeRecord struct {
	Version               int    `json:"version"`
	AppConfigurationToken string `json:"app_configuration_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresAt             int64  `json:"expires_at"`
	SeedSHA256            string `json:"seed_sha256"`
}

func newStoreMemory(t *testing.T, record *storeRecord) *storeMemory {
	t.Helper()

	m := &storeMemory{}
	if record != nil {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		m.data = data
	}

	return m
}

func (m *storeMemory) record(t *testing.T) storeRecord {
	t.Helper()

	m.mu.Lock()
	defer m.mu.Unlock()

	var record storeRecord
	if err := json.Unmarshal(m.data, &record); err != nil {
		t.Fatalf("the store holds %q: %v", m.data, err)
	}

	return record
}

func storeSeedOf(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))

	return hex.EncodeToString(sum[:])
}

// storeRotator stands in for tooling.tokens.rotate. It trades every refresh
// token for xoxe-new and refresh-2, and records what it was given. It fails
// on a cancelled context, as a call to Slack would.
type storeRotator struct {
	mu      sync.Mutex
	rotated []string
	// noExpiry makes it answer with no expiry, as Slack may.
	noExpiry bool
}

func (r *storeRotator) rotate(ctx context.Context, refreshToken tokens.RefreshToken) (tokens.Set, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return tokens.Set{}, err
	}
	r.rotated = append(r.rotated, string(refreshToken))

	next := tokens.Set{AppConfigurationToken: "xoxe-new", RefreshToken: "refresh-2"}
	if !r.noExpiry {
		next.AppConfigurationTokenExpiresAt = time.Unix(4102444800, 0)
	}

	return next, nil
}

func (r *storeRotator) rotations() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return strings.Join(r.rotated, ",")
}

// storeSource returns a source with refreshToken, unless it is "", and
// store, rotating through a new storeRotator.
func storeSource(refreshToken tokens.RefreshToken, store tokens.Store) (*tokens.Source, *storeRotator) {
	r := &storeRotator{}
	s := tokens.NewSource(r.rotate)
	s.SetRefreshToken(refreshToken)
	s.SetStore(store)

	return s, r
}

// storeGet gets a token from s, and fails the test on an error.
func storeGet(t *testing.T, s *tokens.Source) tokens.AppConfigurationToken {
	t.Helper()

	token, err := s.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	return token
}

// An empty store gets the tokens from rotating the configured refresh
// token, and remembers which refresh token began them.
func TestStoreEmpty(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, nil)
	s, r := storeSource("refresh-1", store)

	if got := storeGet(t, s); got != "xoxe-new" {
		t.Errorf("Get = %q", string(got))
	}

	want := storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-new",
		RefreshToken:          "refresh-2",
		ExpiresAt:             4102444800,
		SeedSHA256:            storeSeedOf("refresh-1"),
	}
	if got := store.record(t); got != want {
		t.Errorf("stored %+v, want %+v", got, want)
	}
	if got := r.rotations(); got != "refresh-1" {
		t.Errorf("rotated %q", got)
	}
}

// The store and the refresh token may be set in either order.
func TestStoreSetBeforeRefreshToken(t *testing.T) {
	t.Parallel()

	r := &storeRotator{}
	store := newStoreMemory(t, nil)
	s := tokens.NewSource(r.rotate)
	s.SetStore(store)
	s.SetRefreshToken("refresh-1")

	storeGet(t, s)

	if got := store.record(t); got.SeedSHA256 != storeSeedOf("refresh-1") {
		t.Errorf("stored seed %q, want the configured refresh token's", got.SeedSHA256)
	}
}

// A later process uses the stored token, without rotating.
func TestStoreValidToken(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
		SeedSHA256:            storeSeedOf("refresh-1"),
	})
	s, r := storeSource("refresh-1", store)

	for range 2 {
		if got := storeGet(t, s); got != "xoxe-stored" {
			t.Errorf("Get = %q, want the stored token", string(got))
		}
	}

	if got := r.rotations(); got != "" {
		t.Errorf("rotated %q, want none", got)
	}
	if store.saves != 0 {
		t.Errorf("saved %d times, want none", store.saves)
	}
}

// An expired stored token is replaced by rotating the stored refresh token,
// not the configured one, which Slack has voided.
func TestStoreExpiredToken(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             1700000000,
		SeedSHA256:            storeSeedOf("refresh-1"),
	})
	s, r := storeSource("refresh-1", store)

	if got := storeGet(t, s); got != "xoxe-new" {
		t.Errorf("Get = %q", string(got))
	}

	if got := r.rotations(); got != "refresh-stored" {
		t.Errorf("rotated %q, want the stored refresh token", got)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" || got.SeedSHA256 != storeSeedOf("refresh-1") {
		t.Errorf("stored %+v", got)
	}
}

// A stored token Slack refused is not used again, even though its recorded
// expiry is still ahead.
func TestStoreRefusedToken(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
	})
	s, r := storeSource("", store)

	refused := storeGet(t, s)
	if refused != "xoxe-stored" {
		t.Fatalf("Get = %q, want the stored token", string(refused))
	}
	if !s.MarkAsRefused(refused) {
		t.Fatal("MarkAsRefused = false, want a store to get another from")
	}

	if got := storeGet(t, s); got != "xoxe-new" {
		t.Errorf("Get after the refusal = %q, want a rotated token", string(got))
	}
	if got := r.rotations(); got != "refresh-stored" {
		t.Errorf("rotated %q", got)
	}
}

// A refresh token in the configuration that did not begin the stored
// tokens is newer than them: the user generated a new pair.
func TestStoreNewConfiguredRefreshToken(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
		SeedSHA256:            storeSeedOf("refresh-old"),
	})
	s, r := storeSource("refresh-1", store)

	storeGet(t, s)

	if got := r.rotations(); got != "refresh-1" {
		t.Errorf("rotated %q, want the configured refresh token", got)
	}
	if got := store.record(t); got.SeedSHA256 != storeSeedOf("refresh-1") {
		t.Errorf("stored seed %q, want the configured refresh token's", got.SeedSHA256)
	}
}

// A record that does not say which refresh token began it may be the only
// live chain, so a configured refresh token does not override it.
func TestStoreRecordWithoutSeed(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
	})
	s, _ := storeSource("refresh-1", store)

	if got := storeGet(t, s); got != "xoxe-stored" {
		t.Errorf("Get = %q, want the stored token", string(got))
	}
}

// An emptied store reads as one that holds nothing.
func TestStoreBlank(t *testing.T) {
	t.Parallel()

	store := &storeMemory{data: []byte(" \n")}
	s, _ := storeSource("refresh-1", store)

	storeGet(t, s)

	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v", got)
	}
}

// When the store fails to take a rotation's result, the source keeps it,
// and saves it before it hands out a token again.
func TestStoreSaveRetried(t *testing.T) {
	t.Parallel()

	store := &storeMemory{saveErr: errors.New("save failed")}
	s, r := storeSource("refresh-1", store)

	for range 2 {
		if token, err := s.Get(t.Context()); err == nil {
			t.Fatalf("Get = %q although the store failed", string(token))
		}
	}

	store.mu.Lock()
	store.saveErr = nil
	store.mu.Unlock()

	if got := storeGet(t, s); got != "xoxe-new" {
		t.Errorf("Get = %q", string(got))
	}
	if got := r.rotations(); got != "refresh-1" {
		t.Errorf("rotated %q, want once", got)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v", got)
	}
}

// A rotation and its save run to the end when the caller's context is
// cancelled: Slack has voided the old refresh token by then.
func TestStoreCancelled(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, nil)
	s, _ := storeSource("refresh-1", store)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := s.Get(ctx); err != nil {
		t.Fatalf("Get on a cancelled context returned %v, want the rotation to finish", err)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v, want the rotated tokens", got)
	}
}

// Sources that share one token file, as separate processes do, rotate once
// between them.
func TestStoreSharedFile(t *testing.T) {
	t.Parallel()

	r := &storeRotator{}
	path := filepath.Join(t.TempDir(), "tokens.json")

	var wg sync.WaitGroup
	for range 8 {
		s := tokens.NewSource(r.rotate)
		s.SetRefreshToken("refresh-1")
		s.SetStore(storage.NewFile(path))

		wg.Go(func() {
			if _, err := s.Get(t.Context()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	if got := r.rotations(); got != "refresh-1" {
		t.Errorf("rotated %q, want once", got)
	}
}

// With no refresh token configured, the store is the only source, and the
// seed it holds is kept.
func TestStoreOnly(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:      1,
		RefreshToken: "refresh-stored",
		SeedSHA256:   "seed",
	})
	s, r := storeSource("", store)

	storeGet(t, s)

	if got := r.rotations(); got != "refresh-stored" {
		t.Errorf("rotated %q", got)
	}
	if got := store.record(t); got.SeedSHA256 != "seed" || got.AppConfigurationToken != "xoxe-new" {
		t.Errorf("stored %+v", got)
	}
}

// A stored token with no expiry is used until Slack refuses it, and a
// rotation that reports no expiry is stored without one.
func TestStoreUnknownExpiry(t *testing.T) {
	t.Parallel()

	store := newStoreMemory(t, &storeRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
	})
	r := &storeRotator{noExpiry: true}
	s := tokens.NewSource(r.rotate)
	s.SetStore(store)

	refused := storeGet(t, s)
	if refused != "xoxe-stored" {
		t.Fatalf("Get = %q, want the stored token", string(refused))
	}
	s.MarkAsRefused(refused)
	storeGet(t, s)

	if got := store.record(t); got.AppConfigurationToken != "xoxe-new" || got.ExpiresAt != 0 {
		t.Errorf("stored %+v, want the new token with no expiry", got)
	}
}

// With an app configuration token in the configuration too, the source
// still reads the store first: once the chain began, Slack has revoked the
// configured token.
func TestStoreConfiguredToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		record  *storeRecord
		want    tokens.AppConfigurationToken
		rotated string
	}{
		"empty store": {
			want: "xoxe-configured",
		},
		"good stored token": {
			record: &storeRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             4102444800,
				SeedSHA256:            storeSeedOf("refresh-1"),
			},
			want: "xoxe-stored",
		},
		"expired stored token": {
			record: &storeRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             1700000000,
				SeedSHA256:            storeSeedOf("refresh-1"),
			},
			want:    "xoxe-new",
			rotated: "refresh-stored",
		},
		"tokens from an older pair": {
			record: &storeRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             4102444800,
				SeedSHA256:            storeSeedOf("refresh-old"),
			},
			want: "xoxe-configured",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, r := storeSource("refresh-1", newStoreMemory(t, tt.record))
			s.SetAppConfigurationToken("xoxe-configured")

			// The second call reuses what the first settled on.
			for range 2 {
				if got := storeGet(t, s); got != tt.want {
					t.Errorf("Get = %q, want %q", string(got), string(tt.want))
				}
			}
			if got := r.rotations(); got != tt.rotated {
				t.Errorf("rotated %q, want %q", got, tt.rotated)
			}
		})
	}
}

func TestStoreEmptyWithoutRefreshToken(t *testing.T) {
	t.Parallel()

	s, _ := storeSource("", newStoreMemory(t, nil))

	if _, err := s.Get(t.Context()); err == nil || !strings.Contains(err.Error(), "neither the token store nor the configuration holds a refresh token") {
		t.Fatalf("Get returned %v, want an error saying there is no refresh token", err)
	}
}

func TestStoreFailures(t *testing.T) {
	t.Parallel()

	secret := "xoxe-secret-in-store"

	tests := map[string]struct {
		store *storeMemory
		want  string
	}{
		"lock": {
			store: &storeMemory{lockErr: errors.New("lock failed")},
			want:  "locking the token store: lock failed",
		},
		"load": {
			store: &storeMemory{loadErr: errors.New("load failed")},
			want:  "reading the token store: load failed",
		},
		"save": {
			store: &storeMemory{saveErr: errors.New("save failed")},
			want:  "the next call tries to keep them again: writing the token store: save failed",
		},
		"not JSON": {
			store: &storeMemory{data: []byte(secret)},
			want:  "the token store holds something that is not a token record",
		},
		"other version": {
			store: &storeMemory{data: []byte(`{"version":2,"refresh_token":"` + secret + `"}`)},
			want:  "the token store holds a record of version 2, want 1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s, _ := storeSource("refresh-1", tt.store)

			_, err := s.Get(t.Context())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Get returned %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the error quotes the store: %v", err)
			}
		})
	}
}
