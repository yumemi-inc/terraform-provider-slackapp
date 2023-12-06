package slackappmanifest

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/listvalidatorx"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/typeconv"
)

type AppHome struct {
	HomeTabEnabled             types.Bool `tfsdk:"home_tab_enabled"`
	MessagesTabEnabled         types.Bool `tfsdk:"messages_tab_enabled"`
	MessagesTabReadOnlyEnabled types.Bool `tfsdk:"messages_tab_read_only_enabled"`
}

func (*AppHome) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [App Home](https://api.slack.com/surfaces/tabs) configuration.",
		Attributes: map[string]schema.Attribute{
			"home_tab_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the [Home tab](https://api.slack.com/surfaces/tabs) is enabled.",
				Optional:            true,
			},
			"messages_tab_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the [Messages tab in your App Home](https://api.slack.com/surfaces/tabs) is enabled.",
				Optional:            true,
			},
			"messages_tab_read_only_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the users can send messages to your app in the [Messages tab of your App Home](https://api.slack.com/surfaces/tabs).",
				Optional:            true,
			},
		},
	}
}

func (h AppHome) Read() manifest.AppHome {
	return manifest.AppHome{
		HomeTabEnabled:             h.HomeTabEnabled.ValueBoolPointer(),
		MessagesTabEnabled:         h.MessagesTabEnabled.ValueBoolPointer(),
		MessagesTabReadOnlyEnabled: h.MessagesTabReadOnlyEnabled.ValueBoolPointer(),
	}
}

type BotUser struct {
	DisplayName  types.String `tfsdk:"display_name"`
	AlwaysOnline types.Bool   `tfsdk:"always_online"`
}

func (*BotUser) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [bot user](https://api.slack.com/bot-users) configuration.",
		Attributes: map[string]schema.Attribute{
			"display_name": &schema.StringAttribute{
				MarkdownDescription: "A string containing the display name of the bot user. Maximum length is 80 characters. Allowed characters: `a-z`, `A-Z`, `0-9`, `-`, `_`, and `.`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(80),
					stringvalidator.RegexMatches(
						regexp.MustCompile("^[0-9a-zA-Z-_.]+$"),
						"must be a string that contains only `a-z`, `A-Z`, `0-9`, `-`, `_`, and `.`",
					),
				},
			},
			"always_online": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the bot user will always appear to be online.",
				Optional:            true,
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtName("display_name"),
			),
		},
	}
}

func (u BotUser) Read() manifest.BotUser {
	return manifest.BotUser{
		DisplayName:  u.DisplayName.ValueString(),
		AlwaysOnline: u.AlwaysOnline.ValueBoolPointer(),
	}
}

