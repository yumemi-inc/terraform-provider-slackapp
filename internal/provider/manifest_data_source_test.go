package provider_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const manifestDataSourceFullConfig = `
provider "slackapp" {
  app_configuration_token = "test"
}

data "slackapp_manifest" "test" {
  metadata {
    major_version = 1
    minor_version = 2
  }

  display_information {
    name             = "Example App"
    description      = "Short"
    long_description = "Long"
    background_color = "#4a154b"
  }

  features {
    unfurl_domains = ["example.com", "example.org"]

    app_home {
      home_tab_enabled               = true
      messages_tab_enabled           = false
      messages_tab_read_only_enabled = true
    }

    bot_user {
      display_name  = "Example Bot"
      always_online = false
    }

    shortcut {
      name        = "Global"
      callback_id = "global_cb"
      description = "A global shortcut"
      type        = "global"
    }

    shortcut {
      name        = "Message"
      callback_id = "message_cb"
      description = "A message shortcut"
      type        = "message"
    }

    slash_command {
      command       = "/hello"
      description   = "Say hello"
      should_escape = true
      url           = "https://example.com/hello"
      usage_hint    = "[name]"
    }

    slash_command {
      command     = "/bye"
      description = "Say bye"
    }

    workflow_step {
      name        = "Step"
      callback_id = "step_cb"
    }
  }

  oauth_config {
    redirect_urls = ["https://example.com/b", "https://example.com/a"]

    scopes {
      bot  = ["commands", "chat:write"]
      user = ["identify"]
    }
  }

  settings {
    allowed_ip_address_ranges = ["192.0.2.0/24", "198.51.100.1"]
    org_deploy_enabled        = true
    socket_mode_enabled       = false
    token_rotation_enabled    = true

    event_subscriptions {
      request_url = "https://example.com/events"
      bot_events  = ["app_mention", "message.channels"]
      user_events = ["reaction_added"]
    }

    interactivity {
      is_enabled               = true
      request_url              = "https://example.com/interactivity"
      message_menu_options_url = "https://example.com/options"
    }
  }
}
`

// manifestDataSourceFullJSON is what manifestDataSourceFullConfig outputs,
// byte for byte. Keys follow internal/manifest's struct order, and the
// attributes that are sets (redirect_urls, scopes, ...) come out sorted.
const manifestDataSourceFullJSON = `{"_metadata":{"major_version":1,"minor_version":2},"display_information":{"name":"Example App","description":"Short","long_description":"Long","background_color":"#4a154b"},"settings":{"allowed_ip_address_ranges":["192.0.2.0/24","198.51.100.1"],"event_subscriptions":{"request_url":"https://example.com/events","bot_events":["app_mention","message.channels"],"user_events":["reaction_added"]},"interactivity":{"is_enabled":true,"request_url":"https://example.com/interactivity","message_menu_options_url":"https://example.com/options"},"org_deploy_enabled":true,"socket_mode_enabled":false,"token_rotation_enabled":true},"features":{"app_home":{"home_tab_enabled":true,"messages_tab_enabled":false,"messages_tab_read_only_enabled":true},"bot_user":{"display_name":"Example Bot","always_online":false},"shortcuts":[{"name":"Global","callback_id":"global_cb","description":"A global shortcut","type":"global"},{"name":"Message","callback_id":"message_cb","description":"A message shortcut","type":"message"}],"slash_commands":[{"command":"/hello","description":"Say hello","should_escape":true,"url":"https://example.com/hello","usage_hint":"[name]"},{"command":"/bye","description":"Say bye"}],"unfurl_domains":["example.com","example.org"],"workflow_steps":[{"name":"Step","callback_id":"step_cb"}]},"oauth_config":{"redirect_urls":["https://example.com/a","https://example.com/b"],"scopes":{"bot":["chat:write","commands"],"user":["identify"]}}}`

// Every block and attribute the data source has, and the exact JSON it
// outputs. A change to this output changes every config built with the data
// source, so it must be deliberate.
func TestAccManifestDataSource_full(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: manifestDataSourceFullConfig,
				Check:  resource.TestCheckResourceAttr("data.slackapp_manifest.test", "json", manifestDataSourceFullJSON),
			},
		},
	})
}

