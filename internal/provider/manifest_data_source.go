package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/manifestblocks"
)

type manifestDataSourceModel struct {
	// Blocks
	Metadata           *manifestblocks.Metadata           `tfsdk:"metadata"`
	DisplayInformation *manifestblocks.DisplayInformation `tfsdk:"display_information"`
	Settings           *manifestblocks.Settings           `tfsdk:"settings"`
	Features           *manifestblocks.Features           `tfsdk:"features"`
	OauthConfig        *manifestblocks.OauthConfig        `tfsdk:"oauth_config"`

	// Attributes
	Json types.String `tfsdk:"json"`
}

func (m *manifestDataSourceModel) Read() manifest.App {
	return manifest.App{
		Metadata:           manifestblocks.MapOptionModel[manifest.Metadata](m.Metadata),
		DisplayInformation: m.DisplayInformation.Read(),
		Settings:           manifestblocks.MapOptionModel[manifest.Settings](m.Settings),
		Features:           manifestblocks.MapOptionModel[manifest.Features](m.Features),
		OauthConfig:        manifestblocks.MapOptionModel[manifest.OauthConfig](m.OauthConfig),
	}
}

type manifestDataSource struct{}

//declscope:shared // provider.go registers it
func newManifestDataSource() datasource.DataSource {
	return &manifestDataSource{}
}

func (d *manifestDataSource) Metadata(
	_ context.Context,
	_ datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = "slackapp_manifest"
}

func (d *manifestDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Represents manifest of the Slack App.",
		Blocks: map[string]schema.Block{
			"metadata":            (*manifestblocks.Metadata)(nil).Schema(),
			"display_information": (*manifestblocks.DisplayInformation)(nil).Schema(),
			"settings":            (*manifestblocks.Settings)(nil).Schema(),
			"features":            (*manifestblocks.Features)(nil).Schema(),
			"oauth_config":        (*manifestblocks.OauthConfig)(nil).Schema(),
		},
		Attributes: map[string]schema.Attribute{
			"json": &schema.StringAttribute{
				MarkdownDescription: "JSON representation of the manifest.",
				Computed:            true,
			},
		},
	}
}

func (d *manifestDataSource) Read(
	ctx context.Context,
	request datasource.ReadRequest,
	response *datasource.ReadResponse,
) {
	var data manifestDataSourceModel

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
