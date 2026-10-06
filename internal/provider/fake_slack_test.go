package provider_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeSlack serves the apps.manifest.* methods the provider calls, keeping
// each app's manifest in memory. It returns a manifest exactly as it was
// sent, so it stands for a Slack that never reorders or rewrites anything.
//
//declscope:shared // acceptance_test.go runs every test against one
type fakeSlack struct {
	//declscope:private
	server *httptest.Server

	//declscope:private
	mu sync.Mutex
	//declscope:private
	nextID int
	//declscope:private
	apps map[string]json.RawMessage
	//declscope:private
	calls map[string]int
}

// fakeSlackAppID is the ID the fake gives the nth app it creates, counting
// from 1, so that a test can name an app before it exists.
//
//declscope:shared // acceptance_test.go names an app before creating it
func fakeSlackAppID(n int) string {
	return fmt.Sprintf("A%09d", n)
}

//declscope:shared // acceptance_test.go starts one per test
func newFakeSlack(t *testing.T) *fakeSlack {
	t.Helper()

	f := &fakeSlack{
		apps:  map[string]json.RawMessage{},
		calls: map[string]int{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)

	return f
}

// baseURL is what the provider's base_url points at.
//
//declscope:shared // acceptance_test.go checks Slack through it
func (f *fakeSlack) baseURL() string {
	return f.server.URL + "/"
}

// manifest returns the manifest Slack holds for the app, and whether the app
// exists.
//
//declscope:shared // acceptance_test.go checks Slack through it
func (f *fakeSlack) manifest(appID string) (json.RawMessage, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, ok := f.apps[appID]

	return m, ok
}

// appCount returns how many apps exist.
//
//declscope:shared // acceptance_test.go checks Slack through it
func (f *fakeSlack) appCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.apps)
}

// callCount returns how many times a method was called.
//
//declscope:shared // acceptance_test.go checks Slack through it
func (f *fakeSlack) callCount(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls[method]
}

func (f *fakeSlack) serveHTTP(w http.ResponseWriter, r *http.Request) {
	method := strings.TrimPrefix(r.URL.Path, "/")

	if r.Header.Get("Authorization") == "" {
		f.reply(w, map[string]any{"ok": false, "error": "not_authed"})

		return
	}

	var body struct {
		AppID    string `json:"app_id"`
		Manifest string `json:"manifest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		f.reply(w, map[string]any{"ok": false, "error": "invalid_json"})

		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls[method]++

	switch method {
	case "apps.manifest.create":
		f.nextID++
		appID := fakeSlackAppID(f.nextID)
		f.apps[appID] = json.RawMessage(body.Manifest)
		f.reply(w, map[string]any{
			"ok":     true,
			"app_id": appID,
			"credentials": map[string]string{
				"client_id":          "client-" + appID,
				"client_secret":      "secret-" + appID,
				"verification_token": "verification-" + appID,
				"signing_secret":     "signing-" + appID,
			},
			"oauth_authorize_url": "https://slack.com/oauth/v2/authorize?client_id=client-" + appID,
		})
	case "apps.manifest.update":
		if _, ok := f.apps[body.AppID]; !ok {
			f.reply(w, map[string]any{"ok": false, "error": "app_not_found"})

			return
		}
		f.apps[body.AppID] = json.RawMessage(body.Manifest)
		f.reply(w, map[string]any{"ok": true, "app_id": body.AppID, "permissions_updated": false})
	case "apps.manifest.export":
		m, ok := f.apps[body.AppID]
		if !ok {
			f.reply(w, map[string]any{"ok": false, "error": "app_not_found"})

			return
		}
		f.reply(w, map[string]any{"ok": true, "manifest": m})
	case "apps.manifest.delete":
		if _, ok := f.apps[body.AppID]; !ok {
			f.reply(w, map[string]any{"ok": false, "error": "app_not_found"})

			return
		}
		delete(f.apps, body.AppID)
		f.reply(w, map[string]any{"ok": true})
	default:
		f.reply(w, map[string]any{"ok": false, "error": "unknown_method"})
	}
}

func (f *fakeSlack) reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
