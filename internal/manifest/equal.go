package manifest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Equal reports whether two manifests mean the same to Slack: it decodes
// both as generic JSON, sorts the arrays Slack treats as sets, and compares
// the results. Object key order and whitespace do not matter. It returns an
// error when either side is not valid JSON.
//
// It decodes into generic values rather than App, this package's
// model for the slackapp_manifest data source, on purpose: that struct models only
// part of the manifest, and decoding into it would drop every other field
// (functions, workflows, outgoing_domains, ...) from both sides, so a change
// to only those fields would compare equal and never reach Slack.
func Equal(a, b string) (bool, error) {
	valueA, err := decodeForEqual(a)
	if err != nil {
		return false, err
	}

	valueB, err := decodeForEqual(b)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(valueA, valueB), nil
}

// equalSetPaths are the manifest arrays whose order means nothing to Slack: it
// exports them in its own order. Every other array, such as shortcuts or
// slash_commands, keeps its order in the comparison.
var equalSetPaths = [][]string{
	{"oauth_config", "redirect_urls"},
	{"oauth_config", "scopes", "bot"},
	{"oauth_config", "scopes", "user"},
	{"settings", "allowed_ip_address_ranges"},
	{"settings", "event_subscriptions", "bot_events"},
	{"settings", "event_subscriptions", "user_events"},
	{"features", "unfurl_domains"},
}

// decodeForEqual decodes a manifest and sorts the arrays at equalSetPaths.
// Numbers stay json.Number, so that no precision is lost in the comparison.
func decodeForEqual(s string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	if decoder.More() {
		return nil, fmt.Errorf("unexpected data after the manifest")
	}

	root, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	for _, p := range equalSetPaths {
		sortSetForEqual(root, p)
	}

	return root, nil
}

// sortSetForEqual sorts the array at path in place, when one is there. Elements are
// ordered by their JSON encoding, so that an array of anything sorts.
func sortSetForEqual(root map[string]any, path []string) {
	parent := root
	for _, key := range path[:len(path)-1] {
		next, ok := parent[key].(map[string]any)
		if !ok {
			return
		}
		parent = next
	}

	values, ok := parent[path[len(path)-1]].([]any)
	if !ok {
		return
	}

	sort.SliceStable(values, func(i, j int) bool {
		return setElementKeyForEqual(values[i]) < setElementKeyForEqual(values[j])
	})
}

func setElementKeyForEqual(v any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}

	return string(encoded)
}
