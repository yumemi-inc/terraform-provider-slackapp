# Terraform Provider for Slack Apps

[![Release](https://github.com/yumemi-inc/terraform-provider-slackapp/actions/workflows/release.yaml/badge.svg)](https://github.com/yumemi-inc/terraform-provider-slackapp/actions/workflows/release.yaml)

Terraform provider for managing Slack Apps using [app manifest](https://api.slack.com/automation/manifest).


## Examples

### Minimum Application

```terraform
terraform {
  required_providers {
    slackapp = {
      source  = "yumemi-inc/slackapp"
      version = "~> 0.2.4"
    }
  }
}

provider "slackapp" {
  // If you want to use non-default base URL for Slack API, use this argument to configure it.
  // It also can be set via SLACK_BASE_URL environment variable.
  base_url = "https://slack.com/api/"
  
  // App configuration token and refresh token can be retrieved from https://api.slack.com/reference/manifests#config-tokens.
  // They are special tokens to manage apps in workspace global.
  // They also can be set via SLACK_APP_CONFIGURATION_TOKEN and SLACK_REFRESH_TOKEN environment variables.
  app_configuration_token = "<YOUR_APP_CONFIGURATION_TOKEN>"
  refresh_token           = "<YOUR_REFRESH_TOKEN>"
}

// Data source slackapp_manifest is for constructing the manifest using Terraform language.
// If you want to use custom JSON representation, use jsonencode function instead.
data "slackapp_manifest" "default" {
  display_information {
    name = "Example Slack App"
  }

  settings {
    org_deploy_enabled  = false
    socket_mode_enabled = false
    token_rotation_enabled = false
  }
}

resource "slackapp_application" "default" {
  manifest = data.slackapp_manifest.default.json
}

output "app_id" {
  value = data.slackapp_application.default.id
}
```

## Development

Every tool is pinned in [`mise.toml`](mise.toml). Run `mise install` once, then:

| Command | What it does |
| --- | --- |
| `mise run build` | Build the provider binary |
| `mise run test` | Run the unit tests |
| `mise run lint` | Run golangci-lint (linters and formatters) and [declscope](https://github.com/mpyw/declscope) |
| `mise run fix` | Apply the fixes that golangci-lint and declscope suggest |
| `mise run docs` | Regenerate `docs/` from the provider schema |

## Releasing

Run the **Tag and Release** workflow from the Actions tab with a version such as `v0.3.0`. It pushes the tag and then publishes a signed release, which the Terraform Registry picks up.

To publish a tag that already exists, run the **Release** workflow instead.
