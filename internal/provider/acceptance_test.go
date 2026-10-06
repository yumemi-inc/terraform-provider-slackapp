package provider_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider"
)

// These tests drive the provider through the real Terraform CLI, against
// fakeSlack. They use resource.UnitTest rather than resource.Test, since
// nothing leaves the machine: they need no TF_ACC, only a terraform binary on
// PATH, which mise.toml pins.

var acceptanceProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"slackapp": providerserver.NewProtocol6WithError(provider.New("test")()),
}

const acceptanceResource = "slackapp_application.test"

// acceptanceBaseConfig is a provider pointed at the fake, and a manifest
// built with the slackapp_manifest data source.
func acceptanceBaseConfig(f *fakeSlack, name string) string {
	return fmt.Sprintf(`
provider "slackapp" {
  base_url                = %q
  app_configuration_token = "test"
}

data "slackapp_manifest" "test" {
  display_information {
    name = %q
  }

  features {
    bot_user {
      display_name = "Test Bot"
    }
  }

  oauth_config {
    scopes {
      bot = ["commands", "chat:write"]
    }
  }

  settings {
    socket_mode_enabled = true
  }
}
`, f.baseURL(), name)
}

// acceptanceConfig adds one app built from that manifest.
func acceptanceConfig(f *fakeSlack, name string) string {
	return acceptanceBaseConfig(f, name) + `
resource "slackapp_application" "test" {
  manifest = data.slackapp_manifest.test.json
}
`
}

// acceptanceCheckName checks the name Slack holds for the app in state.
func acceptanceCheckName(f *fakeSlack, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[acceptanceResource]
		if !ok {
			return fmt.Errorf("%s is not in state", acceptanceResource)
		}

		raw, ok := f.manifest(rs.Primary.ID)
		if !ok {
			return fmt.Errorf("Slack has no app %s", rs.Primary.ID)
		}

		var m struct {
			DisplayInformation struct {
				Name string `json:"name"`
			} `json:"display_information"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}

		if m.DisplayInformation.Name != want {
			return fmt.Errorf("Slack holds name %q, want %q", m.DisplayInformation.Name, want)
		}

		return nil
	}
}

// acceptanceCheckDestroyed checks that destroy deleted every app.
func acceptanceCheckDestroyed(f *fakeSlack) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if n := f.appCount(); n != 0 {
			return fmt.Errorf("Slack still has %d app(s) after destroy", n)
		}

		return nil
	}
}

// Each step's apply is followed by a plan that must be empty, so a manifest
// that drifts after create or update fails here.
func TestAccApplication_lifecycle(t *testing.T) {
	f := newFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceConfig(f, "Example"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(acceptanceResource, "id"),
					resource.TestCheckResourceAttrSet(acceptanceResource, "credentials.client_secret"),
					resource.TestCheckResourceAttrSet(acceptanceResource, "oauth_authorize_url"),
					acceptanceCheckName(f, "Example"),
				),
			},
			{
				Config: acceptanceConfig(f, "Renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					acceptanceCheckName(f, "Renamed"),
					func(*terraform.State) error {
						if n := f.callCount("apps.manifest.create"); n != 1 {
							return fmt.Errorf("apps.manifest.create called %d times, want 1: a rename must update in place", n)
						}

						return nil
					},
				),
			},
		},
	})
}

// An imported app has no credentials in state, since only
// apps.manifest.create returns them. The plan after import must still be
// empty.
//
// This does not guard the KeepPrior plan modifiers on its own: they matter
// only when something else in the plan changes, and fakeSlack returns the
// manifest unchanged, so the plan is empty with or without them.
//
// The app is first taken out of state with a removed block, which keeps it in
// Slack, and then brought back with an import block. Terraform plans again
// after each apply, so a non-empty plan after the import fails the test.
func TestAccApplication_import(t *testing.T) {
	f := newFakeSlack(t)

	// The only app this test creates.
	appID := fakeSlackAppID(1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceConfig(f, "Example"),
				Check:  resource.TestCheckResourceAttr(acceptanceResource, "id", appID),
			},
			{
				Config: acceptanceBaseConfig(f, "Example") + `
removed {
  from = slackapp_application.test

  lifecycle {
    destroy = false
  }
}
`,
				Check: func(*terraform.State) error {
					if _, ok := f.manifest(appID); !ok {
						return fmt.Errorf("removed block deleted app %s from Slack", appID)
					}

					return nil
				},
			},
			{
				Config: acceptanceConfig(f, "Example") + fmt.Sprintf(`
import {
  to = slackapp_application.test
  id = %q
}
`, appID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(acceptanceResource, "id", appID),
					resource.TestCheckNoResourceAttr(acceptanceResource, "credentials.client_secret"),
					resource.TestCheckNoResourceAttr(acceptanceResource, "oauth_authorize_url"),
				),
			},
		},
	})
}
