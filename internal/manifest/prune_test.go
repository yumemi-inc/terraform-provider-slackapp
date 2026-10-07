package manifest

import "testing"

func TestPruneToPrior(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		exported string
		prior    string
		want     string
	}{
		"defaults Slack fills in are dropped": {
			exported: `{"display_information":{"name":"A"},"features":{"bot_user":{"display_name":"B","always_online":true}},"oauth_config":{"pkce_enabled":false},"settings":{"socket_mode_enabled":true,"is_mcp_enabled":false,"interactivity":{"is_enabled":true}}}`,
			prior:    `{"display_information":{"name":"A"},"features":{"bot_user":{"display_name":"B"}},"oauth_config":{},"settings":{"socket_mode_enabled":true}}`,
			want:     `{"display_information":{"name":"A"},"features":{"bot_user":{"display_name":"B"}},"oauth_config":{},"settings":{"socket_mode_enabled":true}}`,
		},
		"an object the prior lacks is dropped whole": {
			exported: `{"display_information":{"name":"A"},"settings":{"org_deploy_enabled":false}}`,
			prior:    `{"display_information":{"name":"A"}}`,
			want:     `{"display_information":{"name":"A"}}`,
		},
		"a field both have keeps Slack's value": {
			// A change made in Slack to a field the config states must
			// still show.
			exported: `{"display_information":{"name":"Edited","description":"d"}}`,
			prior:    `{"display_information":{"name":"A"}}`,
			want:     `{"display_information":{"name":"Edited"}}`,
		},
		"fields in array elements are dropped": {
			exported: `{"features":{"slash_commands":[{"command":"/a","description":"x","should_escape":false},{"command":"/b","description":"y","should_escape":false}]}}`,
			prior:    `{"features":{"slash_commands":[{"command":"/a","description":"x"},{"command":"/b","description":"y"}]}}`,
			want:     `{"features":{"slash_commands":[{"command":"/a","description":"x"},{"command":"/b","description":"y"}]}}`,
		},
		"an array of another length is kept as exported": {
			exported: `{"features":{"slash_commands":[{"command":"/a","should_escape":false}]}}`,
			prior:    `{"features":{"slash_commands":[{"command":"/a"},{"command":"/b"}]}}`,
			want:     `{"features":{"slash_commands":[{"command":"/a","should_escape":false}]}}`,
		},
		"a value of another type is kept as exported": {
			exported: `{"settings":{"interactivity":{"is_enabled":true}}}`,
			prior:    `{"settings":{"interactivity":true}}`,
			want:     `{"settings":{"interactivity":{"is_enabled":true}}}`,
		},
		"numbers keep their precision": {
			exported: `{"_metadata":{"major_version":1,"minor_version":10000000000000001}}`,
			prior:    `{"_metadata":{"major_version":1,"minor_version":1}}`,
			want:     `{"_metadata":{"major_version":1,"minor_version":10000000000000001}}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := PruneToPrior(tc.exported, tc.prior)
			if err != nil {
				t.Fatal(err)
			}

			equal, err := Equal(got, tc.want)
			if err != nil {
				t.Fatal(err)
			}
			if !equal {
				t.Errorf("PruneToPrior returned\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestPruneToPriorInvalidJSON(t *testing.T) {
	t.Parallel()

	if _, err := PruneToPrior(`{`, `{}`); err == nil {
		t.Error("PruneToPrior accepted an invalid exported manifest")
	}

	if _, err := PruneToPrior(`{}`, `{"a":1} {"b":2}`); err == nil {
		t.Error("PruneToPrior accepted a manifest in state with trailing data")
	}
}
