package manifest

import (
	"testing"
)

func TestEqual(t *testing.T) {
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
			// outgoing_domains is not in the data source's model. Decoding into
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := Equal(tc.prior, tc.next)
			if err != nil {
				t.Fatal(err)
			}

			if got != tc.want {
				t.Fatalf("Equal(%q, %q) = %v, want %v", tc.prior, tc.next, got, tc.want)
			}
		})
	}
}

func TestEqualInvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := Equal(`not json`, `{}`); err == nil {
		t.Error("Equal accepted an invalid manifest")
	}
}

// A manifest that is not an object compares as a plain JSON value.
func TestEqualNonObject(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		a, b string
		want bool
	}{
		"equal arrays":        {`["a","b"]`, `[ "a", "b" ]`, true},
		"arrays in order":     {`["a","b"]`, `["b","a"]`, false},
		"object and an array": {`{}`, `[]`, false},
	}

	for name, tc := range cases {
		got, err := Equal(tc.a, tc.b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != tc.want {
			t.Errorf("%s: Equal(%s, %s) = %v, want %v", name, tc.a, tc.b, got, tc.want)
		}
	}
}

func TestEqualTrailingData(t *testing.T) {
	t.Parallel()

	if _, err := Equal(`{}`, `{} {}`); err == nil {
		t.Error("Equal accepted a second document after the manifest")
	}
}

// Set-like arrays whose elements are objects sort too.
func TestEqualSetOfObjects(t *testing.T) {
	t.Parallel()

	got, err := Equal(
		`{"features":{"unfurl_domains":[{"d":"b"},{"d":"a"}]}}`,
		`{"features":{"unfurl_domains":[{"d":"a"},{"d":"b"}]}}`,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("set-like arrays of objects compared in order")
	}
}
