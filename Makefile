ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := $(abspath $(ROOT_DIR)/bin)
GO_INSTALL = ./scripts/go_install.sh

GO_APIDIFF_VER := v0.8.2
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(BIN_DIR)/$(GO_APIDIFF_BIN)-$(GO_APIDIFF_VER)

default: operator

.PHONY: generate-crd
generate-crd:
	go generate main.go

.PHONY: generate
generate:
	$(MAKE) generate-crd

.PHONY: operator
operator:
	CGO_ENABLED=0 go build -ldflags \
            "-X github.com/rancher/ali-operator/pkg/version.GitCommit=$(GIT_COMMIT) \
             -X github.com/rancher/ali-operator/pkg/version.Version=$(TAG)" \
        -o bin/ali-operator .

$(GO_APIDIFF):
	GOBIN=$(BIN_DIR) $(GO_INSTALL) github.com/joelanford/go-apidiff $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

APIDIFF_OLD_COMMIT ?= $(shell git rev-parse origin/main)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(APIDIFF_OLD_COMMIT) --print-compatible