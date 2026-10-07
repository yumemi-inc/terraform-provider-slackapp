package manifest

import "testing"

func manifestPtr[T any](v T) *T { return &v }

func TestToJsonString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		app  App
		want string
	}{
		"only the name": {
			app:  App{DisplayInformation: DisplayInformation{Name: "A"}},
			want: `{"display_information":{"name":"A"}}`,
		},
		"false is written, nil is not": {
			app: App{
				DisplayInformation: DisplayInformation{Name: "A"},
				Settings:           &Settings{SocketModeEnabled: manifestPtr(false)},
			},
			want: `{"display_information":{"name":"A"},"settings":{"socket_mode_enabled":false}}`,
		},
		"empty arrays are left out": {
			app: App{
				DisplayInformation: DisplayInformation{Name: "A"},
				OauthConfig:        &OauthConfig{Scopes: &Scopes{Bot: []string{}}},
			},
			want: `{"display_information":{"name":"A"},"oauth_config":{"scopes":{}}}`,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := tc.app.ToJsonString()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("ToJsonString() = %s, want %s", got, tc.want)
			}
		})
	}
}
