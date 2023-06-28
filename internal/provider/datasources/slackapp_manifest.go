package datasources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/common"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/provider/datasources/slackappmanifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/typeconv"
)

type SlackAppManifestModel struct {
	// Blocks
	Metadata           *slackappmanifest.Metadata           `tfsdk:"metadata"`
	DisplayInformation *slackappmanifest.DisplayInformation `tfsdk:"display_information"`
	Settings           *slackappmanifest.Settings           `tfsdk:"settings"`
	Features           *slackappmanifest.Features           `tfsdk:"features"`
	OauthConfig        *slackappmanifest.OauthConfig        `tfsdk:"oauth_config"`

	// Attributes
	Json types.String `tfsdk:"json"`
}

func (m *SlackAppManifestModel) Read() manifest.App {
	return manifest.App{
		Metadata:           typeconv.MapOptionModel[manifest.Metadata](m.Metadata),
		DisplayInformation: m.DisplayInformation.Read(),
		Settings:           typeconv.MapOptionModel[manifest.Settings](m.Settings),
		Features:           typeconv.MapOptionModel[manifest.Features](m.Features),
		OauthConfig:        typeconv.MapOptionModel[manifest.OauthConfig](m.OauthConfig),
	}
}

type SlackAppManifest struct {
	ctx *common.ProviderContext
}

func NewSlackAppManifest() datasource.DataSource {
	return &SlackAppManifest{}
}

func (d *SlackAppManifest) Metadata(
	_ context.Context,
	_ datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = "slackapp_manifest"
}

func (d *SlackAppManifest) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Represents manifest of the Slack App.",
		Blocks: map[string]schema.Block{
			"metadata":            (*slackappmanifest.Metadata)(nil).Schema(),
			"display_information": (*slackappmanifest.DisplayInformation)(nil).Schema(),
			"settings":            (*slackappmanifest.Settings)(nil).Schema(),
			"features":            (*slackappmanifest.Features)(nil).Schema(),
			"oauth_config":        (*slackappmanifest.OauthConfig)(nil).Schema(),
		},
		Attributes: map[string]schema.Attribute{
			"json": &schema.StringAttribute{
				MarkdownDescription: "JSON representation of the manifest.",
				Computed:            true,
			},
		},
	}
}

func (d *SlackAppManifest) Configure(
	_ context.Context,
	request datasource.ConfigureRequest,
	response *datasource.ConfigureResponse,
) {
	if request.ProviderData == nil {
		return
	}

	providerContext, ok := request.ProviderData.(*common.ProviderContext)
	if !ok {
		response.Diagnostics.AddError(
			"The ctx did not configured properly.",
			"request.ProviderData.(type) != *ctx.ConfiguredProvider",
		)

		return
	}

	d.ctx = providerContext
}

func (d *SlackAppManifest) Read(
	ctx context.Context,
	request datasource.ReadRequest,
	response *datasource.ReadResponse,
) {
	var data SlackAppManifestModel

	response.Diagnostics.Append(request.Config.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	appManifest := data.Read()

	json, err := appManifest.ToJsonString()
	if err != nil {
		response.Diagnostics.AddError("Failed to marshal the manifest into JSON.", err.Error())
	}

	data.Json = types.StringValue(json)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}
