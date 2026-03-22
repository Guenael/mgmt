# awsresgen Makefile - generates mgmt resources from AWS CloudFormation schemas.
#
# Schemas are downloaded from the public AWS CloudFormation schema registry:
#   https://schema.cloudformation.us-east-1.amazonaws.com/
#
# Two methods are available for listing and downloading schemas:
#
# 1. Unauthenticated (default): Uses the public schema registry URLs. No AWS
#    credentials needed. Targets: list, download, download-all
#
# 2. Authenticated (-auth suffix): Uses the AWS CLI. Requires aws CLI and
#    valid credentials. Targets: list-auth, download-auth, download-all-auth

SCHEMAS_DIR := schemas
OUTPUT_DIR := ..
TEMPLATE := templates/aws_resource.go.tpl
ROOT_DIR := $(shell cd ../../.. && pwd)

SCHEMA_BASE_URL := https://schema.cloudformation.us-east-1.amazonaws.com
SCHEMA_ZIP_URL := $(SCHEMA_BASE_URL)/CloudformationSchema.zip

# resources.txt lists the CloudFormation type names to download and generate.
# One type per line. Lines starting with # are comments. Empty lines are
# ignored. Add or remove types to control which AWS resources mgmt supports.
RESOURCES_FILE := resources.txt

# AWS_REGION is used by the -auth targets. Defaults to us-east-1.
AWS_REGION ?= us-east-1

.PHONY: generate list download download-all list-auth download-auth download-all-auth clean

# --------------------------------------------------------------------------
# Code generation
# --------------------------------------------------------------------------

# generate produces aws_*_gen.go files in engine/resources/ from the schemas
# in the schemas/ directory.
generate: $(TEMPLATE) main.go generator.go
	@if [ -z "$$(ls $(SCHEMAS_DIR)/*.json 2>/dev/null)" ]; then \
		echo "No schemas found in $(SCHEMAS_DIR)/. Run 'make download' first."; \
		exit 1; \
	fi
	@cd $(ROOT_DIR) && go run ./engine/resources/awsresgen/ \
		-schemas=engine/resources/awsresgen/$(SCHEMAS_DIR) \
		-output=engine/resources/aws \
		-resources=engine/resources/awsresgen/$(RESOURCES_FILE) \
		-templates=engine/resources/awsresgen/$(TEMPLATE)

# --------------------------------------------------------------------------
# Unauthenticated (public schema registry, no AWS credentials)
# --------------------------------------------------------------------------

# list prints all available CloudFormation resource type names by downloading
# the public schema archive (~12MB). Output can be pasted into resources.txt.
# Pipe to grep to filter, e.g.: make list | grep -i ec2
list:
	@echo "Fetching resource type index (via zip archive)..." >&2
	@tmpdir=$$(mktemp -d) && \
	curl -sfL "$(SCHEMA_ZIP_URL)" -o "$$tmpdir/schemas.zip" && \
	unzip -o -q "$$tmpdir/schemas.zip" -d "$$tmpdir/schemas" && \
	grep -h '"typeName"' $$tmpdir/schemas/*.json | \
		sed 's/.*"typeName"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/' | \
		sort && \
	rm -rf "$$tmpdir"

# download fetches schemas for all types listed in resources.txt from the
# public schema registry URLs.
download:
	@mkdir -p $(SCHEMAS_DIR)
	@count=0; \
	while IFS= read -r type || [ -n "$$type" ]; do \
		type=$$(echo "$$type" | sed 's/[[:space:]]*$$//; s/^[[:space:]]*//'); \
		case "$$type" in \
			''|\#*) continue ;; \
		esac; \
		slug=$$(echo "$$type" | tr '[:upper:]' '[:lower:]' | sed 's/::/-/g'); \
		file=$$(echo "$$type" | sed 's/::/_/g'); \
		echo "Downloading $$type..."; \
		if curl -sfL "$(SCHEMA_BASE_URL)/$$slug.json" \
			-o "$(SCHEMAS_DIR)/$$file.json"; then \
			count=$$((count + 1)); \
		else \
			echo "  FAILED: $$type (may not exist in registry)"; \
		fi; \
	done < $(RESOURCES_FILE); \
	echo "Downloaded $$count schema(s)."

