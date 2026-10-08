// Package provider is the slackapp Terraform provider: the provider itself,
// the slackapp_application resource and the slackapp_manifest data source.
//
// It connects Terraform to packages that know nothing of it: internal/slack
// talks to Slack's API, and internal/manifest holds the data source's
// manifest model. Its subpackages hold the Terraform-facing pieces:
//
//   - manifesttype: the resource's manifest attribute type, which compares
//     manifests by meaning.
//   - planmodifiers: the resource's plan modifiers.
//   - manifestblocks: the data source's block models.
//   - validators: the limits Slack puts on a manifest.
//
//declscope:core // the package's entry point: main.go calls New
package provider

import (
	"context"
	"errors"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

func configureSlackClient(d Model) (*slack.Client, error) {
	baseURL := stringOrEnvironment(d.BaseURL, "SLACK_BASE_URL")
	appConfigurationToken := stringOrEnvironment(d.AppConfigurationToken, "SLACK_APP_CONFIGURATION_TOKEN")
	refreshToken := stringOrEnvironment(d.RefreshToken, "SLACK_REFRESH_TOKEN")

	var client *slack.Client
	if refreshToken == "" {
		if appConfigurationToken == "" {
			return nil, errors.New("either app configuration token or refresh token must be provided")
		}

		client = slack.NewClient(slack.AppConfigurationToken(appConfigurationToken))
	} else {
		client = slack.NewClientFromRefreshToken(slack.RefreshToken(refreshToken))
	}

	if baseURL != "" {
		client = client.WithBaseURL(baseURL)
	}

	return client, nil
}

// stringOrEnvironment returns the attribute's value, or the environment
// variable name when the attribute is not set.
func stringOrEnvironment(attribute types.String, name string) string {
	if attribute.IsNull() {
		return os.Getenv(name)
	}

	return attribute.ValueString()
}

type Model struct {
	AppConfigurationToken types.String `tfsdk:"app_configuration_token"`
	RefreshToken          types.String `tfsdk:"refresh_token"`
	BaseURL               types.String `tfsdk:"base_url"`
}

type Provider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &Provider{
			version,
		}
	}
}

func (p *Provider) Metadata(
	_ context.Context,
	_ provider.MetadataRequest,
	response *provider.MetadataResponse,
) {
	response.TypeName = "slackapp"
	response.Version = p.version
}

func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		// The Registry's "->" callout. Kept for users arriving from the old
		// namespace, whose last release (v0.2.9) points here.
		MarkdownDescription: "-> This provider was published as `yumemi-inc/slackapp` up to v0.2.9. " +
			"To switch, change `source` to `\"ymm-oss/slackapp\"`, then run " +
			"`terraform state replace-provider yumemi-inc/slackapp ymm-oss/slackapp` and `terraform init`.",
		Attributes: map[string]schema.Attribute{
			"app_configuration_token": schema.StringAttribute{
				MarkdownDescription: "App configuration token for the Slack Workspace.",
				Sensitive:           true,
				Optional:            true,
			},
			"refresh_token": schema.StringAttribute{
				MarkdownDescription: "Refresh token for the Slack Workspace.",
				Sensitive:           true,
				Optional:            true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the Slack API. Defaults to `https://slack.com/api/`.",
				Optional:            true,
			},
		},
	}
}

func (p *Provider) Configure(
	ctx context.Context,
	request provider.ConfigureRequest,
	response *provider.ConfigureResponse,
) {
	var data Model

	response.Diagnostics.Append(request.Config.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	client, err := configureSlackClient(data)
	if err != nil {
		response.Diagnostics.AddError("Error occurred while configuring the provider.", err.Error())
	}

	response.ResourceData = client
}

func (p *Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newManifestDataSource,
	}
}

func (p *Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newApplicationResource,
	}
}
