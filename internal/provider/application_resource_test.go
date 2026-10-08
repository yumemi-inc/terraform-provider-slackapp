package provider_test

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

// These tests drive the provider through the real Terraform CLI, against
// fakeSlack. They use resource.UnitTest rather than resource.Test, since
// nothing leaves the machine: they need no TF_ACC, only a terraform binary on
// PATH, which mise.toml pins.

const applicationResourceResource = "slackapp_application.test"

// applicationResourceBaseConfig is a provider pointed at the fake, and a manifest
// built with the slackapp_manifest data source.
func applicationResourceBaseConfig(f *fakeSlack, name string) string {
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

// applicationResourceConfig adds one app built from that manifest.
func applicationResourceConfig(f *fakeSlack, name string) string {
	return applicationResourceBaseConfig(f, name) + `
resource "slackapp_application" "test" {
  manifest = data.slackapp_manifest.test.json
}
`
}

// applicationResourceCheckName checks the name Slack holds for the app in state.
func applicationResourceCheckName(f *fakeSlack, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[applicationResourceResource]
		if !ok {
			return fmt.Errorf("%s is not in state", applicationResourceResource)
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

// applicationResourceCheckDestroyed checks that destroy deleted every app.
//
//declscope:shared // provider_test.go checks the refresh-token test cleaned up
func applicationResourceCheckDestroyed(f *fakeSlack) resource.TestCheckFunc {
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceConfig(f, "Example"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(applicationResourceResource, "id"),
					resource.TestCheckResourceAttrSet(applicationResourceResource, "credentials.client_secret"),
					resource.TestCheckResourceAttrSet(applicationResourceResource, "oauth_authorize_url"),
					applicationResourceCheckName(f, "Example"),
				),
			},
			{
				Config: applicationResourceConfig(f, "Renamed"),
				Check: resource.ComposeAggregateTestCheckFunc(
					applicationResourceCheckName(f, "Renamed"),
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceConfig(f, "Example"),
				Check:  resource.TestCheckResourceAttr(applicationResourceResource, "id", appID),
			},
			{
				Config: applicationResourceBaseConfig(f, "Example") + `
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
				Config: applicationResourceConfig(f, "Example") + fmt.Sprintf(`
import {
  to = slackapp_application.test
  id = %q
}
`, appID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(applicationResourceResource, "id", appID),
					resource.TestCheckNoResourceAttr(applicationResourceResource, "credentials.client_secret"),
					resource.TestCheckNoResourceAttr(applicationResourceResource, "oauth_authorize_url"),
				),
			},
		},
	})
}

// applicationResourceRichConfig is applicationResourceConfig with more than one entry in each
// array Slack treats as a set, so that reordering them shows.
func applicationResourceRichConfig(f *fakeSlack, name string) string {
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

// applicationResourceJSONConfig is an app whose manifest is written with jsonencode,
// and carries outgoing_domains, which the slackapp_manifest data source does not
// model.
func applicationResourceJSONConfig(f *fakeSlack, domain string) string {
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

// applicationResourceCheckOutgoingDomains checks the outgoing_domains Slack holds for
// the app in state.
func applicationResourceCheckOutgoingDomains(f *fakeSlack, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[applicationResourceResource]
		if !ok {
			return fmt.Errorf("%s is not in state", applicationResourceResource)
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceRichConfig(f, "Example"),
				Check:  applicationResourceCheckName(f, "Example"),
			},
			{
				Config: applicationResourceRichConfig(f, "Renamed"),
				Check:  applicationResourceCheckName(f, "Renamed"),
			},
		},
	})
}

// A field the provider does not model must still reach Slack when it is the
// only thing that changes, and must not drift when it does not.
func TestAccApplication_unmodeledField(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceJSONConfig(f, "a.example.com"),
				Check:  applicationResourceCheckOutgoingDomains(f, "a.example.com"),
			},
			{
				Config: applicationResourceJSONConfig(f, "b.example.com"),
				Check:  applicationResourceCheckOutgoingDomains(f, "b.example.com"),
			},
		},
	})
}

// applicationResourceDescriptionConfig is an app whose manifest is written with
// jsonencode, with or without a description.
func applicationResourceDescriptionConfig(f *fakeSlack, description string) string {
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

// applicationResourceCheckDescription checks the description Slack holds for the app
// in state, where "" means none.
func applicationResourceCheckDescription(f *fakeSlack, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[applicationResourceResource]
		if !ok {
			return fmt.Errorf("%s is not in state", applicationResourceResource)
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
				Check:  applicationResourceCheckDescription(f, "Old"),
			},
			{
				Config: applicationResourceDescriptionConfig(f, ""),
				Check:  applicationResourceCheckDescription(f, ""),
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
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
				Config:             applicationResourceDescriptionConfig(f, "Old"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
				Check:  applicationResourceCheckDescription(f, "Old"),
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
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceRichConfig(f, "Example"),
			},
			{
				Config: applicationResourceBaseConfig(f, "Example") + `
removed {
  from = slackapp_application.test

  lifecycle {
    destroy = false
  }
}
`,
			},
			{
				Config: applicationResourceRichConfig(f, "Example") + fmt.Sprintf(`
import {
  to = slackapp_application.test
  id = %q
}
`, appID),
				Check: applicationResourceCheckName(f, "Example"),
			},
		},
	})
}

// When Slack lists what is wrong with a manifest, each item is shown as its
// own error, under the message Slack gave. The detail names the operation and
// where in the manifest the item points.
func TestAccApplication_createFailsWithErrors(t *testing.T) {
	f := newRewritingFakeSlack(t)
	f.failWith("apps.manifest.create", map[string]any{
		"ok":    false,
		"error": "invalid_manifest",
		"errors": []map[string]string{
			{"message": "Event subscription requires a request URL", "pointer": "/settings/event_subscriptions"},
		},
	})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config:      applicationResourceDescriptionConfig(f, "Old"),
				ExpectError: regexp.MustCompile(`(?s)Error: Event subscription requires a request URL.*Failed to create the Slack app.*/settings/event_subscriptions`),
			},
		},
	})
}

// Without a list, the summary names the operation, and the error code Slack
// gave is shown.
func TestAccApplication_createFails(t *testing.T) {
	f := newRewritingFakeSlack(t)
	f.failWith("apps.manifest.create", map[string]any{"ok": false, "error": "ratelimited"})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config:      applicationResourceDescriptionConfig(f, "Old"),
				ExpectError: regexp.MustCompile(`(?s)Error: Failed to create the Slack app.*ratelimited`),
			},
		},
	})
}

// A failed update leaves the app as it was, and the next apply sends the
// change again. The error says that the update failed, not the create.
func TestAccApplication_updateFails(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
			},
			{
				PreConfig: func() {
					f.failWith("apps.manifest.update", map[string]any{"ok": false, "error": "invalid_manifest"})
				},
				Config:      applicationResourceDescriptionConfig(f, "New"),
				ExpectError: regexp.MustCompile(`(?s)Error: Failed to update the Slack app.*invalid_manifest`),
			},
			{
				PreConfig: func() { f.succeed("apps.manifest.update") },
				Config:    applicationResourceDescriptionConfig(f, "New"),
				Check:     applicationResourceCheckDescription(f, "New"),
			},
		},
	})
}

// A list of errors on update is shown as on create, with the detail naming
// the update.
func TestAccApplication_updateFailsWithErrors(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
			},
			{
				PreConfig: func() {
					f.failWith("apps.manifest.update", map[string]any{
						"ok":    false,
						"error": "invalid_manifest",
						"errors": []map[string]string{
							{"message": "Description is too long", "pointer": "/display_information/description"},
						},
					})
				},
				Config:      applicationResourceDescriptionConfig(f, "New"),
				ExpectError: regexp.MustCompile(`(?s)Error: Description is too long.*Failed to update the Slack app.*/display_information/description`),
			},
			{
				PreConfig: func() { f.succeed("apps.manifest.update") },
				Config:    applicationResourceDescriptionConfig(f, "New"),
				Check:     applicationResourceCheckDescription(f, "New"),
			},
		},
	})
}

// A failed refresh stops the plan, rather than planning from stale state. The
// error says that reading the app failed.
func TestAccApplication_refreshFails(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
			},
			{
				PreConfig: func() {
					f.failWith("apps.manifest.export", map[string]any{"ok": false, "error": "internal_error"})
				},
				Config:      applicationResourceDescriptionConfig(f, "Old"),
				ExpectError: regexp.MustCompile(`(?s)Error: Failed to read the Slack app.*internal_error`),
			},
			{
				PreConfig: func() { f.succeed("apps.manifest.export") },
				Config:    applicationResourceDescriptionConfig(f, "Old"),
			},
		},
	})
}

// applicationResourceDeleteInSlack deletes app n in the fake through the API,
// the way someone outside Terraform would.
func applicationResourceDeleteInSlack(t *testing.T, f *fakeSlack, n int) {
	t.Helper()

	if _, err := applicationResourceSlackClient(f).AppsManifestDelete(t.Context(), slack.AppsManifestDeleteRequest{AppID: fakeSlackAppID(n)}); err != nil {
		t.Fatal(err)
	}
}

// An app deleted in Slack, outside Terraform, leaves state on refresh, so
// the plan creates it again. The new app has a new ID and new credentials.
func TestAccApplication_deletedInSlack(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(applicationResourceResource, "id", fakeSlackAppID(1)),
					resource.TestCheckResourceAttr(applicationResourceResource, "credentials.client_secret", "secret-"+fakeSlackAppID(1)),
				),
			},
			{
				PreConfig: func() { applicationResourceDeleteInSlack(t, f, 1) },
				Config:    applicationResourceDescriptionConfig(f, "Old"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(applicationResourceResource, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(applicationResourceResource, "id", fakeSlackAppID(2)),
					resource.TestCheckResourceAttr(applicationResourceResource, "credentials.client_id", "client-"+fakeSlackAppID(2)),
					resource.TestCheckResourceAttr(applicationResourceResource, "credentials.client_secret", "secret-"+fakeSlackAppID(2)),
					resource.TestCheckResourceAttr(applicationResourceResource, "credentials.verification_token", "verification-"+fakeSlackAppID(2)),
					resource.TestCheckResourceAttr(applicationResourceResource, "credentials.signing_secret", "signing-"+fakeSlackAppID(2)),
					resource.TestCheckResourceAttr(applicationResourceResource, "oauth_authorize_url", "https://slack.com/oauth/v2/authorize?client_id=client-"+fakeSlackAppID(2)),
					applicationResourceCheckDescription(f, "Old"),
				),
			},
		},
	})
}

// Destroying an app already deleted in Slack, outside Terraform, succeeds.
func TestAccApplication_destroyDeletedInSlack(t *testing.T) {
	cases := map[string]struct {
		// staleExport keeps apps.manifest.export answering with the app
		// after it is deleted, so refresh keeps it and the delete itself
		// meets app_not_found. Terraform destroys without refreshing when
		// run with -refresh=false, which leads there too.
		staleExport bool
		// deletes is how many times apps.manifest.delete is called,
		// counting the one from outside Terraform.
		deletes int
	}{
		"refresh finds it gone": {staleExport: false, deletes: 1},
		"delete finds it gone":  {staleExport: true, deletes: 2},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeSlack(t)

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: providerFactories,
				CheckDestroy: resource.ComposeAggregateTestCheckFunc(
					applicationResourceCheckDestroyed(f),
					func(*terraform.State) error {
						if n := f.callCount("apps.manifest.delete"); n != tc.deletes {
							return fmt.Errorf("apps.manifest.delete called %d times, want %d", n, tc.deletes)
						}

						return nil
					},
				),
				Steps: []resource.TestStep{
					{
						Config: applicationResourceConfig(f, "Example"),
					},
					{
						PreConfig: func() {
							if tc.staleExport {
								m, ok := f.manifest(fakeSlackAppID(1))
								if !ok {
									t.Fatal("Slack has no app")
								}
								f.failWith("apps.manifest.export", map[string]any{"ok": true, "manifest": m})
							}
							applicationResourceDeleteInSlack(t, f, 1)
						},
						Config:  applicationResourceConfig(f, "Example"),
						Destroy: true,
					},
				},
			})
		})
	}
}

// apps.manifest.export answering ok but without a usable manifest is an
// error, not an empty app.
func TestAccApplication_exportWithoutManifest(t *testing.T) {
	cases := map[string]struct {
		reply map[string]any
		error string
	}{
		"no manifest":       {map[string]any{"ok": true}, `Slack API returned empty manifest`},
		"a null manifest":   {map[string]any{"ok": true, "manifest": nil}, `Slack API returned empty manifest`},
		"not a JSON object": {map[string]any{"ok": true, "manifest": []string{"x"}}, `not a JSON object`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newRewritingFakeSlack(t)

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: providerFactories,
				CheckDestroy:             applicationResourceCheckDestroyed(f),
				Steps: []resource.TestStep{
					{
						Config: applicationResourceDescriptionConfig(f, "Old"),
					},
					{
						PreConfig:   func() { f.failWith("apps.manifest.export", tc.reply) },
						Config:      applicationResourceDescriptionConfig(f, "Old"),
						ExpectError: regexp.MustCompile(tc.error),
					},
					{
						PreConfig: func() { f.succeed("apps.manifest.export") },
						Config:    applicationResourceDescriptionConfig(f, "Old"),
					},
				},
			})
		})
	}
}

// A failed delete keeps the app in state, so that destroying again still
// reaches it. The error says that the delete failed.
func TestAccApplication_deleteFails(t *testing.T) {
	f := newRewritingFakeSlack(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: applicationResourceDescriptionConfig(f, "Old"),
			},
			{
				PreConfig: func() {
					f.failWith("apps.manifest.delete", map[string]any{"ok": false, "error": "internal_error"})
				},
				Config:      applicationResourceDescriptionConfig(f, "Old"),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`(?s)Error: Failed to delete the Slack app.*internal_error`),
			},
			{
				PreConfig: func() { f.succeed("apps.manifest.delete") },
				Config:    applicationResourceDescriptionConfig(f, "Old"),
				Check:     applicationResourceCheckDescription(f, "Old"),
			},
		},
	})
}

// Slack drops _metadata from the manifest it exports, so Read carries it
// over from state. Without that, a manifest with _metadata would drift
// after every apply.
func TestAccApplication_metadataKept(t *testing.T) {
	f := newRewritingFakeSlack(t)

	config := fmt.Sprintf(`
provider "slackapp" {
  base_url                = %q
  app_configuration_token = "test"
}

data "slackapp_manifest" "test" {
  metadata {
    major_version = 1
    minor_version = 1
  }

  display_information {
    name = "Example"
  }
}

resource "slackapp_application" "test" {
  manifest = data.slackapp_manifest.test.json
}
`, f.baseURL())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		CheckDestroy:             applicationResourceCheckDestroyed(f),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					func(*terraform.State) error {
						raw, ok := f.manifest(fakeSlackAppID(1))
						if !ok {
							return fmt.Errorf("Slack has no app")
						}
						if !strings.Contains(string(raw), `"_metadata"`) {
							return fmt.Errorf("the manifest sent to Slack has no _metadata: %s", raw)
						}

						return nil
					},
					resource.TestMatchResourceAttr(applicationResourceResource, "manifest", regexp.MustCompile(`"_metadata"`)),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

// applicationResourceSlackClient is a Slack client pointed at the fake, for a test
// to change Slack the way someone outside Terraform would.
func applicationResourceSlackClient(f *fakeSlack) *slack.Client {
	return slack.NewClient().WithAppConfigurationToken("test").WithBaseURL(f.baseURL())
}
