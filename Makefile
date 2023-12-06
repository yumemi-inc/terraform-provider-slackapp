.PHONY: prepare
prepare:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

.PHONY: check
check:
	golangci-lint run

.PHONY: fix
fix:
	golangci-lint run --fix

terraform-provider-slackapp:
	go build .

docs: terraform-provider-slackapp
	tfplugindocs generate

.DEFAULT_GOAL = all
.PHONY: all
all: prepare terraform-provider-slackapp docs
