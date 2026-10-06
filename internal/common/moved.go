package common

// The provider moves to ymm-oss/slackapp after v0.2.9, the last release
// under yumemi-inc/slackapp. These texts tell users so: MovedNotice heads
// every page on the Terraform Registry, and the warning shows on every plan.

// MovedNotice uses the Registry's "!>" callout, since the Registry does not
// render GitHub's "> [!CAUTION]" alerts.
const MovedNotice = "!> **This provider has moved.** v0.2.9 is the last release of `yumemi-inc/slackapp`. " +
	"Later releases are published as [ymm-oss/slackapp](https://registry.terraform.io/providers/ymm-oss/slackapp)."

const MovedWarningSummary = "yumemi-inc/slackapp has moved to ymm-oss/slackapp"

const MovedWarningDetail = "v0.2.9 is the last release of yumemi-inc/slackapp. " +
	"Later releases are published as ymm-oss/slackapp:\n" +
	"https://registry.terraform.io/providers/ymm-oss/slackapp\n\n" +
	"To switch:\n" +
	"  1. Change source in required_providers to \"ymm-oss/slackapp\"\n" +
	"  2. Run: terraform state replace-provider yumemi-inc/slackapp ymm-oss/slackapp\n" +
	"  3. Run: terraform init"
