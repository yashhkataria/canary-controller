# Targets are added phase by phase; see the Status section in CLAUDE.md.
# Add "## description" after a target name to list it in `make help`.

.DEFAULT_GOAL := help

.PHONY: help tools

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'

tools: ## Check that the required local tools are installed
	@bash scripts/check-tools.sh
