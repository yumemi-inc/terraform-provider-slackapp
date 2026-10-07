package provider_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeSlack serves the apps.manifest.* methods the provider calls, keeping
// each app's manifest in memory. By default it returns a manifest exactly as
// it was sent. One from newRewritingFakeSlack rewrites it on export the way
// Slack does, without changing what it means.
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
	//declscope:private
	rewrite bool
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

// newRewritingFakeSlack returns a fake whose apps.manifest.export rewrites
// the manifest the way Slack was seen to (2026-10, against a test
// workspace):
//
//   - _metadata is dropped.
//   - Settings the manifest did not state come back with their defaults,
//     both ones the slackapp_manifest data source models (bot_user.always_online,
//     settings.interactivity, ...) and ones it does not
//     (oauth_config.pkce_enabled, settings.is_mcp_enabled, ...).
//   - Object keys come back in another order.
//
// Arrays keep the order they were sent in, as Slack's do.
//
//declscope:shared // acceptance_test.go checks that this causes no drift
func newRewritingFakeSlack(t *testing.T) *fakeSlack {
	t.Helper()

	f := newFakeSlack(t)
	f.rewrite = true

	return f
}

// fakeSlackDefaults are the defaults Slack fills in on export, by the
// object they go in. An object that is absent stays absent.
var fakeSlackDefaults = []struct {
	path     []string
	key      string
	defaults any
}{
	{[]string{"features", "bot_user"}, "always_online", true},
	{[]string{"oauth_config"}, "pkce_enabled", false},
	{[]string{"settings"}, "interactivity", map[string]any{"is_enabled": true}},
	{[]string{"settings"}, "org_deploy_enabled", false},
	{[]string{"settings"}, "token_rotation_enabled", false},
	{[]string{"settings"}, "app_level_token_rotation_enabled", false},
	{[]string{"settings"}, "is_mcp_enabled", false},
}

// fakeSlackRewrite rewrites a manifest as newRewritingFakeSlack describes.
func fakeSlackRewrite(raw json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var m map[string]any
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}

	delete(m, "_metadata")

	for _, d := range fakeSlackDefaults {
		parent := m
		for _, key := range d.path {
			next, ok := parent[key].(map[string]any)
			if !ok {
				parent = nil

				break
			}
			parent = next
		}
		if parent == nil {
			continue
		}
		if _, ok := parent[d.key]; !ok {
			parent[d.key] = d.defaults
		}
	}

	// encoding/json writes map keys sorted, which is not the order the
	// slackapp_manifest data source writes them in.
	return json.Marshal(m)
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

// editManifest changes the manifest Slack holds for the app, as someone
// editing it in Slack's own settings pages would.
//
//declscope:shared // acceptance_test.go checks that the change shows as drift
func (f *fakeSlack) editManifest(t *testing.T, appID string, edit func(map[string]any)) {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	var m map[string]any
	if err := json.Unmarshal(f.apps[appID], &m); err != nil {
		t.Fatal(err)
	}

	edit(m)

	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	f.apps[appID] = raw
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
		if f.rewrite {
			rewritten, err := fakeSlackRewrite(m)
			if err != nil {
				f.reply(w, map[string]any{"ok": false, "error": "invalid_manifest"})

				return
			}
			m = rewritten
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

// fakeSlackExport creates an app holding manifest, and returns what
// apps.manifest.export gives back for it.
func fakeSlackExport(t *testing.T, f *fakeSlack, manifest string) string {
	t.Helper()

	call := func(method string, body map[string]string) map[string]json.RawMessage {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}

		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.baseURL()+method, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer test")
		request.Header.Set("Content-Type", "application/json")

		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		var decoded map[string]json.RawMessage
		if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
			t.Fatal(err)
		}
		if string(decoded["ok"]) != "true" {
			t.Fatalf("%s failed: %s", method, decoded["error"])
		}

		return decoded
	}

	var appID string
	if err := json.Unmarshal(call("apps.manifest.create", map[string]string{"manifest": manifest})["app_id"], &appID); err != nil {
		t.Fatal(err)
	}

	return string(call("apps.manifest.export", map[string]string{"app_id": appID})["manifest"])
}

func TestFakeSlackExport(t *testing.T) {
	t.Parallel()

	// Keys out of alphabetical order, _metadata, an array, and settings
	// without the defaults Slack fills in.
	const manifest = `{"settings":{"socket_mode_enabled":true},"_metadata":{"major_version":1},"display_information":{"name":"A"},"oauth_config":{"scopes":{"bot":["b","a"]}}}`

	cases := map[string]struct {
		newFake func(*testing.T) *fakeSlack
		want    string
	}{
		"as sent": {
			newFake: newFakeSlack,
			want:    manifest,
		},
		"rewritten": {
			// _metadata dropped, defaults filled in, keys sorted, the array
			// kept in order.
			newFake: newRewritingFakeSlack,
			want:    `{"display_information":{"name":"A"},"oauth_config":{"pkce_enabled":false,"scopes":{"bot":["b","a"]}},"settings":{"app_level_token_rotation_enabled":false,"interactivity":{"is_enabled":true},"is_mcp_enabled":false,"org_deploy_enabled":false,"socket_mode_enabled":true,"token_rotation_enabled":false}}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := fakeSlackExport(t, tc.newFake(t), manifest); got != tc.want {
				t.Errorf("apps.manifest.export returned\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}
