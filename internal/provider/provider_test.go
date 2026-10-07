package provider_test

import (
	"fmt"
	"regexp"
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

	for _, name := range []string{"SLACK_APP_CONFIGURATION_TOKEN", "SLACK_REFRESH_TOKEN", "SLACK_BASE_URL"} {
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
				ExpectError: regexp.MustCompile(`either app configuration token or refresh token must be\s+provided`),
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

// With only a refresh token, the provider rotates it for an app
// configuration token before its first call.
func TestAccProvider_refreshToken(t *testing.T) {
	providerUnsetEnvironment(t)

	f := newFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: providerResource(fmt.Sprintf(`
  base_url      = %q
  refresh_token = "xoxe-refresh-0"
`, f.baseURL())),
				Check: func(*terraform.State) error {
					rotated := f.rotatedRefreshTokens()
					if len(rotated) == 0 || rotated[0] != "xoxe-refresh-0" {
						return fmt.Errorf("tooling.tokens.rotate was called with %q, want the configured refresh token first", rotated)
					}
					if got := f.authorization("apps.manifest.create"); !strings.HasPrefix(got, "Bearer xoxe.xoxp-rotated-") {
						return fmt.Errorf("apps.manifest.create sent Authorization %q, want a rotated token", got)
					}

					return nil
				},
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
