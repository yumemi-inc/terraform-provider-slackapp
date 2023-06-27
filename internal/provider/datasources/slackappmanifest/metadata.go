package slackappmanifest

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/typeconv"
)

type Metadata struct {
	MajorVersion types.Int64 `tfsdk:"major_version"`
	MinorVersion types.Int64 `tfsdk:"minor_version"`
}

func (*Metadata) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings that describe the manifest.",
		Attributes: map[string]schema.Attribute{
			"major_version": &schema.Int64Attribute{
				MarkdownDescription: "An integer that specifies the major version of the manifest schema to target.",
				Optional:            true,
			},
			"minor_version": &schema.Int64Attribute{
				MarkdownDescription: "An integer that specifies the minor version of the manifest schema to target.",
				Optional:            true,
			},
		},
	}
}

func (m Metadata) Read() manifest.Metadata {
	return manifest.Metadata{
		MajorVersion: typeconv.Int64PtrAsIntPtr(m.MajorVersion.ValueInt64Pointer()),
		MinorVersion: typeconv.Int64PtrAsIntPtr(m.MinorVersion.ValueInt64Pointer()),
	}
}
