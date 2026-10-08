# Terraform Provider for Slack Apps

[![CI](https://github.com/ymm-oss/terraform-provider-slackapp/actions/workflows/ci.yml/badge.svg)](https://github.com/ymm-oss/terraform-provider-slackapp/actions/workflows/ci.yml)
[![Codecov](https://codecov.io/gh/ymm-oss/terraform-provider-slackapp/graph/badge.svg)](https://codecov.io/gh/ymm-oss/terraform-provider-slackapp)

> [!NOTE]
> This provider was published as `yumemi-inc/slackapp` up to v0.2.9. Later releases are [ymm-oss/slackapp](https://registry.terraform.io/providers/ymm-oss/slackapp).
>
> To switch, change `source` to `"ymm-oss/slackapp"`, then run:
>
> ```bash
> terraform state replace-provider yumemi-inc/slackapp ymm-oss/slackapp
> terraform init
> ```

Terraform provider for managing Slack Apps using [app manifest](https://api.slack.com/automation/manifest).


## Examples

### Minimum Application

```terraform
terraform {
  required_providers {
    slackapp = {
      source  = "ymm-oss/slackapp"
      version = "~> 0.4.0"
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

  // Slack voids a refresh token once it is rotated, so keep the newest tokens between runs.
  // See "Keeping rotated tokens" below.
  token_store = {
    file = "/var/lib/atlantis/slackapp-tokens.json"
  }
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
  value = slackapp_application.default.id
}
```

### Keeping rotated tokens

An app configuration token expires after 12 hours. With a refresh token, the provider rotates it for a new one. Slack voids the refresh token it rotates, and revokes the old app configuration token.

> [!IMPORTANT]
> Without a token store, the configured refresh token works for one run only. Terraform restarts the provider for each plan, apply and destroy, so a later run uses a voided token and fails with `invalid_refresh_token`.

Set `token_store` to keep the newest tokens between runs:

<table>
<thead>
<tr><th>Store</th><th>Use it when</th><th>Set it with</th></tr>
</thead>
<tbody>
<tr>
<td><code>file</code></td>
<td>The disk outlives a run, as on an Atlantis server or your own machine</td>
<td>

```terraform
provider "slackapp" {
  token_store = {
    file = "/var/lib/atlantis/slackapp-tokens.json"
  }
}
```

```sh
export SLACK_TOKEN_STORE_FILE=/var/lib/atlantis/slackapp-tokens.json
```

</td>
</tr>
<tr>
<td><code>command</code></td>
<td>Each run starts on a clean disk, as on a CI runner</td>
<td>

```terraform
provider "slackapp" {
  token_store = {
    command = ["/path/to/helper", "--any", "args"]
  }
}
```

```sh
# A program with no arguments
export SLACK_TOKEN_STORE_COMMAND=/path/to/helper
```

</td>
</tr>
</tbody>
</table>

The `token_store` attribute wins over the environment variables. With it set, neither variable is read.

The provider reads the store before it uses the configured tokens. It rotates only when the stored token is about to expire, or when Slack refuses it. When you generate a new pair of tokens and configure them, the provider starts from the new pair.

A file store is written with mode `0600`. Runs that share it take turns through a lock file beside it, so two runs never rotate the same refresh token. The lock may not work on a network file system, so keep the file on a local disk.

A command store runs your program with one more argument:

| Argument | The program must |
| --- | --- |
| `get` | Write what it last received from `store` to stdout. Write nothing when it holds nothing yet. |
| `store` | Read all of stdin and keep it, replacing what it held. |

> [!CAUTION]
> When the program fails, the provider shows its stderr in the error. Never print the tokens to stderr, for example with `set -x`.

> [!WARNING]
> The provider takes no lock around a command store. Do not start two runs that share one at the same time, or make the program serialize them.

For example, this program keeps the tokens in AWS Secrets Manager. The secret is `slackapp-tokens`, or what `SLACKAPP_TOKENS_SECRET_ID` names. The program creates it on the first `store`. It needs `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue` and `secretsmanager:CreateSecret`.

```bash
#!/usr/bin/env bash
# Keeps the slackapp provider's tokens in AWS Secrets Manager.
set -euo pipefail

secret_id=${SLACKAPP_TOKENS_SECRET_ID:-slackapp-tokens}

# The AWS CLI's errors go here, apart from the tokens on stdout.
err=$(mktemp)
trap 'rm -f "$err"' EXIT

case $1 in
get)
  if value=$(aws secretsmanager get-secret-value --secret-id "$secret_id" \
    --query SecretString --output text 2>"$err"); then
    printf '%s\n' "$value"
  elif ! grep -q ResourceNotFoundException "$err"; then
    cat "$err" >&2
    exit 1
  fi
  # No secret yet: print nothing.
  ;;
store)
  value=$(cat)
  # file:///dev/stdin keeps the tokens out of the process list.
  if ! printf '%s' "$value" | aws secretsmanager put-secret-value --secret-id "$secret_id" \
    --secret-string file:///dev/stdin >/dev/null 2>"$err"; then
    if ! grep -q ResourceNotFoundException "$err"; then
      cat "$err" >&2
      exit 1
    fi
    printf '%s' "$value" | aws secretsmanager create-secret --name "$secret_id" \
      --secret-string file:///dev/stdin >/dev/null
  fi
  ;;
*)
  echo "usage: $0 get|store" >&2
  exit 2
  ;;
esac
```

| Behavior | Why |
| --- | --- |
| `get` prints nothing while the secret does not exist | The provider then starts from the configured refresh token |
| Any other error fails, with the AWS CLI's message on stderr | An empty answer for a failed read would make the provider rotate a voided token |
| `store` passes the tokens through `file:///dev/stdin` | Command-line arguments show in the process list |

## Development

Every tool is pinned in [`mise.toml`](mise.toml). Run `mise install` once, then:

| Command | What it does |
| --- | --- |
| `mise run build` | Build the provider binary |
| `mise run coverage` | Run the tests and report statement coverage per package |
| `mise run test` | Run the unit tests, and the acceptance tests that drive the provider through Terraform against a fake Slack API |
| `mise run lint` | Run golangci-lint (linters and formatters) and [declscope](https://github.com/mpyw/declscope) |
| `mise run fix` | Apply the fixes that golangci-lint and declscope suggest |
| `mise run docs` | Regenerate `docs/` from the provider schema |

## Releasing

Run the **Tag and Release** workflow from the Actions tab with a version such as `v1.2.3`. It pushes the tag and then publishes a signed release, which the Terraform Registry picks up.

To publish a tag that already exists, run the **Release** workflow instead.

The signing key is held in the `release` environment as `GPG_PRIVATE_KEY` and `PASSPHRASE`. Its public key, `44635C222EB523FD`, is registered for the ymm-oss namespace on the Terraform Registry.
