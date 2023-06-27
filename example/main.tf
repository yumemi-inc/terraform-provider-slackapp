terraform {
  required_providers {
    slackapp = {
      source = "yumemi-inc/slackapp"
    }
  }
}

provider "slackapp" {
}

data "slackapp_manifest" "default" {
  display_information {
    name = "Example Slack App"
  }

  settings {
    org_deploy_enabled  = false
    socket_mode_enabled = false
  }
}

resource "slackapp_application" "default" {
  manifest = data.slackapp_manifest.default.json
}

output "manifest_json" {
  value = data.slackapp_manifest.default.json
}
