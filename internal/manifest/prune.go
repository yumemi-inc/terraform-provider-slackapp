package manifest

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PruneToPrior returns the manifest Slack exported, keeping only the fields
// that the manifest in state also has.
//
// Slack fills in every setting a manifest did not state with its default
// (features.bot_user.always_online, settings.interactivity,
// oauth_config.pkce_enabled, settings.is_mcp_enabled, ...), and the set grows
// as Slack adds settings. Comparing those against a config that never
// mentions them would show drift that applying cannot clear, since Slack
// fills them in again.
//
// Fields are dropped wherever they appear: in nested objects, and in the
// objects of an array whose length matches the prior one, such as
// slash_commands. A field both sides have is kept, so a change Slack reports
// to anything the config states still shows as drift.
//
// Pruning applies only when refreshing. At plan time the config and state
// are compared in full, so a field added to or removed from the config is
// sent to Slack: removing one makes Slack reset it to its default.
func PruneToPrior(exported, prior string) (string, error) {
	exportedValue, err := pruneDecode(exported)
	if err != nil {
		return "", fmt.Errorf("exported manifest: %w", err)
	}

	priorValue, err := pruneDecode(prior)
	if err != nil {
		return "", fmt.Errorf("manifest in state: %w", err)
	}

	pruned, err := json.Marshal(pruneValue(exportedValue, priorValue))
	if err != nil {
		return "", err
	}

	return string(pruned), nil
}

func pruneDecode(s string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	if decoder.More() {
		return nil, fmt.Errorf("unexpected data after the manifest")
	}

	return value, nil
}

// pruneValue drops from exported every object key that prior does not have,
// at every level the two have the same shape.
func pruneValue(exported, prior any) any {
	switch exportedTyped := exported.(type) {
	case map[string]any:
		priorTyped, ok := prior.(map[string]any)
		if !ok {
			return exported
		}

		pruned := make(map[string]any, len(priorTyped))
		for key, value := range exportedTyped {
			if priorValue, ok := priorTyped[key]; ok {
				pruned[key] = pruneValue(value, priorValue)
			}
		}

		return pruned
	case []any:
		// An array of another length differs anyway, and which elements
		// would pair up is unclear, so it is left as Slack sent it.
		priorTyped, ok := prior.([]any)
		if !ok || len(priorTyped) != len(exportedTyped) {
			return exported
		}

		pruned := make([]any, len(exportedTyped))
		for i := range exportedTyped {
			pruned[i] = pruneValue(exportedTyped[i], priorTyped[i])
		}

		return pruned
	default:
		return exported
	}
}
