# Auto generated binary variables helper managed by https://github.com/bwplotka/bingo v0.9. DO NOT EDIT.
# All tools are designed to be build inside $GOBIN.
BINGO_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
GOPATH ?= $(shell go env GOPATH)
GOBIN  ?= $(firstword $(subst :, ,${GOPATH}))/bin
GO     ?= $(shell which go)

# Below generated variables ensure that every time a tool under each variable is invoked, the correct version
# will be used; reinstalling only if needed.
# For example for api variable:
#
# In your main Makefile (for non array binaries):
#
#include .bingo/Variables.mk # Assuming -dir was set to .bingo .
#
#command: $(API)
#	@echo "Running api"
#	@$(API) <flags/args..>
#
API := $(GOBIN)/api-v0.1.2
$(API): $(BINGO_DIR)/api.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/api-v0.1.2"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=api.mod -o=$(GOBIN)/api-v0.1.2 "github.com/observatorium/api"

MDOX := $(GOBIN)/mdox-v0.9.0
$(MDOX): $(BINGO_DIR)/mdox.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/mdox-v0.9.0"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=mdox.mod -o=$(GOBIN)/mdox-v0.9.0 "github.com/bwplotka/mdox"

OBSERVATORIUM := $(GOBIN)/observatorium-v0.1.2
$(OBSERVATORIUM): $(BINGO_DIR)/observatorium.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/observatorium-v0.1.2"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=observatorium.mod -o=$(GOBIN)/observatorium-v0.1.2 "github.com/observatorium/api"

THANOS := $(GOBIN)/thanos-v0.39.2
$(THANOS): $(BINGO_DIR)/thanos.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/thanos-v0.39.2"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=thanos.mod -o=$(GOBIN)/thanos-v0.39.2 "github.com/thanos-io/thanos/cmd/thanos"

UP := $(GOBIN)/up-v0.0.0-20240109123005-e1e1857b0b6e
$(UP): $(BINGO_DIR)/up.mod
	@# Install binary/ries using Go 1.14+ build command. This is using bwplotka/bingo-controlled, separate go module with pinned dependencies.
	@echo "(re)installing $(GOBIN)/up-v0.0.0-20240109123005-e1e1857b0b6e"
	@cd $(BINGO_DIR) && GOWORK=off $(GO) build -mod=mod -modfile=up.mod -o=$(GOBIN)/up-v0.0.0-20240109123005-e1e1857b0b6e "github.com/observatorium/up/cmd/up"