# download-all fetches the full archive of all ~1550 CloudFormation schemas
# and extracts them into the schemas directory.
download-all:
	@mkdir -p $(SCHEMAS_DIR)
	@echo "Downloading full CloudFormation schema archive..."
	@curl -sfL "$(SCHEMA_ZIP_URL)" -o "$(SCHEMAS_DIR)/CloudformationSchema.zip"
	@unzip -o -q "$(SCHEMAS_DIR)/CloudformationSchema.zip" -d "$(SCHEMAS_DIR)"
	@rm -f "$(SCHEMAS_DIR)/CloudformationSchema.zip"
	@for f in $(SCHEMAS_DIR)/*.json; do \
		base=$$(basename "$$f"); \
		newname=$$(echo "$$base" | sed 's/-/_/g' | sed 's/^\(.\)/\U\1/' | \
			sed 's/_\(.\)/_\U\1/g'); \
		if [ "$$base" != "$$newname" ]; then \
			mv "$$f" "$(SCHEMAS_DIR)/$$newname"; \
		fi; \
	done
	@echo "Extracted $$(ls $(SCHEMAS_DIR)/*.json | wc -l) schema(s)."

# --------------------------------------------------------------------------
# Authenticated (requires AWS CLI + credentials)
# --------------------------------------------------------------------------

# list-auth prints all available CloudFormation resource type names using the
# AWS CLI. Faster and lighter than the zip method.
# Pipe to grep to filter, e.g.: make list-auth | grep -i ec2
list-auth:
	@echo "Fetching resource type index (via AWS CLI)..." >&2
	@aws cloudformation list-types \
		--type RESOURCE \
		--visibility PUBLIC \
		--provisioning-type FULLY_MUTABLE \
		--region $(AWS_REGION) \
		--query 'TypeSummaries[].TypeName' \
		--output text | \
		tr '\t' '\n' | \
		sort

# download-auth fetches schemas for all types listed in resources.txt using
# the AWS CLI describe-type command.
download-auth:
	@mkdir -p $(SCHEMAS_DIR)
	@count=0; \
	while IFS= read -r type || [ -n "$$type" ]; do \
		type=$$(echo "$$type" | sed 's/[[:space:]]*$$//; s/^[[:space:]]*//'); \
		case "$$type" in \
			''|\#*) continue ;; \
		esac; \
		file=$$(echo "$$type" | sed 's/::/_/g'); \
		echo "Downloading $$type..."; \
		if aws cloudformation describe-type \
			--type RESOURCE \
			--type-name "$$type" \
			--region $(AWS_REGION) \
			--query 'Schema' \
			--output text > "$(SCHEMAS_DIR)/$$file.json" 2>/dev/null; then \
			count=$$((count + 1)); \
		else \
			echo "  FAILED: $$type"; \
			rm -f "$(SCHEMAS_DIR)/$$file.json"; \
		fi; \
	done < $(RESOURCES_FILE); \
	echo "Downloaded $$count schema(s)."

# download-all-auth lists all available resource types via the AWS CLI and
# downloads the schema for each one. This can take a while (~1550 API calls).
download-all-auth:
	@mkdir -p $(SCHEMAS_DIR)
	@echo "Listing all resource types via AWS CLI..." >&2
	@count=0; \
	for type in $$(aws cloudformation list-types \
		--type RESOURCE \
		--visibility PUBLIC \
		--provisioning-type FULLY_MUTABLE \
		--region $(AWS_REGION) \
		--query 'TypeSummaries[].TypeName' \
		--output text | tr '\t' '\n'); do \
		file=$$(echo "$$type" | sed 's/::/_/g'); \
		echo "Downloading $$type..."; \
		if aws cloudformation describe-type \
			--type RESOURCE \
			--type-name "$$type" \
			--region $(AWS_REGION) \
			--query 'Schema' \
			--output text > "$(SCHEMAS_DIR)/$$file.json" 2>/dev/null; then \
			count=$$((count + 1)); \
		else \
			echo "  FAILED: $$type"; \
			rm -f "$(SCHEMAS_DIR)/$$file.json"; \
		fi; \
	done; \
	echo "Downloaded $$count schema(s)."

# --------------------------------------------------------------------------
# Cleanup
# --------------------------------------------------------------------------

# clean removes all generated resource files and downloaded schemas.
clean:
	rm -f $(ROOT_DIR)/engine/resources/aws/aws_*_gen.go
	rm -f $(SCHEMAS_DIR)/*.json
