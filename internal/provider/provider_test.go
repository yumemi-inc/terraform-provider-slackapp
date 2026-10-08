package provider_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider"
)

// providerFactories serves the provider from the test process, so that
// Terraform drives it and coverage counts what it runs.
//
//declscope:shared // every acceptance test file starts Terraform with it
var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"slackapp": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// providerUnsetEnvironment clears the variables the provider falls back to,
// so that a developer's shell does not change what a test sees.
func providerUnsetEnvironment(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"SLACK_APP_CONFIGURATION_TOKEN", "SLACK_REFRESH_TOKEN", "SLACK_BASE_URL",
		"SLACK_TOKEN_STORE_FILE", "SLACK_TOKEN_STORE_COMMAND",
	} {
		t.Setenv(name, "")
	}
}

// providerResource is one app, in a provider configured with attributes.
func providerResource(attributes string) string {
	return fmt.Sprintf(`
provider "slackapp" {
%s
}

resource "slackapp_application" "test" {
  manifest = jsonencode({ display_information = { name = "Example" } })
}
`, attributes)
}

// providerCheckAuthorization checks the token the provider sent to create
// the app.
func providerCheckAuthorization(f *fakeSlack, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := f.authorization("apps.manifest.create"); got != want {
			return fmt.Errorf("apps.manifest.create sent Authorization %q, want %q", got, want)
		}

		return nil
	}
}

func TestAccProvider_noToken(t *testing.T) {
	providerUnsetEnvironment(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerResource(""),
				ExpectError: regexp.MustCompile(`either app configuration token, refresh token or token\s+store must be provided`),
			},
		},
	})
}

// With no attributes, the provider reads the token and the base URL from
// the environment.
func TestAccProvider_environment(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	t.Setenv("SLACK_APP_CONFIGURATION_TOKEN", "xoxe-from-env")
	t.Setenv("SLACK_BASE_URL", f.baseURL())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: providerResource(""),
				Check:  providerCheckAuthorization(f, "Bearer xoxe-from-env"),
			},
		},
	})
}

// An attribute wins over the environment variable it falls back to.
func TestAccProvider_attributesOverEnvironment(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	t.Setenv("SLACK_APP_CONFIGURATION_TOKEN", "xoxe-from-env")
	t.Setenv("SLACK_BASE_URL", "http://127.0.0.1:1/")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: providerResource(fmt.Sprintf(`
  base_url                = %q
  app_configuration_token = "xoxe-from-attribute"
`, f.baseURL())),
				Check: providerCheckAuthorization(f, "Bearer xoxe-from-attribute"),
			},
		},
	})
}

// providerCheckRotations checks the refresh tokens the provider rotated,
// in order.
func providerCheckRotations(f *fakeSlack, want ...string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := f.rotatedRefreshTokens(); strings.Join(got, ",") != strings.Join(want, ",") {
			return fmt.Errorf("tooling.tokens.rotate was called with %q, want %q", got, want)
		}
		if got := f.authorization("apps.manifest.create"); !strings.HasPrefix(got, "Bearer xoxe.xoxp-rotated-") {
			return fmt.Errorf("apps.manifest.create sent Authorization %q, want a rotated token", got)
		}

		return nil
	}
}

// With a refresh token and a token file, the provider rotates the refresh
// token once and keeps the result. Terraform configures the provider anew
// for each plan, apply and destroy, and the fake voids a refresh token once
// it is used, so every later run must start from the file.
func TestAccProvider_tokenStoreFile(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	file := filepath.Join(t.TempDir(), "state", "tokens.json")
	config := providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
  token_store   = { file = %q }
`, f.baseURL(), file))

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  providerCheckRotations(f, "xoxe-refresh-0"),
			},
		},
	})

	// A second run, as after a separate plan, still works with the voided
	// refresh token in its configuration.
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  providerCheckRotations(f, "xoxe-refresh-0"),
			},
		},
	})

	// A run after the stored token expired, as on the next day, rotates the
	// stored refresh token, not the configured one.
	providerExpireStoredToken(t, file)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  providerCheckRotations(f, "xoxe-refresh-0", "xoxe-refresh-1"),
			},
		},
	})
}

// providerExpireStoredToken makes the token in a token file look expired.
func providerExpireStoredToken(t *testing.T, file string) {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	record["expires_at"] = 1700000000

	if data, err = json.Marshal(record); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// The token store can come from the environment, as can the refresh token.
func TestAccProvider_tokenStoreFileFromEnvironment(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	t.Setenv("SLACK_BASE_URL", f.baseURL())
	t.Setenv("SLACK_REFRESH_TOKEN", "xoxe-refresh-0")
	t.Setenv("SLACK_TOKEN_STORE_FILE", filepath.Join(t.TempDir(), "tokens.json"))

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: providerResource(""),
				Check:  providerCheckRotations(f, "xoxe-refresh-0"),
			},
		},
	})
}

// The token_store attribute is taken whole, as the other attributes win
// over their variables: with it set, neither variable is read, so a command
// in the environment does not clash with a file in the attribute.
func TestAccProvider_tokenStoreAttributeOverEnvironment(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	file := filepath.Join(t.TempDir(), "tokens.json")
	t.Setenv("SLACK_TOKEN_STORE_COMMAND", "/nonexistent/helper")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
  token_store   = { file = %q }
`, f.baseURL(), file)),
				Check: resource.ComposeTestCheckFunc(
					providerCheckRotations(f, "xoxe-refresh-0"),
					func(*terraform.State) error {
						if _, err := os.Stat(file); err != nil {
							return fmt.Errorf("the token file was not written: %w", err)
						}

						return nil
					},
				),
			},
		},
	})
}

