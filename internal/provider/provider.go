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

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/common"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/provider/datasources"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/provider/resources"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack"
)

func configureSlackClient(d Model) (*slack.Client, error) {
	baseURL := os.Getenv("SLACK_BASE_URL")
	appConfigurationToken := os.Getenv("SLACK_APP_CONFIGURATION_TOKEN")
	refreshToken := os.Getenv("SLACK_REFRESH_TOKEN")

	if !d.BaseURL.IsNull() {
		baseURL = d.BaseURL.ValueString()
	}

	if !d.AppConfigurationToken.IsNull() {
		appConfigurationToken = d.AppConfigurationToken.ValueString()
	}

	if !d.RefreshToken.IsNull() {
		refreshToken = d.RefreshToken.ValueString()
	}

	var client *slack.Client
	if refreshToken == "" {
		if appConfigurationToken == "" {
			return nil, errors.New("either app configuration token or refresh token must be provided")
		}

		client = slack.NewClient(appConfigurationToken)
	} else {
		client = slack.NewClientFromRefreshToken(refreshToken)
	}

	if baseURL != "" {
		client = client.WithBaseURL(baseURL)
	}

	return client, nil
}

func configure(d Model) (*common.ProviderContext, error) {
	slackClient, err := configureSlackClient(d)
	if err != nil {
		return nil, err
	}

	return &common.ProviderContext{
		SlackClient: slackClient,
	}, nil
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

	client, err := configure(data)
	if err != nil {
		response.Diagnostics.AddError("Error occurred while configuring the provider.", err.Error())
	}

	response.DataSourceData = client
	response.ResourceData = client
}

func (p *Provider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewSlackAppManifest,
	}
}

func (p *Provider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewSlackApp,
	}
}
