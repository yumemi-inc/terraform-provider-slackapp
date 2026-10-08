package slack_test

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

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/tokenstore"
)

// tokenStoreMemory is a slack.TokenStore that keeps the record in memory,
// and fails where a test tells it to.
type tokenStoreMemory struct {
	mu      sync.Mutex
	data    []byte
	saves   int
	lockErr error
	loadErr error
	saveErr error
}

func (m *tokenStoreMemory) Lock(context.Context) (func(), error) {
	if m.lockErr != nil {
		return nil, m.lockErr
	}

	return func() {}, nil
}

func (m *tokenStoreMemory) Load(context.Context) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.data, m.loadErr
}

func (m *tokenStoreMemory) Save(_ context.Context, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.saveErr != nil {
		return m.saveErr
	}
	m.data = data
	m.saves++

	return nil
}

// tokenStoreRecord is the record as the client writes it.
type tokenStoreRecord struct {
	Version               int    `json:"version"`
	AppConfigurationToken string `json:"app_configuration_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresAt             int64  `json:"expires_at"`
	SeedSHA256            string `json:"seed_sha256"`
}

func newTokenStoreMemory(t *testing.T, record *tokenStoreRecord) *tokenStoreMemory {
	t.Helper()

	m := &tokenStoreMemory{}
	if record != nil {
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		m.data = data
	}

	return m
}

func (m *tokenStoreMemory) record(t *testing.T) tokenStoreRecord {
	t.Helper()

	m.mu.Lock()
	defer m.mu.Unlock()

	var record tokenStoreRecord
	if err := json.Unmarshal(m.data, &record); err != nil {
		t.Fatalf("the store holds %q: %v", m.data, err)
	}

	return record
}

func tokenStoreSeedOf(refreshToken string) string {
	sum := sha256.Sum256([]byte(refreshToken))

	return hex.EncodeToString(sum[:])
}

// tokenStoreServer rotates every refresh token for xoxe-new and refresh-2.
func tokenStoreServer(t *testing.T) *clientServer {
	t.Helper()

	return newClientServer(t, map[string]string{
		"tooling.tokens.rotate": `{"ok":true,"token":"xoxe-new","refresh_token":"refresh-2","iat":1700000000,"exp":4102444800}`,
		"apps.manifest.export":  `{"ok":true,"manifest":{}}`,
	})
}

func tokenStoreExport(t *testing.T, c *slack.Client) error {
	t.Helper()

	_, err := c.AppsManifestExport(t.Context(), slack.AppsManifestExportRequest{AppID: "A1"})

	return err
}

// An empty store gets the tokens from rotating the configured refresh
// token, and remembers which refresh token began them.
func TestTokenStoreEmpty(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, nil)
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	want := tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-new",
		RefreshToken:          "refresh-2",
		ExpiresAt:             4102444800,
		SeedSHA256:            tokenStoreSeedOf("refresh-1"),
	}
	if got := store.record(t); got != want {
		t.Errorf("stored %+v, want %+v", got, want)
	}
	if got := s.rotated(); strings.Join(got, ",") != "refresh-1" {
		t.Errorf("rotated %q", got)
	}
}

// The store and the refresh token may be set in either order.
func TestTokenStoreSetBeforeRefreshToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, nil)
	c := slack.NewClient().WithTokenStore(store).WithRefreshToken("refresh-1").WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := store.record(t); got.SeedSHA256 != tokenStoreSeedOf("refresh-1") {
		t.Errorf("stored seed %q, want the configured refresh token's", got.SeedSHA256)
	}
}

// A later process uses the stored token, without rotating.
func TestTokenStoreValidToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
		SeedSHA256:            tokenStoreSeedOf("refresh-1"),
	})
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(s.methods(), ","); got != "apps.manifest.export(xoxe-stored)" {
		t.Errorf("calls = %s", got)
	}
	if store.saves != 0 {
		t.Errorf("saved %d times, want none", store.saves)
	}
}

// An expired stored token is replaced by rotating the stored refresh token,
// not the configured one, which Slack has voided.
func TestTokenStoreExpiredToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             1700000000,
		SeedSHA256:            tokenStoreSeedOf("refresh-1"),
	})
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := s.rotated(); strings.Join(got, ",") != "refresh-stored" {
		t.Errorf("rotated %q, want the stored refresh token", got)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" || got.SeedSHA256 != tokenStoreSeedOf("refresh-1") {
		t.Errorf("stored %+v", got)
	}
}

// A stored token Slack says expired is not used again, even though its
// recorded expiry is still ahead.
func TestTokenStoreRejectedToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	s.expire("xoxe-stored")
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
	})
	c := slack.NewClient().WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	want := "apps.manifest.export(xoxe-stored),tooling.tokens.rotate,apps.manifest.export(xoxe-new)"
	if got := strings.Join(s.methods(), ","); got != want {
		t.Errorf("calls = %s, want %s", got, want)
	}
}

// A refresh token in the configuration that did not begin the stored
// tokens is newer than them: the user generated a new pair.
func TestTokenStoreNewConfiguredRefreshToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
		SeedSHA256:            tokenStoreSeedOf("refresh-old"),
	})
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := s.rotated(); strings.Join(got, ",") != "refresh-1" {
		t.Errorf("rotated %q, want the configured refresh token", got)
	}
	if got := store.record(t); got.SeedSHA256 != tokenStoreSeedOf("refresh-1") {
		t.Errorf("stored seed %q, want the configured refresh token's", got.SeedSHA256)
	}
}

// A record that does not say which refresh token began it may be the only
// live chain, so a configured refresh token does not override it.
func TestTokenStoreRecordWithoutSeed(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
		ExpiresAt:             4102444800,
	})
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(s.methods(), ","); got != "apps.manifest.export(xoxe-stored)" {
		t.Errorf("calls = %s", got)
	}
}

// An emptied store reads as one that holds nothing.
func TestTokenStoreBlank(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := &tokenStoreMemory{data: []byte(" \n")}
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v", got)
	}
}

// When the store fails to take a rotation's result, the client keeps it and
// saves it on its next call, before it calls Slack.
func TestTokenStoreSaveRetried(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := &tokenStoreMemory{saveErr: errors.New("save failed")}
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	for range 2 {
		if err := tokenStoreExport(t, c); err == nil {
			t.Fatal("export succeeded although the store failed")
		}
	}

	store.mu.Lock()
	store.saveErr = nil
	store.mu.Unlock()

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	// One rotation, and no call while the tokens were not kept.
	if got := strings.Join(s.methods(), ","); got != "tooling.tokens.rotate,apps.manifest.export(xoxe-new)" {
		t.Errorf("calls = %s", got)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v", got)
	}
}

// A rotation and its save run to the end when the caller's context is
// cancelled: Slack has voided the old refresh token by then.
func TestTokenStoreCancelled(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, nil)
	c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(store).WithBaseURL(s.URL + "/")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := c.AppsManifestExport(ctx, slack.AppsManifestExportRequest{AppID: "A1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("export returned %v, want the cancellation", err)
	}
	if got := store.record(t); got.RefreshToken != "refresh-2" {
		t.Errorf("stored %+v, want the rotated tokens", got)
	}
}

// Clients that share one token file, as separate processes do, rotate once
// between them.
func TestTokenStoreSharedFile(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	path := filepath.Join(t.TempDir(), "tokens.json")

	var wg sync.WaitGroup
	for range 8 {
		c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(tokenstore.NewFile(path)).WithBaseURL(s.URL + "/")
		wg.Go(func() {
			if err := tokenStoreExport(t, c); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	if got := s.rotated(); len(got) != 1 {
		t.Errorf("rotated %q, want once", got)
	}
}

// With no refresh token configured, the store is the only source, and the
// seed it holds is kept.
func TestTokenStoreOnly(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:      1,
		RefreshToken: "refresh-stored",
		SeedSHA256:   "seed",
	})
	c := slack.NewClient().WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	if got := s.rotated(); strings.Join(got, ",") != "refresh-stored" {
		t.Errorf("rotated %q", got)
	}
	if got := store.record(t); got.SeedSHA256 != "seed" || got.AppConfigurationToken != "xoxe-new" {
		t.Errorf("stored %+v", got)
	}
}

// A stored token with no expiry is used until Slack refuses it, and a
// rotation that reports no expiry is stored without one.
func TestTokenStoreUnknownExpiry(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"tooling.tokens.rotate": `{"ok":true,"token":"xoxe-new","refresh_token":"refresh-2"}`,
		"apps.manifest.export":  `{"ok":true,"manifest":{}}`,
	})
	s.expire("xoxe-stored")
	store := newTokenStoreMemory(t, &tokenStoreRecord{
		Version:               1,
		AppConfigurationToken: "xoxe-stored",
		RefreshToken:          "refresh-stored",
	})
	c := slack.NewClient().WithTokenStore(store).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err != nil {
		t.Fatal(err)
	}

	want := "apps.manifest.export(xoxe-stored),tooling.tokens.rotate,apps.manifest.export(xoxe-new)"
	if got := strings.Join(s.methods(), ","); got != want {
		t.Errorf("calls = %s, want %s", got, want)
	}
	if got := store.record(t); got.AppConfigurationToken != "xoxe-new" || got.ExpiresAt != 0 {
		t.Errorf("stored %+v, want the new token with no expiry", got)
	}
}

// With an app configuration token in the configuration too, the client
// still reads the store first: once the chain began, Slack has revoked the
// configured token.
func TestTokenStoreConfiguredToken(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		record *tokenStoreRecord
		want   string
	}{
		"empty store": {
			want: "apps.manifest.export(xoxe-configured)",
		},
		"good stored token": {
			record: &tokenStoreRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             4102444800,
				SeedSHA256:            tokenStoreSeedOf("refresh-1"),
			},
			want: "apps.manifest.export(xoxe-stored)",
		},
		"expired stored token": {
			record: &tokenStoreRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             1700000000,
				SeedSHA256:            tokenStoreSeedOf("refresh-1"),
			},
			want: "tooling.tokens.rotate,apps.manifest.export(xoxe-new)",
		},
		"tokens from an older pair": {
			record: &tokenStoreRecord{
				Version:               1,
				AppConfigurationToken: "xoxe-stored",
				RefreshToken:          "refresh-stored",
				ExpiresAt:             4102444800,
				SeedSHA256:            tokenStoreSeedOf("refresh-old"),
			},
			want: "apps.manifest.export(xoxe-configured)",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := tokenStoreServer(t)
			c := slack.NewClient().WithAppConfigurationToken("xoxe-configured").
				WithRefreshToken("refresh-1").
				WithTokenStore(newTokenStoreMemory(t, tt.record)).
				WithBaseURL(s.URL + "/")

			for range 2 {
				if err := tokenStoreExport(t, c); err != nil {
					t.Fatal(err)
				}
			}

			// The second call reuses what the first settled on.
			want := tt.want + "," + tt.want[strings.LastIndex(tt.want, ",")+1:]
			if got := strings.Join(s.methods(), ","); got != want {
				t.Errorf("calls = %s, want %s", got, want)
			}
		})
	}
}

func TestTokenStoreEmptyWithoutRefreshToken(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	c := slack.NewClient().WithTokenStore(newTokenStoreMemory(t, nil)).WithBaseURL(s.URL + "/")

	if err := tokenStoreExport(t, c); err == nil || !strings.Contains(err.Error(), "neither the token store nor the configuration holds a refresh token") {
		t.Fatalf("export returned %v, want an error saying there is no refresh token", err)
	}
}

// When Slack refuses the token and no other can be had, the error says
// both.
func TestTokenStoreNothingAfterRefusal(t *testing.T) {
	t.Parallel()

	s := tokenStoreServer(t)
	s.refuse("xoxe-configured", "token_revoked")
	c := slack.NewClient().WithAppConfigurationToken("xoxe-configured").WithTokenStore(newTokenStoreMemory(t, nil)).WithBaseURL(s.URL + "/")

	err := tokenStoreExport(t, c)

	var slackErr *slack.ErrorResponse
	if !errors.As(err, &slackErr) || slackErr.Error() != "token_revoked" {
		t.Fatalf("export returned %v, want token_revoked", err)
	}
	if !strings.Contains(err.Error(), "holds a refresh token") {
		t.Errorf("error = %q, want why no other token could be had", err)
	}
}

func TestTokenStoreFailures(t *testing.T) {
	t.Parallel()

	secret := "xoxe-secret-in-store"

	tests := map[string]struct {
		store *tokenStoreMemory
		want  string
	}{
		"lock": {
			store: &tokenStoreMemory{lockErr: errors.New("lock failed")},
			want:  "locking the token store: lock failed",
		},
		"load": {
			store: &tokenStoreMemory{loadErr: errors.New("load failed")},
			want:  "reading the token store: load failed",
		},
		"save": {
			store: &tokenStoreMemory{saveErr: errors.New("save failed")},
			want:  "the next call tries to keep them again: writing the token store: save failed",
		},
		"not JSON": {
			store: &tokenStoreMemory{data: []byte(secret)},
			want:  "the token store holds something that is not a token record",
		},
		"other version": {
			store: &tokenStoreMemory{data: []byte(`{"version":2,"refresh_token":"` + secret + `"}`)},
			want:  "the token store holds a record of version 2, want 1",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := tokenStoreServer(t)
			c := slack.NewClient().WithRefreshToken("refresh-1").WithTokenStore(tt.store).WithBaseURL(s.URL + "/")

			err := tokenStoreExport(t, c)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("export returned %v, want %q", err, tt.want)
			}
			if strings.Contains(err.Error(), secret) {
				t.Errorf("the error quotes the store: %v", err)
			}
		})
	}
}
