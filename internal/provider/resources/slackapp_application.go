package resources

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/common"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack"
)

type SlackAppModel struct {
	// Arguments
	Manifest types.String `tfsdk:"manifest"`

	// Attributes
	ID                types.String `tfsdk:"id"`
	Credentials       types.Object `tfsdk:"credentials"`
	OauthAuthorizeURL types.String `tfsdk:"oauth_authorize_url"`
}

type SlackApp struct {
	ctx *common.ProviderContext
}

func NewSlackApp() resource.Resource {
	return &SlackApp{}
}

func (r *SlackApp) Metadata(
	_ context.Context,
	_ resource.MetadataRequest,
	response *resource.MetadataResponse,
) {
	response.TypeName = "slackapp_application"
}

func (r *SlackApp) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Arguments
			"manifest": &schema.StringAttribute{
				MarkdownDescription: "A JSON app manifest encoded as a string. This manifest must use a valid [app manifest schema - read our guide to creating one](https://api.slack.com/reference/manifests#fields).",
				Required:            true,
			},

			// Attributes
			"id": &schema.StringAttribute{
				MarkdownDescription: "Unique identifier of the app.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": &schema.ObjectAttribute{
				MarkdownDescription: "Secrets and credentials for the app.",
				Computed:            true,
				Sensitive:           true,
				AttributeTypes: map[string]attr.Type{
					"client_id":          types.StringType,
					"client_secret":      types.StringType,
					"verification_token": types.StringType,
					"signing_secret":     types.StringType,
				},
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
			},
			"oauth_authorize_url": &schema.StringAttribute{
				MarkdownDescription: "URL of the OAuth 2 authorization endpoint.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *SlackApp) Configure(
	_ context.Context,
	request resource.ConfigureRequest,
	response *resource.ConfigureResponse,
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

	r.ctx = providerContext
}

func (r *SlackApp) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data SlackAppModel

	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	apiResponse, err := r.ctx.SlackClient.AppsManifestCreate(
		ctx, slack.AppsManifestCreateRequest{
			Manifest: data.Manifest.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}

	data.ID = types.StringValue(apiResponse.AppID)
	data.Credentials = types.ObjectValueMust(
		map[string]attr.Type{
			"client_id":          types.StringType,
			"client_secret":      types.StringType,
			"verification_token": types.StringType,
			"signing_secret":     types.StringType,
		},
		map[string]attr.Value{
			"client_id":          types.StringValue(apiResponse.Credentials.ClientID),
			"client_secret":      types.StringValue(apiResponse.Credentials.ClientSecret),
			"verification_token": types.StringValue(apiResponse.Credentials.VerificationToken),
			"signing_secret":     types.StringValue(apiResponse.Credentials.SigningSecret),
		},
	)
	data.OauthAuthorizeURL = types.StringValue(apiResponse.OauthAuthorizeURL)

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *SlackApp) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data SlackAppModel

	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	apiResponse, err := r.ctx.SlackClient.AppsManifestExport(
		ctx, slack.AppsManifestExportRequest{
			AppID: data.ID.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}

	manifestJSON, err := json.Marshal(apiResponse.Manifest)
	if err != nil {
		response.Diagnostics.AddError("Failed to re-serialize the JSON manifest.", err.Error())

		return
	}

	data.Manifest = types.StringValue(string(manifestJSON))

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *SlackApp) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var before, after SlackAppModel

	response.Diagnostics.Append(request.State.Get(ctx, &before)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &after)...)

	if response.Diagnostics.HasError() {
		return
	}

	_, err := r.ctx.SlackClient.AppsManifestUpdate(
		ctx, slack.AppsManifestUpdateRequest{
			AppID:    after.ID.ValueString(),
			Manifest: after.Manifest.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}

	after.Credentials = before.Credentials
	after.OauthAuthorizeURL = before.OauthAuthorizeURL

	response.Diagnostics.Append(response.State.Set(ctx, &after)...)
}

func (r *SlackApp) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data SlackAppModel

	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	_, err := r.ctx.SlackClient.AppsManifestDelete(
		ctx, slack.AppsManifestDeleteRequest{
			AppID: data.ID.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}
}

func (r *SlackApp) ImportState(
	ctx context.Context,
	request resource.ImportStateRequest,
	response *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func (r *SlackApp) handleSlackErrorInDiag(diagnostics *diag.Diagnostics, err error) {
	slackErr, ok := err.(*slack.ErrorResponse)
	if ok && len(slackErr.Errors) > 0 {
		for _, e := range slackErr.Errors {
			diagnostics.AddError(e.Message, e.Pointer)
		}
	} else {
		diagnostics.AddError("Failed to create a Slack App using API.", err.Error())
	}
}
