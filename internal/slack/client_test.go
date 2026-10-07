package slack_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

// clientCall is one request the Slack API received.
type clientCall struct {
	method        string
	authorization string
	userAgent     string
	contentType   string
	body          string
}

// clientServer is a Slack API that answers each method with a fixed reply
// and records what it was sent.
type clientServer struct {
	*httptest.Server

	mu      sync.Mutex
	replies map[string]string
	calls   []clientCall
}

func newClientServer(t *testing.T, replies map[string]string) *clientServer {
	t.Helper()

	s := &clientServer{replies: replies}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		method := strings.TrimPrefix(r.URL.Path, "/")

		s.mu.Lock()
		s.calls = append(s.calls, clientCall{
			method:        method,
			authorization: r.Header.Get("Authorization"),
			userAgent:     r.Header.Get("User-Agent"),
			contentType:   r.Header.Get("Content-Type"),
			body:          string(body),
		})
		reply, ok := s.replies[method]
		s.mu.Unlock()

		if !ok {
			reply = `{"ok":false,"error":"unknown_method"}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reply)
	}))
	t.Cleanup(s.Close)

	return s
}

func (s *clientServer) client(token string) *slack.Client {
	return slack.NewClient(token).WithBaseURL(s.URL + "/")
}

func (s *clientServer) recorded() []clientCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]clientCall(nil), s.calls...)
}

// clientJSONEqual reports whether two JSON documents are equal, ignoring
// formatting.
func clientJSONEqual(t *testing.T, got, want string) bool {
	t.Helper()

	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("not JSON: %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("not JSON: %q: %v", want, err)
	}

	gb, _ := json.Marshal(g)
	wb, _ := json.Marshal(w)

	return string(gb) == string(wb)
}

func TestClientRequest(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"apps.manifest.create": `{"ok":true,"app_id":"A1"}`,
	})

	if _, err := s.client("xoxe-token").AppsManifestCreate(t.Context(), slack.AppsManifestCreateRequest{Manifest: `{"a":1}`}); err != nil {
		t.Fatal(err)
	}

	calls := s.recorded()
	if len(calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(calls))
	}

	got := calls[0]
	if got.method != "apps.manifest.create" {
		t.Errorf("method = %q", got.method)
	}
	if got.authorization != "Bearer xoxe-token" {
		t.Errorf("Authorization = %q", got.authorization)
	}
	if got.userAgent != "ymm-oss/terraform-provider-slackapp" {
		t.Errorf("User-Agent = %q", got.userAgent)
	}
	if got.contentType != "application/json" {
		t.Errorf("Content-Type = %q", got.contentType)
	}
	if !clientJSONEqual(t, got.body, `{"manifest":"{\"a\":1}"}`) {
		t.Errorf("body = %s", got.body)
	}
}

func TestClientMethods(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"apps.manifest.create": `{"ok":true,"app_id":"A1","credentials":{"client_id":"cid","client_secret":"cs","verification_token":"vt","signing_secret":"ss"},"oauth_authorize_url":"https://slack.com/oauth"}`,
		"apps.manifest.update": `{"ok":true,"app_id":"A1","permissions_updated":true}`,
		"apps.manifest.export": `{"ok":true,"manifest":{"display_information":{"name":"A"},"outgoing_domains":["x"]}}`,
		"apps.manifest.delete": `{"ok":true}`,
	})
	c := s.client("token")
	ctx := t.Context()

	created, err := c.AppsManifestCreate(ctx, slack.AppsManifestCreateRequest{Manifest: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if created.AppID != "A1" || created.OauthAuthorizeURL != "https://slack.com/oauth" {
		t.Errorf("create returned %+v", created)
	}
	if created.Credentials.ClientID != "cid" || created.Credentials.ClientSecret != "cs" ||
		created.Credentials.VerificationToken != "vt" || created.Credentials.SigningSecret != "ss" {
		t.Errorf("create returned credentials %+v", created.Credentials)
	}

	updated, err := c.AppsManifestUpdate(ctx, slack.AppsManifestUpdateRequest{AppID: "A1", Manifest: `{}`})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AppID != "A1" || !updated.PermissionsUpdated {
		t.Errorf("update returned %+v", updated)
	}

	exported, err := c.AppsManifestExport(ctx, slack.AppsManifestExportRequest{AppID: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	// The manifest is kept as Slack sent it, fields the provider does not
	// model included.
	if !clientJSONEqual(t, string(exported.Manifest), `{"display_information":{"name":"A"},"outgoing_domains":["x"]}`) {
		t.Errorf("export returned manifest %s", exported.Manifest)
	}

	if _, err := c.AppsManifestDelete(ctx, slack.AppsManifestDeleteRequest{AppID: "A1"}); err != nil {
		t.Fatal(err)
	}

	calls := s.recorded()
	wantBodies := []string{
		`{"manifest":"{}"}`,
		`{"app_id":"A1","manifest":"{}"}`,
		`{"app_id":"A1"}`,
		`{"app_id":"A1"}`,
	}
	if len(calls) != len(wantBodies) {
		t.Fatalf("got %d calls, want %d", len(calls), len(wantBodies))
	}
	for i, want := range wantBodies {
		if !clientJSONEqual(t, calls[i].body, want) {
			t.Errorf("%s sent %s, want %s", calls[i].method, calls[i].body, want)
		}
	}
}

func TestClientErrorResponse(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"apps.manifest.update": `{"ok":false,"error":"invalid_manifest","errors":[{"message":"PKCE cannot be disabled once enabled","pointer":"/oauth_config/pkce_enabled"}]}`,
	})

	_, err := s.client("token").AppsManifestUpdate(t.Context(), slack.AppsManifestUpdateRequest{AppID: "A1", Manifest: `{}`})
	if err == nil {
		t.Fatal("update succeeded, want an error")
	}

	var slackErr *slack.ErrorResponse
	if !errors.As(err, &slackErr) {
		t.Fatalf("error is %T, want *slack.ErrorResponse", err)
	}
	if slackErr.Error() != "invalid_manifest" {
		t.Errorf("Error() = %q", slackErr.Error())
	}
	if slackErr.IsOk() {
		t.Error("IsOk() = true")
	}
	if len(slackErr.Errors) != 1 || slackErr.Errors[0].Pointer != "/oauth_config/pkce_enabled" {
		t.Errorf("Errors = %+v", slackErr.Errors)
	}
}

func TestClientEveryMethodReportsErrors(t *testing.T) {
	t.Parallel()

	// Every method answers unknown_method from a server with no replies.
	c := newClientServer(t, nil).client("token")
	ctx := t.Context()

	calls := map[string]func() error{
		"apps.manifest.create": func() error {
			_, err := c.AppsManifestCreate(ctx, slack.AppsManifestCreateRequest{})
			return err
		},
		"apps.manifest.update": func() error {
			_, err := c.AppsManifestUpdate(ctx, slack.AppsManifestUpdateRequest{})
			return err
		},
		"apps.manifest.export": func() error {
			_, err := c.AppsManifestExport(ctx, slack.AppsManifestExportRequest{})
			return err
		},
		"apps.manifest.delete": func() error {
			_, err := c.AppsManifestDelete(ctx, slack.AppsManifestDeleteRequest{})
			return err
		},
		"tooling.tokens.rotate": func() error {
			_, err := c.ToolingTokensRotate(ctx, "refresh")
			return err
		},
	}

	for method, call := range calls {
		err := call()

		var slackErr *slack.ErrorResponse
		if !errors.As(err, &slackErr) || slackErr.Error() != "unknown_method" {
			t.Errorf("%s returned %v, want unknown_method", method, err)
		}
	}
}

func TestClientRefreshToken(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"tooling.tokens.rotate": `{"ok":true,"token":"xoxe-new","refresh_token":"refresh-2","iat":1700000000,"exp":1700043200}`,
		"apps.manifest.export":  `{"ok":true,"manifest":{}}`,
	})
	c := slack.NewClientFromRefreshToken("refresh-1").WithBaseURL(s.URL + "/")

	for range 2 {
		if _, err := c.AppsManifestExport(t.Context(), slack.AppsManifestExportRequest{AppID: "A1"}); err != nil {
			t.Fatal(err)
		}
	}

	calls := s.recorded()
	methods := make([]string, len(calls))
	for i, call := range calls {
		methods[i] = call.method
	}
	// The token is rotated once, before the first call that needs it.
	if strings.Join(methods, ",") != "tooling.tokens.rotate,apps.manifest.export,apps.manifest.export" {
		t.Fatalf("calls = %v", methods)
	}

	rotate := calls[0]
	if rotate.authorization != "" {
		t.Errorf("rotate sent Authorization %q, want none", rotate.authorization)
	}
	if rotate.contentType != "application/x-www-form-urlencoded" {
		t.Errorf("rotate Content-Type = %q", rotate.contentType)
	}
	if form, err := url.ParseQuery(rotate.body); err != nil || form.Get("refresh_token") != "refresh-1" {
		t.Errorf("rotate body = %q", rotate.body)
	}

	for _, call := range calls[1:] {
		if call.authorization != "Bearer xoxe-new" {
			t.Errorf("export sent Authorization %q, want the rotated token", call.authorization)
		}
	}
}

func TestClientRefreshTokenFailure(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"tooling.tokens.rotate": `{"ok":false,"error":"invalid_refresh_token"}`,
		"apps.manifest.create":  `{"ok":true,"app_id":"A1"}`,
		"apps.manifest.update":  `{"ok":true,"app_id":"A1"}`,
		"apps.manifest.export":  `{"ok":true,"manifest":{}}`,
		"apps.manifest.delete":  `{"ok":true}`,
	})
	newClient := func() *slack.Client {
		return slack.NewClientFromRefreshToken("refresh-1").WithBaseURL(s.URL + "/")
	}
	ctx := t.Context()

	calls := map[string]func() error{
		"create": func() error {
			_, err := newClient().AppsManifestCreate(ctx, slack.AppsManifestCreateRequest{})
			return err
		},
		"update": func() error {
			_, err := newClient().AppsManifestUpdate(ctx, slack.AppsManifestUpdateRequest{})
			return err
		},
		"export": func() error {
			_, err := newClient().AppsManifestExport(ctx, slack.AppsManifestExportRequest{})
			return err
		},
		"delete": func() error {
			_, err := newClient().AppsManifestDelete(ctx, slack.AppsManifestDeleteRequest{})
			return err
		},
	}

	for name, call := range calls {
		err := call()

		var slackErr *slack.ErrorResponse
		if !errors.As(err, &slackErr) || slackErr.Error() != "invalid_refresh_token" {
			t.Errorf("%s returned %v, want invalid_refresh_token", name, err)
		}
	}

	// Only the rotations reached Slack: no method ran without a token.
	for _, call := range s.recorded() {
		if call.method != "tooling.tokens.rotate" {
			t.Errorf("%s was called although the token could not be rotated", call.method)
		}
	}
}

func TestToolingTokensRotateTimestamps(t *testing.T) {
	t.Parallel()

	s := newClientServer(t, map[string]string{
		"tooling.tokens.rotate": `{"ok":true,"token":"t","refresh_token":"r","iat":1700000000,"exp":1700043200}`,
	})

	got, err := s.client("").ToolingTokensRotate(t.Context(), "refresh")
	if err != nil {
		t.Fatal(err)
	}

	if !got.IssuedAt.Time().Equal(time.Unix(1700000000, 0)) {
		t.Errorf("IssuedAt = %v", got.IssuedAt.Time())
	}
	if !got.ExpiresAt.Time().Equal(time.Unix(1700043200, 0)) {
		t.Errorf("ExpiresAt = %v", got.ExpiresAt.Time())
	}
}

func TestUnixTimestampInvalid(t *testing.T) {
	t.Parallel()

	var ts slack.UnixTimestamp
	if err := json.Unmarshal([]byte(`"yesterday"`), &ts); err == nil {
		t.Error("UnmarshalJSON accepted a string")
	}
}

func TestClientInvalidResponse(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<html>not JSON</html>")
	}))
	t.Cleanup(s.Close)

	_, err := slack.NewClient("token").WithBaseURL(s.URL+"/").AppsManifestDelete(t.Context(), slack.AppsManifestDeleteRequest{AppID: "A1"})
	if err == nil {
		t.Fatal("delete succeeded on a reply that is not JSON")
	}
}

func TestClientUnreachable(t *testing.T) {
	t.Parallel()

	s := httptest.NewServer(http.NotFoundHandler())
	base := s.URL + "/"
	s.Close()

	c := slack.NewClient("token").WithBaseURL(base)
	ctx := context.Background()

	if _, err := c.AppsManifestCreate(ctx, slack.AppsManifestCreateRequest{}); err == nil {
		t.Error("create succeeded against a closed server")
	}
	if _, err := c.AppsManifestUpdate(ctx, slack.AppsManifestUpdateRequest{}); err == nil {
		t.Error("update succeeded against a closed server")
	}
	if _, err := c.AppsManifestExport(ctx, slack.AppsManifestExportRequest{}); err == nil {
		t.Error("export succeeded against a closed server")
	}
	if _, err := c.AppsManifestDelete(ctx, slack.AppsManifestDeleteRequest{}); err == nil {
		t.Error("delete succeeded against a closed server")
	}
	if _, err := c.ToolingTokensRotate(ctx, "refresh"); err == nil {
		t.Error("rotate succeeded against a closed server")
	}
}

func TestClientInvalidBaseURL(t *testing.T) {
	t.Parallel()

	// A control character makes the URL unparseable, so building the
	// request fails before anything is sent.
	c := slack.NewClient("token").WithBaseURL("http://\x7f/")
	ctx := t.Context()

	if _, err := c.AppsManifestCreate(ctx, slack.AppsManifestCreateRequest{}); err == nil {
		t.Error("create succeeded with an invalid base URL")
	}
	if _, err := c.AppsManifestUpdate(ctx, slack.AppsManifestUpdateRequest{}); err == nil {
		t.Error("update succeeded with an invalid base URL")
	}
	if _, err := c.AppsManifestExport(ctx, slack.AppsManifestExportRequest{}); err == nil {
		t.Error("export succeeded with an invalid base URL")
	}
	if _, err := c.AppsManifestDelete(ctx, slack.AppsManifestDeleteRequest{}); err == nil {
		t.Error("delete succeeded with an invalid base URL")
	}
	if _, err := c.ToolingTokensRotate(ctx, "refresh"); err == nil {
		t.Error("rotate succeeded with an invalid base URL")
	}
}