// providerTokenHelper writes a token store helper that keeps the tokens in
// a file, and returns its path.
func providerTokenHelper(t *testing.T) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the helper is a shell script")
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "helper.sh")
	script := fmt.Sprintf(`#!/bin/sh
case "$1" in
  get) cat %[1]q 2>/dev/null || true ;;
  store) cat > %[1]q ;;
  *) echo "unknown operation $1" >&2; exit 2 ;;
esac
`, filepath.Join(dir, "tokens.json"))
	if err := os.WriteFile(helper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	return helper
}

func TestAccProvider_tokenStoreCommand(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	helper := providerTokenHelper(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
  token_store   = { command = [%q] }
`, f.baseURL(), helper)),
				Check: providerCheckRotations(f, "xoxe-refresh-0"),
			},
		},
	})
}

// With the helper from the environment, and no token in the configuration,
// the provider starts from what the helper keeps.
func TestAccProvider_tokenStoreCommandFromEnvironment(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	helper := providerTokenHelper(t)
	t.Setenv("SLACK_BASE_URL", f.baseURL())
	t.Setenv("SLACK_TOKEN_STORE_COMMAND", helper)

	seed := exec.CommandContext(t.Context(), helper, "store")
	seed.Stdin = strings.NewReader(`{"version":1,"refresh_token":"xoxe-refresh-kept"}`)
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seeding the helper: %v: %s", err, out)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: providerResource(""),
				Check:  providerCheckRotations(f, "xoxe-refresh-kept"),
			},
		},
	})
}

func TestAccProvider_tokenStoreInvalid(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)

	tests := map[string]struct {
		attributes string
		want       *regexp.Regexp
	}{
		"both": {
			attributes: `token_store = { file = "tokens.json", command = ["helper"] }`,
			want:       regexp.MustCompile(`(?s)Invalid Attribute Combination.*command`),
		},
		"neither": {
			attributes: `token_store = {}`,
			want:       regexp.MustCompile(`(?s)Invalid Attribute Combination.*command`),
		},
		"empty command": {
			attributes: `token_store = { command = [] }`,
			want:       regexp.MustCompile(`(?s)Invalid Attribute Value.*at least 1`),
		},
		"empty program": {
			attributes: `token_store = { command = [""] }`,
			want:       regexp.MustCompile(`(?s)Invalid Attribute Value Length.*at least 1`),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: providerFactories,
				Steps: []resource.TestStep{
					{
						Config: providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
  %s
`, f.baseURL(), tt.attributes)),
						ExpectError: tt.want,
					},
				},
			})
		})
	}
}

// providerUnknownError matches an error that names each attribute in names,
// in order, as not known until apply.
func providerUnknownError(names []string) *regexp.Regexp {
	patterns := make([]string, len(names))
	for i, name := range names {
		patterns[i] = regexp.QuoteMeta(name) + `\s+is not known until apply`
	}

	return regexp.MustCompile(`(?s)` + strings.Join(patterns, `.*`))
}

// A token attribute Terraform knows only at apply is an error, not unset:
// unset, the provider would rotate the refresh token without keeping the
// result.
func TestAccProvider_tokenUnknown(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)

	tests := map[string]struct {
		attributes string
		want       []string
	}{
		"app configuration token": {
			attributes: `app_configuration_token = terraform_data.later.output`,
			want:       []string{"app_configuration_token"},
		},
		"refresh token": {
			attributes: `refresh_token = terraform_data.later.output`,
			want:       []string{"refresh_token"},
		},
		"token store": {
			attributes: `token_store = terraform_data.later.output == "" ? null : { file = "tokens.json" }`,
			want:       []string{"token_store"},
		},
		"file": {
			attributes: `token_store = { file = terraform_data.later.output }`,
			want:       []string{"token_store.file"},
		},
		"command": {
			attributes: `token_store = { command = [terraform_data.later.output] }`,
			want:       []string{"token_store.command"},
		},
		// Every unknown attribute is named at once, not one per run.
		"several": {
			attributes: `
  refresh_token = terraform_data.later.output
  token_store   = { file = terraform_data.later.output }
`,
			want: []string{"refresh_token", "token_store.file"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: providerFactories,
				Steps: []resource.TestStep{
					{
						Config: `
resource "terraform_data" "later" {
  input = "xoxe-later"
}
` + providerResource(fmt.Sprintf(`
  base_url = %q
  %s
`, f.baseURL(), tt.attributes)),
						ExpectError: providerUnknownError(tt.want),
					},
				},
			})
		})
	}

	if got := f.rotatedRefreshTokens(); len(got) != 0 {
		t.Errorf("rotated %q although the configuration was not known", got)
	}
}

// The environment may name a file or a command, but not both.
func TestAccProvider_tokenStoreEnvironmentConflict(t *testing.T) {
	providerUnsetEnvironment(t)

	t.Setenv("SLACK_REFRESH_TOKEN", "xoxe-refresh-0")
	t.Setenv("SLACK_TOKEN_STORE_FILE", "tokens.json")
	t.Setenv("SLACK_TOKEN_STORE_COMMAND", "helper")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      providerResource(""),
				ExpectError: regexp.MustCompile(`either a file or a command, not both`),
			},
		},
	})
}

func TestAccProvider_refreshTokenRejected(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)
	f.failWith("tooling.tokens.rotate", map[string]any{"ok": false, "error": "invalid_refresh_token"})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
`, f.baseURL())),
				ExpectError: regexp.MustCompile("invalid_refresh_token"),
			},
		},
	})
}
