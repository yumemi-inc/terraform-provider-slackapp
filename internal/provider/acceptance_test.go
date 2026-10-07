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

// acceptanceRichConfig is acceptanceConfig with more than one entry in each
// array Slack treats as a set, so that reordering them shows.
func acceptanceRichConfig(f *fakeSlack, name string) string {
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
    redirect_urls = ["https://example.com/a", "https://example.com/b"]

    scopes {
      bot  = ["channels:read", "chat:write", "commands"]
      user = ["identify", "users:read"]
    }
  }

  settings {
    event_subscriptions {
      request_url = "https://example.com/events"
      bot_events  = ["app_mention", "message.channels"]
    }
  }
}

resource "slackapp_application" "test" {
  manifest = data.slackapp_manifest.test.json
}
`, f.baseURL(), name)
}

// acceptanceJSONConfig is an app whose manifest is written with jsonencode,
// and carries outgoing_domains, which the slackapp_manifest data source does not
// model.
func acceptanceJSONConfig(f *fakeSlack, domain string) string {
	return fmt.Sprintf(`
provider "slackapp" {
  base_url                = %q
  app_configuration_token = "test"
}

resource "slackapp_application" "test" {
  manifest = jsonencode({
    display_information = {
      name = "Example"
    }
    oauth_config = {
      scopes = {
        bot = ["commands", "chat:write"]
      }
    }
    outgoing_domains = [%q]
  })
}
`, f.baseURL(), domain)
}

// acceptanceCheckOutgoingDomains checks the outgoing_domains Slack holds for
// the app in state.
func acceptanceCheckOutgoingDomains(f *fakeSlack, want string) resource.TestCheckFunc {
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
			OutgoingDomains []string `json:"outgoing_domains"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}

		if len(m.OutgoingDomains) != 1 || m.OutgoingDomains[0] != want {
			return fmt.Errorf("Slack holds outgoing_domains %q, want [%q]", m.OutgoingDomains, want)
		}

		return nil
	}
}

// Slack exports a manifest with its keys and set-like arrays in its own
// order. The plan after each apply must still be empty.
func TestAccApplication_rewrittenExport(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceRichConfig(f, "Example"),
				Check:  acceptanceCheckName(f, "Example"),
			},
			{
				Config: acceptanceRichConfig(f, "Renamed"),
				Check:  acceptanceCheckName(f, "Renamed"),
			},
		},
	})
}

// A field the provider does not model must still reach Slack when it is the
// only thing that changes, and must not drift when it does not.
func TestAccApplication_unmodeledField(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceJSONConfig(f, "a.example.com"),
				Check:  acceptanceCheckOutgoingDomains(f, "a.example.com"),
			},
			{
				Config: acceptanceJSONConfig(f, "b.example.com"),
				Check:  acceptanceCheckOutgoingDomains(f, "b.example.com"),
			},
		},
	})
}

// acceptanceDescriptionConfig is an app whose manifest is written with
// jsonencode, with or without a description.
func acceptanceDescriptionConfig(f *fakeSlack, description string) string {
	displayInformation := `{ name = "Example" }`
	if description != "" {
		displayInformation = fmt.Sprintf(`{ name = "Example", description = %q }`, description)
	}

	return fmt.Sprintf(`
provider "slackapp" {
  base_url                = %q
  app_configuration_token = "test"
}

resource "slackapp_application" "test" {
  manifest = jsonencode({
    display_information = %s
    settings = {
      socket_mode_enabled = true
    }
  })
}
`, f.baseURL(), displayInformation)
}

// acceptanceCheckDescription checks the description Slack holds for the app
// in state, where "" means none.
func acceptanceCheckDescription(f *fakeSlack, want string) resource.TestCheckFunc {
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
				Description *string `json:"description"`
			} `json:"display_information"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return err
		}

		got := ""
		if m.DisplayInformation.Description != nil {
			got = *m.DisplayInformation.Description
		}
		if got != want {
			return fmt.Errorf("Slack holds description %q, want %q", got, want)
		}

		return nil
	}
}

// A field removed from the config must be removed from Slack too, even
// though Slack fills in defaults that the config never stated.
func TestAccApplication_removedField(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceDescriptionConfig(f, "Old"),
				Check:  acceptanceCheckDescription(f, "Old"),
			},
			{
				Config: acceptanceDescriptionConfig(f, ""),
				Check:  acceptanceCheckDescription(f, ""),
			},
		},
	})
}

// A change made in Slack to a field the config states must show as drift,
// even though fields the config does not state are ignored.
func TestAccApplication_driftInSlack(t *testing.T) {
	f := newRewritingFakeSlack(t)
	appID := fakeSlackAppID(1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceDescriptionConfig(f, "Old"),
			},
			{
				PreConfig: func() {
					f.editManifest(t, appID, func(m map[string]any) {
						displayInformation, ok := m["display_information"].(map[string]any)
						if !ok {
							t.Fatal("Slack holds no display_information")
						}
						displayInformation["description"] = "Edited in Slack"
					})
				},
				Config:             acceptanceDescriptionConfig(f, "Old"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: acceptanceDescriptionConfig(f, "Old"),
				Check:  acceptanceCheckDescription(f, "Old"),
			},
		},
	})
}

// An imported app has every default Slack fills in in state, since there is
// no prior manifest to prune it to. The import's apply sends the config
// once, and the plan after it must be empty.
func TestAccApplication_importRewritten(t *testing.T) {
	f := newRewritingFakeSlack(t)
	appID := fakeSlackAppID(1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptanceProviderFactories,
		CheckDestroy:             acceptanceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: acceptanceRichConfig(f, "Example"),
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
			},
			{
				Config: acceptanceRichConfig(f, "Example") + fmt.Sprintf(`
import {
  to = slackapp_application.test
  id = %q
}
`, appID),
				Check: acceptanceCheckName(f, "Example"),
			},
		},
	})
}
