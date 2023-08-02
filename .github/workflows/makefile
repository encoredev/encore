# This makefile is used inconjunction with the .reviewdog.yml file in the root of the repo
.PHONY: list-modules go-vet staticcheck errcheck ineffassign go-fmt

# Automatically gather all information
ALL_SRC :=  $(shell find ../../ -name "*.go")
ALL_MODS = $(shell find ../../ -name go.mod)
MOD_DIRS = $(sort $(realpath $(dir $(ALL_MODS))))
REPO_DIR := $(realpath ../../)
SEMGREP_DIR := "$(REPO_DIR)/tools/semgrep-rules"

# List modules reports all found Go modules within the repository
list-modules:
	@echo $(MOD_DIRS)

# Function to run a command in each Go module with appropriate build tags
#
# REL_DIR is the relative path to the file from the repository root
#         it is computed by removing the REPO_DIR prefix from the $dir variable,
#		  then we remove the prefix "/" to make it relative
#         and finally escaping the slashes so we can use it in sed
define run_for_each_module
	@for dir in $(MOD_DIRS); do \
		TAGS=""; \
		if [ "$$dir" != "$(REPO_DIR)" ]; then \
			TAGS="-tags encore,encore_internal,encore_app"; \
		fi; \
		REL_DIR=$$(echo "$${dir#$(REPO_DIR)}/" | sed 's/^\///' | sed 's/\//\\\//g'); \
		(cd "$$dir" && $(1) $$TAGS $(2) | sed "s/^\.\//$$REL_DIR/"); \
	done;
endef

# Run Go vet
go-vet: $(ALL_SRC)
	# The sed statements are:
	#
	# 1. Remove any lines starting with "#" (go vet uses these for each package)
	# 2. Remove any "vet: " prefix from the output (sometimes we get this sometimes we dont)
	# 3. Remove any "./" prefix from the output (we'll get this for files which exist directly in the module root folder - this is done so we don't double up next)
	# 4. Add a "./" prefix to the output (this is so the sed within the run_for_each_module function can add the module path to each line)
	$(call run_for_each_module,go vet,./... 2>&1  | sed '/^#/d' | sed 's/^vet: //' | sed 's/^\.\///' | sed "s/^/\.\//")

## Run staticcheck
staticcheck: $(ALL_SRC)
	$(call run_for_each_module,staticcheck -tests=false -f=json,./... | jq -f "$(REPO_DIR)/.github/workflows/staticcheck-to-rdjsonl.jq" -c)

# Run errcheck
errcheck: $(ALL_SRC)
	$(call run_for_each_module,errcheck -abspath,./...)


## Run ineffassign
ineffassign: $(ALL_SRC)
	$(call run_for_each_module,ineffassign,./... 2>&1)

semgrep: $(ALL_SRC)
	@cd $(REPO_DIR) && semgrep scan --quiet --oss-only --config=auto --config=$(SEMGREP_DIR) --json | jq -f "$(REPO_DIR)/.github/workflows/semgrep-to-rdjson.jq" -c

go-fmt: $(ALL_SRC)
	@cd $(REPO_DIR) && gofmt -s -d . || exit 0
