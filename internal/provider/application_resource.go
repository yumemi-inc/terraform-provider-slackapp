package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/manifesttype"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/planmodifiers"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

type applicationResourceModel struct {
	// Arguments
	Manifest manifesttype.Manifest `tfsdk:"manifest"`

	// Attributes
	ID                types.String `tfsdk:"id"`
	Credentials       types.Object `tfsdk:"credentials"`
	OauthAuthorizeURL types.String `tfsdk:"oauth_authorize_url"`
}

type applicationResource struct {
	client *slack.Client
}

//declscope:shared // provider.go registers it
func newApplicationResource() resource.Resource {
	return &applicationResource{}
}

func (r *applicationResource) Metadata(
	_ context.Context,
	_ resource.MetadataRequest,
	response *resource.MetadataResponse,
) {
	response.TypeName = "slackapp_application"
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			// Arguments
			"manifest": &schema.StringAttribute{
				MarkdownDescription: "A JSON app manifest encoded as a string. This manifest must use a valid [app manifest schema - read our guide to creating one](https://api.slack.com/reference/manifests#fields).",
				Required:            true,
				CustomType:          manifesttype.ManifestType{},
				PlanModifiers: []planmodifier.String{
					planmodifiers.SuppressEquivalentManifest(),
				},
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
					planmodifiers.KeepPriorObject(),
				},
			},
			"oauth_authorize_url": &schema.StringAttribute{
				MarkdownDescription: "URL of the OAuth 2 authorization endpoint.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					planmodifiers.KeepPriorString(),
				},
			},
		},
	}
}

func (r *applicationResource) Configure(
	_ context.Context,
	request resource.ConfigureRequest,
	response *resource.ConfigureResponse,
) {
	if request.ProviderData == nil {
		return
	}

	client, ok := request.ProviderData.(*slack.Client)
	if !ok {
		response.Diagnostics.AddError(
			"The provider is not configured properly.",
			fmt.Sprintf("request.ProviderData is %T, not *slack.Client", request.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *applicationResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var data applicationResourceModel

	response.Diagnostics.Append(request.Plan.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	apiResponse, err := r.client.AppsManifestCreate(
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

func (r *applicationResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var data applicationResourceModel

	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	apiResponse, err := r.client.AppsManifestExport(
		ctx, slack.AppsManifestExportRequest{
			AppID: data.ID.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}

	if len(apiResponse.Manifest) == 0 || string(apiResponse.Manifest) == "null" {
		response.Diagnostics.AddError("Slack API returned empty manifest.", "apps.manifest.export returned ok but no manifest payload")
		return
	}

	var exported map[string]json.RawMessage
	if err := json.Unmarshal(apiResponse.Manifest, &exported); err != nil {
		response.Diagnostics.AddError("Slack API returned a manifest that is not a JSON object.", err.Error())

		return
	}

	// Slack drops _metadata from the manifest on applying, so carry it over
	// from the manifest in state. On import there is none yet.
	hasLocalManifest := !data.Manifest.IsNull() && !data.Manifest.IsUnknown() && strings.TrimSpace(data.Manifest.ValueString()) != ""
	if _, ok := exported["_metadata"]; !ok && hasLocalManifest {
		var local map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data.Manifest.ValueString()), &local); err != nil {
			response.Diagnostics.AddAttributeError(
				path.Root("manifest"),
				"Manifest must be a valid JSON.",
				err.Error(),
			)

			return
		}

		if metadata, ok := local["_metadata"]; ok {
			exported["_metadata"] = metadata
		}
	}

	// Every field Slack exported is kept, including the ones
	// the slackapp_manifest data source does not model.
	manifestJSON, err := json.Marshal(exported)
	if err != nil {
		response.Diagnostics.AddError("Failed to re-serialize the JSON manifest.", err.Error())

		return
	}

	// Slack fills in every setting the manifest did not state. Keep only the
	// fields the manifest in state has, so those defaults do not show as
	// drift. On import there is no manifest in state, and all of it is kept.
	if hasLocalManifest {
		pruned, err := manifest.PruneToPrior(string(manifestJSON), data.Manifest.ValueString())
		if err != nil {
			response.Diagnostics.AddError("Failed to compare the exported manifest with the one in state.", err.Error())

			return
		}

		manifestJSON = []byte(pruned)
	}

	data.Manifest = manifesttype.NewManifestValue(string(manifestJSON))

	response.Diagnostics.Append(response.State.Set(ctx, &data)...)
}

func (r *applicationResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var before, after applicationResourceModel

	response.Diagnostics.Append(request.State.Get(ctx, &before)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &after)...)

	if response.Diagnostics.HasError() {
		return
	}

	_, err := r.client.AppsManifestUpdate(
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

func (r *applicationResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var data applicationResourceModel

	response.Diagnostics.Append(request.State.Get(ctx, &data)...)

	if response.Diagnostics.HasError() {
		return
	}

	_, err := r.client.AppsManifestDelete(
		ctx, slack.AppsManifestDeleteRequest{
			AppID: data.ID.ValueString(),
		},
	)
	if err != nil {
		r.handleSlackErrorInDiag(&response.Diagnostics, err)

		return
	}
}

func (r *applicationResource) ImportState(
	ctx context.Context,
	request resource.ImportStateRequest,
	response *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func (r *applicationResource) handleSlackErrorInDiag(diagnostics *diag.Diagnostics, err error) {
	slackErr, ok := err.(*slack.ErrorResponse)
	if ok && len(slackErr.Errors) > 0 {
		for _, e := range slackErr.Errors {
			diagnostics.AddError(e.Message, e.Pointer)
		}
	} else {
		diagnostics.AddError("Failed to create a Slack App using API.", err.Error())
	}
}