type Shortcut struct {
	Name        types.String `tfsdk:"name"`
	CallbackID  types.String `tfsdk:"callback_id"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
}

func (*Shortcut) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [shortcuts](https://api.slack.com/interactivity/shortcuts) configuration. A maximum of 5 shortcuts can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": &schema.StringAttribute{
					MarkdownDescription: "A string containing the name of the shortcut.",
					Required:            true,
				},
				"callback_id": &schema.StringAttribute{
					MarkdownDescription: "A string containing the `callback_id` of this shortcut. Maximum length is 255 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"description": &schema.StringAttribute{
					MarkdownDescription: "A string containing a short description of this shortcut. Maximum length is 150 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(150),
					},
				},
				"type": &schema.StringAttribute{
					MarkdownDescription: "A string containing one of `message` or `global`. This specifies which [type of shortcut](https://api.slack.com/interactivity/shortcuts) is being described.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.OneOf("message", "global"),
					},
				},
			},
		},
		Validators: []validator.List{
			listvalidatorx.SizeAtMostWarning(5),
		},
	}
}

func (s Shortcut) Read() manifest.Shortcut {
	return manifest.Shortcut{
		Name:        s.Name.ValueString(),
		CallbackID:  s.CallbackID.ValueString(),
		Description: s.Description.ValueString(),
		Type:        manifest.ShortcutType(s.Type.ValueString()),
	}
}

type SlashCommand struct {
	Command      types.String `tfsdk:"command"`
	Description  types.String `tfsdk:"description"`
	ShouldEscape types.Bool   `tfsdk:"should_escape"`
	URL          types.String `tfsdk:"url"`
	UsageHint    types.String `tfsdk:"usage_hint"`
}

func (*SlashCommand) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [slash commands](https://api.slack.com/interactivity/slash-commands) configuration. A maximum of 5 slash commands can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"command": &schema.StringAttribute{
					MarkdownDescription: "A string containing the actual slash command. Maximum length is 32 characters, and should include the leading / character.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(32),
						stringvalidator.RegexMatches(regexp.MustCompile("^/.+$"), "must start with `/`"),
					},
				},
				"description": &schema.StringAttribute{
					MarkdownDescription: "A string containing a description of the slash command that will be displayed to users. Maximum length is 2000 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(2000),
					},
				},
				"should_escape": &schema.BoolAttribute{
					MarkdownDescription: "A boolean that specifies whether or not channels, users, and links typed with the slash command should be escaped.",
					Optional:            true,
				},
				"url": &schema.StringAttribute{
					MarkdownDescription: "A string containing the full https URL that acts as the slash command's [request URL](https://api.slack.com/interactivity/slash-commands#creating_commands).",
					Optional:            true,
				},
				"usage_hint": &schema.StringAttribute{
					MarkdownDescription: "A string a short usage hint about the slash command for users. Maximum length is 1000 characters.",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(1000),
					},
				},
			},
		},
		Validators: []validator.List{
			listvalidatorx.SizeAtMostWarning(5),
		},
	}
}

func (c SlashCommand) Read() manifest.SlashCommand {
	return manifest.SlashCommand{
		Command:      c.Command.ValueString(),
		Description:  c.Description.ValueString(),
		ShouldEscape: c.ShouldEscape.ValueBoolPointer(),
		URL:          c.URL.ValueStringPointer(),
		UsageHint:    c.UsageHint.ValueStringPointer(),
	}
}

type WorkflowStep struct {
	Name       types.String `tfsdk:"name"`
	CallbackID types.String `tfsdk:"callback_id"`
}

func (*WorkflowStep) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [workflow steps](https://api.slack.com/workflows/steps) configuration. A maximum of 10 workflow steps can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": &schema.StringAttribute{
					MarkdownDescription: "A string containing the name of the workflow step. Maximum length of 50 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(50),
					},
				},
				"callback_id": &schema.StringAttribute{
					MarkdownDescription: "A string containing the `callback_id` of the workflow step. Maximum length of 50 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(50),
					},
				},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtMost(10),
		},
	}
}

func (s WorkflowStep) Read() manifest.WorkflowStep {
	return manifest.WorkflowStep{
		Name:       s.Name.ValueString(),
		CallbackID: s.CallbackID.ValueString(),
	}
}

type Features struct {
	// Blocks
	AppHome       *AppHome       `tfsdk:"app_home"`
	BotUser       *BotUser       `tfsdk:"bot_user"`
	Shortcuts     []Shortcut     `tfsdk:"shortcut"`
	SlashCommands []SlashCommand `tfsdk:"slash_command"`
	WorkflowSteps []WorkflowStep `tfsdk:"workflow_step"`

	// Arguments
	UnfurlDomains types.Set `tfsdk:"unfurl_domains"`
}

func (*Features) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings corresponding to the **Features** section of the app config pages.",
		Blocks: map[string]schema.Block{
			"app_home":      (*AppHome)(nil).schema(),
			"bot_user":      (*BotUser)(nil).schema(),
			"shortcut":      (*Shortcut)(nil).schema(),
			"slash_command": (*SlashCommand)(nil).schema(),
			"workflow_step": (*WorkflowStep)(nil).schema(),
		},
		Attributes: map[string]schema.Attribute{
			"unfurl_domains": &schema.SetAttribute{
				MarkdownDescription: "An array of strings containing valid [unfurl domains](https://api.slack.com/reference/messaging/link-unfurling#configuring_domains) to register. A maximum of 5 unfurl domains can be included in this array. Please consult the [unfurl docs](https://api.slack.com/reference/messaging/link-unfurling#configuring_domains) for a list of domain requirements.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (f Features) Read() manifest.Features {
	return manifest.Features{
		AppHome:       typeconv.MapOptionModel[manifest.AppHome](f.AppHome),
		BotUser:       typeconv.MapOptionModel[manifest.BotUser](f.BotUser),
		Shortcuts:     typeconv.MapListModel[manifest.Shortcut](f.Shortcuts),
		SlashCommands: typeconv.MapListModel[manifest.SlashCommand](f.SlashCommands),
		UnfurlDomains: typeconv.MustStringSetAsArray(&f.UnfurlDomains),
		WorkflowSteps: typeconv.MapListModel[manifest.WorkflowStep](f.WorkflowSteps),
	}
}
