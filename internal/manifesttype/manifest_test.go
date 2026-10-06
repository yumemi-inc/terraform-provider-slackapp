package manifesttype

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestStringSemanticEquals(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prior string
		next  string
		want  bool
	}{
		"identical": {
			prior: `{"display_information":{"name":"A"}}`,
			next:  `{"display_information":{"name":"A"}}`,
			want:  true,
		},
		"object key order differs": {
			// Slack export order (struct order) vs jsonencode (alphabetical).
			prior: `{"display_information":{"name":"A","description":"d","background_color":"#000"}}`,
			next:  `{"display_information":{"background_color":"#000","description":"d","name":"A"}}`,
			want:  true,
		},
		"bot scope order differs": {
			prior: `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"bot":["users:read.email","users:read","chat:write","im:write"]}}}`,
			next:  `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"bot":["chat:write","im:write","users:read","users:read.email"]}}}`,
			want:  true,
		},
		"user scope order differs": {
			prior: `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"user":["b","a"]}}}`,
			next:  `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"user":["a","b"]}}}`,
			want:  true,
		},
		"bot_events order differs": {
			prior: `{"display_information":{"name":"A"},"settings":{"event_subscriptions":{"bot_events":["message.im","app_mention"]}}}`,
			next:  `{"display_information":{"name":"A"},"settings":{"event_subscriptions":{"bot_events":["app_mention","message.im"]}}}`,
			want:  true,
		},
		"whitespace differs": {
			prior: `{"display_information":{"name":"A"}}`,
			next:  "{\n  \"display_information\": {\n    \"name\": \"A\"\n  }\n}",
			want:  true,
		},
		"added scope is a real change": {
			prior: `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"bot":["chat:write"]}}}`,
			next:  `{"display_information":{"name":"A"},"oauth_config":{"scopes":{"bot":["chat:write","im:write"]}}}`,
			want:  false,
		},
		"changed field is a real change": {
			prior: `{"display_information":{"name":"A","description":"old"}}`,
			next:  `{"display_information":{"name":"A","description":"new"}}`,
			want:  false,
		},
		"unmodeled field added is a real change": {
			// outgoing_domains is not in internal/slack/manifest. Decoding into
			// that struct would drop it from both sides.
			prior: `{"display_information":{"name":"A"}}`,
			next:  `{"display_information":{"name":"A"},"outgoing_domains":["a.example.com"]}`,
			want:  false,
		},
		"unmodeled field changed is a real change": {
			prior: `{"display_information":{"name":"A"},"functions":{"f":{"title":"Old"}}}`,
			next:  `{"display_information":{"name":"A"},"functions":{"f":{"title":"New"}}}`,
			want:  false,
		},
		"unmodeled nested field changed is a real change": {
			prior: `{"display_information":{"name":"A"},"features":{"assistant_view":{"assistant_description":"old"}}}`,
			next:  `{"display_information":{"name":"A"},"features":{"assistant_view":{"assistant_description":"new"}}}`,
			want:  false,
		},
		"unmodeled field with keys reordered is equal": {
			prior: `{"display_information":{"name":"A"},"functions":{"f":{"title":"T","description":"D"}}}`,
			next:  `{"functions":{"f":{"description":"D","title":"T"}},"display_information":{"name":"A"}}`,
			want:  true,
		},
		"unmodeled array order is significant": {
			// Only the arrays Slack treats as sets are sorted.
			prior: `{"display_information":{"name":"A"},"outgoing_domains":["a.example.com","b.example.com"]}`,
			next:  `{"display_information":{"name":"A"},"outgoing_domains":["b.example.com","a.example.com"]}`,
			want:  false,
		},
		"number precision is kept": {
			prior: `{"_metadata":{"major_version":1,"minor_version":1},"display_information":{"name":"A"}}`,
			next:  `{"_metadata":{"major_version":1,"minor_version":2},"display_information":{"name":"A"}}`,
			want:  false,
		},
		"slash_command order is significant": {
			// slash_commands is an ordered list, not a set: reordering is a change.
			prior: `{"display_information":{"name":"A"},"features":{"slash_commands":[{"command":"/a","description":"a"},{"command":"/b","description":"b"}]}}`,
			next:  `{"display_information":{"name":"A"},"features":{"slash_commands":[{"command":"/b","description":"b"},{"command":"/a","description":"a"}]}}`,
			want:  false,
		},
		"invalid prior json falls back to string compare (unequal)": {
			prior: `not json`,
			next:  `{"display_information":{"name":"A"}}`,
			want:  false,
		},
		"invalid json identical strings are equal": {
			prior: `not json`,
			next:  `not json`,
			want:  true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			prior := NewManifestValue(tc.prior)
			next := NewManifestValue(tc.next)

			got, diags := prior.StringSemanticEquals(context.Background(), next)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if got != tc.want {
				t.Fatalf("StringSemanticEquals(%q, %q) = %v, want %v", tc.prior, tc.next, got, tc.want)
			}
		})
	}
}

func TestStringSemanticEqualsNullUnknown(t *testing.T) {
	t.Parallel()

	known := NewManifestValue(`{"display_information":{"name":"A"}}`)
	null := Manifest{StringValue: basetypes.NewStringNull()}
	unknown := Manifest{StringValue: basetypes.NewStringUnknown()}

	if eq, _ := known.StringSemanticEquals(context.Background(), null); eq {
		t.Fatal("known vs null should not be equal")
	}
	if eq, _ := null.StringSemanticEquals(context.Background(), null); !eq {
		t.Fatal("null vs null should be equal")
	}
	if eq, _ := unknown.StringSemanticEquals(context.Background(), unknown); !eq {
		t.Fatal("unknown vs unknown should be equal")
	}
}
