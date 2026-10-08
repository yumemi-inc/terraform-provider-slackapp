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

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/tokenstore"
)

func configureSlackClient(d Model) (*slack.Client, error) {
	store, err := configureTokenStore(d)
	if err != nil {
		return nil, err
	}

	baseURL := stringOrEnvironment(d.BaseURL, "SLACK_BASE_URL")
	appConfigurationToken := stringOrEnvironment(d.AppConfigurationToken, "SLACK_APP_CONFIGURATION_TOKEN")
	refreshToken := stringOrEnvironment(d.RefreshToken, "SLACK_REFRESH_TOKEN")

	if appConfigurationToken == "" && refreshToken == "" && store == nil {
		return nil, errors.New("either app configuration token, refresh token or token store must be provided")
	}

	client := slack.NewClient().
		WithAppConfigurationToken(slack.AppConfigurationToken(appConfigurationToken)).
		WithRefreshToken(slack.RefreshToken(refreshToken))

	if store != nil {
		client = client.WithTokenStore(store)
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

// configureTokenStore returns the store the configuration names, or nil
// when it names none.
func configureTokenStore(d Model) (slack.TokenStore, error) {
	file, command := tokenStoreOrEnvironment(d.TokenStore)

	switch {
	case file != "" && len(command) > 0:
		// The schema rejects both in the attribute, but not in the
		// environment.
		return nil, errors.New("a token store must be either a file or a command, not both")
	case file != "":
		return tokenstore.NewFile(file), nil
	case len(command) > 0:
		return tokenstore.NewCommand(command), nil
	default:
		return nil, nil
	}
}

// tokenStoreOrEnvironment returns the file and the command the attribute
// names, or those SLACK_TOKEN_STORE_FILE and SLACK_TOKEN_STORE_COMMAND name
// when the attribute is not set. The attribute is taken whole: with it set,
// neither variable is read. From the environment, the command is one
// program, with no arguments.
//
// The schema gives each value in the attribute its type.
func tokenStoreOrEnvironment(attribute types.Object) (file string, command []string) {
	if attribute.IsNull() {
		if name := os.Getenv("SLACK_TOKEN_STORE_COMMAND"); name != "" {
			command = []string{name}
		}

		return os.Getenv("SLACK_TOKEN_STORE_FILE"), command
	}

	attributes := attribute.Attributes()
	fileValue, _ := attributes["file"].(types.String)
	commandValue, _ := attributes["command"].(types.List)

	for _, element := range commandValue.Elements() {
		value, _ := element.(types.String)
		command = append(command, value.ValueString())
	}

	return fileValue.ValueString(), command
}

type Model struct {
	AppConfigurationToken types.String `tfsdk:"app_configuration_token"`
	RefreshToken          types.String `tfsdk:"refresh_token"`
	BaseURL               types.String `tfsdk:"base_url"`
	TokenStore            types.Object `tfsdk:"token_store"`
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
			"token_store": schema.SingleNestedAttribute{
				MarkdownDescription: "Where to keep the tokens that each rotation of the refresh token returns. " +
					"Slack voids a refresh token once it is used, so without a store the configured refresh token " +
					"works for one run only. Set exactly one of `file` and `command`. " +
					"Can also be set with the `SLACK_TOKEN_STORE_FILE` or `SLACK_TOKEN_STORE_COMMAND` environment variable.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"file": schema.StringAttribute{
						MarkdownDescription: "Path of a file to keep the tokens in. Use it where the disk outlives a run, " +
							"such as on an Atlantis server. Runs that share the file take turns through a lock file beside it.",
						Optional: true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
							stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("command")),
						},
					},
					"command": schema.ListAttribute{
						MarkdownDescription: "A program, and its arguments, that keeps the tokens anywhere it likes, " +
							"such as a secret manager. The provider runs it with `get` added, and reads what it last kept from stdout " +
							"(nothing when it keeps nothing yet). It runs it with `store` added, and writes the new tokens to stdin. " +
							"The program must read all of stdin, and must never print the tokens to stderr: the provider shows stderr when the program fails. " +
							"The provider takes no lock: the program must keep two runs from rotating at once.",
						ElementType: types.StringType,
						Optional:    true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
							listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
						},
					},
				},
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