// manifestDataSourceConfig is a data source with the given blocks, inside a
// provider that needs no Slack API.
func manifestDataSourceConfig(blocks string) string {
	return `
provider "slackapp" {
  app_configuration_token = "test"
}

data "slackapp_manifest" "test" {
` + blocks + `
}
`
}

// Blocks and attributes left out are left out of the JSON, rather than
// written as null or false.
func TestAccManifestDataSource_minimal(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: manifestDataSourceConfig(`
  display_information {
    name = "Minimal"
  }
`),
				Check: resource.TestCheckResourceAttr("data.slackapp_manifest.test", "json", `{"display_information":{"name":"Minimal"}}`),
			},
			{
				Config: manifestDataSourceConfig(`
  display_information {
    name = "Minimal"
  }

  features {
    bot_user {
      display_name = "Bot"
    }
  }

  settings {
    socket_mode_enabled = true
  }
`),
				Check: resource.TestCheckResourceAttr("data.slackapp_manifest.test", "json", `{"display_information":{"name":"Minimal"},"settings":{"socket_mode_enabled":true},"features":{"bot_user":{"display_name":"Bot"}}}`),
			},
		},
	})
}

// The data source rejects what Slack would reject, at plan time.
func TestAccManifestDataSource_invalid(t *testing.T) {
	cases := map[string]struct {
		blocks string
		error  string
	}{
		"display_information is required": {
			blocks: ``,
			error:  `display_information`,
		},
		"name longer than 35 characters": {
			blocks: `
  display_information {
    name = "A name that is far too long for Slack's limit"
  }
`,
			error: `length must be at most 35`,
		},
		"background_color not a hex colour": {
			blocks: `
  display_information {
    name             = "A"
    background_color = "purple"
  }
`,
			error: `background_color`,
		},
		"bot display_name with a character Slack refuses": {
			blocks: `
  display_information {
    name = "A"
  }
  features {
    bot_user {
      display_name = "Bot!"
    }
  }
`,
			error: `display_name`,
		},
		"slash command without a leading slash": {
			blocks: `
  display_information {
    name = "A"
  }
  features {
    slash_command {
      command     = "hello"
      description = "x"
    }
  }
`,
			error: "must start with `/`",
		},
		"shortcut of an unknown type": {
			blocks: `
  display_information {
    name = "A"
  }
  features {
    shortcut {
      name        = "S"
      callback_id = "s"
      description = "x"
      type        = "channel"
    }
  }
`,
			error: `value must be one of`,
		},
		"allowed IP range that is not IPv4": {
			blocks: `
  display_information {
    name = "A"
  }
  settings {
    allowed_ip_address_ranges = ["example.com"]
  }
`,
			error: `valid IPv4 address`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: providerFactories,
				Steps: []resource.TestStep{
					{
						Config:      manifestDataSourceConfig(tc.blocks),
						ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.error)),
					},
				},
			})
		})
	}
}

// More than five slash commands or shortcuts is beyond Slack's documented
// limit, which Slack does not enforce today, so it warns and still outputs.
func TestAccManifestDataSource_overDocumentedLimits(t *testing.T) {
	var blocks strings.Builder
	blocks.WriteString(`
  display_information {
    name = "A"
  }
  features {
`)
	for i := range 6 {
		fmt.Fprintf(&blocks, `
    slash_command {
      command     = "/c%d"
      description = "x"
    }
    shortcut {
      name        = "S%d"
      callback_id = "s%d"
      description = "x"
      type        = "global"
    }
`, i, i, i)
	}
	blocks.WriteString("  }\n")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: manifestDataSourceConfig(blocks.String()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.slackapp_manifest.test", "json"),
					resource.TestCheckResourceAttr("data.slackapp_manifest.test", "features.slash_command.#", "6"),
					resource.TestCheckResourceAttr("data.slackapp_manifest.test", "features.shortcut.#", "6"),
				),
			},
		},
	})
}
