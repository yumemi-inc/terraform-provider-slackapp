package manifest

import (
	"encoding/json"
)

type Metadata struct {
	MajorVersion *int `json:"major_version,omitempty"`
	MinorVersion *int `json:"minor_version,omitempty"`
}

type DisplayInformation struct {
	Name            string  `json:"name"`
	Description     *string `json:"description,omitempty"`
	LongDescription *string `json:"long_description,omitempty"`
	BackgroundColor *string `json:"background_color,omitempty"`
}

type EventSubscriptions struct {
	RequestURL *string  `json:"request_url,omitempty"`
	BotEvents  []string `json:"bot_events,omitempty"`
	UserEvents []string `json:"user_events,omitempty"`
}

type Interactivity struct {
	IsEnabled             bool    `json:"is_enabled"`
	RequestURL            *string `json:"request_url,omitempty"`
	MessageMenuOptionsURL *string `json:"message_menu_options_url,omitempty"`
}

type Settings struct {
	AllowedIPAddressRanges []string            `json:"allowed_ip_address_ranges,omitempty"`
	EventSubscriptions     *EventSubscriptions `json:"event_subscriptions,omitempty"`
	Interactivity          *Interactivity      `json:"interactivity,omitempty"`
	OrgDeployEnabled       *bool               `json:"org_deploy_enabled,omitempty"`
	SocketModeEnabled      *bool               `json:"socket_mode_enabled,omitempty"`
}

type AppHome struct {
	HomeTabEnabled             *bool `json:"home_tab_enabled,omitempty"`
	MessagesTabEnabled         *bool `json:"messages_tab_enabled,omitempty"`
	MessagesTabReadOnlyEnabled *bool `json:"messages_tab_read_only_enabled,omitempty"`
}

type BotUser struct {
	DisplayName  string `json:"display_name"`
	AlwaysOnline *bool  `json:"always_online,omitempty"`
}

type ShortcutType string

const (
	ShortcutTypeMessage ShortcutType = "message"
	ShortcutTypeGlobal  ShortcutType = "global"
)

type Shortcut struct {
	Name        string       `json:"name"`
	CallbackID  string       `json:"callback_id"`
	Description string       `json:"description"`
	Type        ShortcutType `json:"type"`
}

type SlashCommand struct {
	Command      string  `json:"command"`
	Description  string  `json:"description"`
	ShouldEscape *bool   `json:"should_escape,omitempty"`
	URL          *string `json:"url,omitempty"`
	UsageHint    *string `json:"usage_hint,omitempty"`
}

type WorkflowStep struct {
	Name       string `json:"name"`
	CallbackID string `json:"callback_id"`
}

type Features struct {
	AppHome       *AppHome       `json:"app_home,omitempty"`
	BotUser       *BotUser       `json:"bot_user,omitempty"`
	Shortcuts     []Shortcut     `json:"shortcuts,omitempty"`
	SlashCommands []SlashCommand `json:"slash_commands,omitempty"`
	UnfurlDomains []string       `json:"unfurl_domains,omitempty"`
	WorkflowSteps []WorkflowStep `json:"workflow_steps,omitempty"`
}

type Scopes struct {
	Bot  []string `json:"bot,omitempty"`
	User []string `json:"user,omitempty"`
}

type OauthConfig struct {
	RedirectURLs []string `json:"redirect_urls,omitempty"`
	Scopes       *Scopes  `json:"scopes,omitempty"`
}

type App struct {
	Metadata           *Metadata          `json:"_metadata,omitempty"`
	DisplayInformation DisplayInformation `json:"display_information"`
	Settings           *Settings          `json:"settings,omitempty"`
	Features           *Features          `json:"features,omitempty"`
	OauthConfig        *OauthConfig       `json:"oauth_config,omitempty"`
}

func (m *App) ToJsonString() (string, error) {
	bytes, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}
