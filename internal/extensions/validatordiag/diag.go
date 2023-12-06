package validatordiag

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// DiscouragedBlockDiagnostic returns an warning Diagnostic to be used when a block is discouraged
func DiscouragedBlockDiagnostic(path path.Path, description string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Block",
		fmt.Sprintf("Block %s %s", path, description),
	)
}

// DiscouragedAttributeValueDiagnostic returns an warning Diagnostic to be used when an attribute has a discouraged value.
func DiscouragedAttributeValueDiagnostic(path path.Path, description string, value string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Attribute Value",
		fmt.Sprintf("Attribute %s %s, got: %s", path, description, value),
	)
}

// DiscouragedAttributeValueLengthDiagnostic returns an warning Diagnostic to be used when an attribute's value has a discouraged length.
func DiscouragedAttributeValueLengthDiagnostic(path path.Path, description string, value string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Attribute Value Length",
		fmt.Sprintf("Attribute %s %s, got: %s", path, description, value),
	)
}

// DiscouragedAttributeValueMatchDiagnostic returns an warning Diagnostic to be used when an attribute's value has a discouraged match.
func DiscouragedAttributeValueMatchDiagnostic(path path.Path, description string, value string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Attribute Value Match",
		fmt.Sprintf("Attribute %s %s, got: %s", path, description, value),
	)
}

// DiscouragedAttributeCombinationDiagnostic returns an warning Diagnostic to be used when a schemavalidator of attributes is discouraged.
func DiscouragedAttributeCombinationDiagnostic(path path.Path, description string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Attribute Combination",
		capitalize(description),
	)
}

// DiscouragedAttributeTypeDiagnostic returns an warning Diagnostic to be used when an attribute has a discouraged type.
func DiscouragedAttributeTypeDiagnostic(path path.Path, description string, value string) diag.Diagnostic {
	return diag.NewAttributeWarningDiagnostic(
		path,
		"Discouraged Attribute Type",
		fmt.Sprintf("Attribute %s %s, got: %s", path, description, value),
	)
}

// capitalize will uppercase the first letter in a UTF-8 string.
func capitalize(str string) string {
	if str == "" {
		return ""
	}

	firstRune, size := utf8.DecodeRuneInString(str)

	return string(unicode.ToUpper(firstRune)) + str[size:]
}
